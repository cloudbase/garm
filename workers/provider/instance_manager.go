// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmErrors "github.com/cloudbase/garm/internal/errors"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	garmX509 "github.com/cloudbase/garm/util/x509"
)

// dbCallTimeout bounds a single store call made by an instance manager. The
// store may be a remote database, and a query that never comes back would
// otherwise park the manager's loop goroutine for good, stranding the instance
// in whatever transient status it was last written to. Provider calls carry
// their own deadline. This is the equivalent for the store.
const dbCallTimeout = 30 * time.Second

// The four helpers below are the manager's store operations. They exist so the
// deadline is applied in one place per operation rather than at every call
// site, and so no call site can forget it.

func (i *instanceManager) setInstanceStatus(name string, status commonParams.InstanceStatus, providerFault []byte, force bool) error {
	ctx, cancel := context.WithTimeout(i.ctx, dbCallTimeout)
	defer cancel()
	return i.helper.SetInstanceStatus(ctx, name, status, providerFault, force)
}

func (i *instanceManager) persistProviderState(name string, providerInstance commonParams.ProviderInstance) (params.Instance, error) {
	ctx, cancel := context.WithTimeout(i.ctx, dbCallTimeout)
	defer cancel()
	return i.helper.persistProviderInstanceState(ctx, name, providerInstance)
}

func (i *instanceManager) controllerInfo() (params.ControllerInfo, error) {
	ctx, cancel := context.WithTimeout(i.ctx, dbCallTimeout)
	defer cancel()
	return i.helper.GetControllerInfo(ctx)
}

func (i *instanceManager) githubEntity(entity params.ForgeEntity) (params.ForgeEntity, error) {
	ctx, cancel := context.WithTimeout(i.ctx, dbCallTimeout)
	defer cancel()
	return i.helper.GetGithubEntity(ctx, entity)
}

func newInstanceManager(ctx context.Context, instance params.Instance, scaleSet params.ScaleSet, provider common.Provider, helper providerHelper) (*instanceManager, error) {
	ctx = garmUtil.WithSlogContext(ctx, slog.Any("worker", fmt.Sprintf("instance-worker-%s", instance.Name)))

	githubEntity, err := scaleSet.GetEntity()
	if err != nil {
		return nil, fmt.Errorf("getting github entity: %w", err)
	}
	return &instanceManager{
		ctx:            ctx,
		instance:       instance,
		provider:       provider,
		deleteBackoff:  time.Second * 0,
		scaleSet:       scaleSet,
		helper:         helper,
		scaleSetEntity: githubEntity,
	}, nil
}

// instanceManager handles the lifecycle of a single instance.
// When an instance is created, a new instance manager is created
// for it. When the instance is placed in pending_create, the manager
// will attempt to create a new compute resource in the designated
// provider. Finally, when an instance is marked as pending_delete, it is removed
// from the provider and on success the instance is marked as deleted. Failure to
// delete, will place the instance back in pending delete. The removal process is
// retried after a backoff period. Instances placed in force_pending_delete will
// ignore provider errors and exit.
type instanceManager struct {
	ctx context.Context

	instance params.Instance
	provider common.Provider
	helper   providerHelper

	scaleSet       params.ScaleSet
	scaleSetEntity params.ForgeEntity

	deleteBackoff time.Duration

	updates chan dbCommon.ChangePayload
	// mux guards i.instance. It must only be held for short reads and writes,
	// never across provider calls.
	mux sync.Mutex
	// consolidateMux serializes consolidateState() invocations. Provider
	// operations can take minutes and cannot be canceled once started, so
	// consolidation works on a snapshot of the instance while updates continue
	// to land on i.instance under mux.
	consolidateMux sync.Mutex
	running        atomic.Bool
	quit           chan struct{}
}

func (i *instanceManager) Start() error {
	i.mux.Lock()
	defer i.mux.Unlock()

	slog.DebugContext(i.ctx, "starting instance manager", "instance", i.instance.Name)
	if i.running.Load() {
		return nil
	}

	i.running.Store(true)
	i.quit = make(chan struct{})
	i.updates = make(chan dbCommon.ChangePayload)

	go i.loop()
	go i.updatesLoop()
	return nil
}

func (i *instanceManager) Stop() error {
	i.mux.Lock()
	defer i.mux.Unlock()

	if !i.running.Load() {
		return nil
	}

	i.running.Store(false)
	close(i.quit)
	close(i.updates)
	return nil
}

func (i *instanceManager) sleepForBackOffOrCanceled() bool {
	timer := time.NewTimer(i.deleteBackoff)
	defer timer.Stop()

	slog.DebugContext(i.ctx, "sleeping for backoff", "duration", i.deleteBackoff, "instance", i.instance.Name)
	select {
	case <-timer.C:
		return false
	case <-i.quit:
		return true
	case <-i.ctx.Done():
		return true
	}
}

func (i *instanceManager) incrementBackOff() {
	if i.deleteBackoff == 0 {
		i.deleteBackoff = time.Second * 1
	} else {
		i.deleteBackoff *= 2
	}
	if i.deleteBackoff > time.Minute*5 {
		i.deleteBackoff = time.Minute * 5
	}
}

func (i *instanceManager) getEntity() (params.ForgeEntity, error) {
	entity, err := i.scaleSet.GetEntity()
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("getting entity: %w", err)
	}
	ghEntity, err := i.githubEntity(entity)
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("getting entity: %w", err)
	}
	return ghEntity, nil
}

func (i *instanceManager) pseudoPoolID() string {
	// nolint:golangci-lint,godox
	// TODO(gabriel-samfira): extend providers to know about scale sets.
	return garmUtil.ScaleSetPseudoPoolID(i.scaleSetEntity.ID, i.scaleSet.ID)
}

func (i *instanceManager) handleCreateInstanceInProvider(instance params.Instance) error {
	entity, err := i.getEntity()
	if err != nil {
		return fmt.Errorf("getting entity: %w", err)
	}

	token, err := i.helper.InstanceTokenGetter().NewInstanceJWTToken(
		instance, entity, i.scaleSet.RunnerBootstrapTimeout)
	if err != nil {
		return fmt.Errorf("creating instance token: %w", err)
	}
	tools, err := cache.GetGithubToolsCache(entity.ID)
	if err != nil {
		return fmt.Errorf("tools not found in cache for entity %s: %w", entity.String(), err)
	}

	if i.scaleSet.TemplateID == 0 {
		cloudConfigSpecs, err := garmUtil.GetCloudConfigSpecFromExtraSpecs(i.scaleSet.ExtraSpecs)
		if err != nil {
			return fmt.Errorf("pool has no template set and extra specs are invalid: %w", err)
		}
		if len(cloudConfigSpecs.RunnerInstallTemplate) == 0 {
			return fmt.Errorf("pool has no template set and extra specs has no runner_install_template set")
		}
	}

	controllerInfo := cache.ControllerInfo()

	caBundles, err := garmX509.CombineAndDeduplicateBundles(entity.Credentials.CABundle, controllerInfo.CACertBundle)
	if err != nil {
		return fmt.Errorf("failed to get CA cert bundle: %w", err)
	}

	bootstrapArgs := commonParams.BootstrapInstance{
		Name:          instance.Name,
		Tools:         tools,
		RepoURL:       entity.ForgeURL(),
		MetadataURL:   instance.MetadataURL,
		CallbackURL:   instance.CallbackURL,
		InstanceToken: token,
		OSArch:        i.scaleSet.OSArch,
		OSType:        i.scaleSet.OSType,
		Flavor:        i.scaleSet.Flavor,
		Image:         i.scaleSet.Image,
		ExtraSpecs:    i.scaleSet.ExtraSpecs,
		// This is temporary. We need to extend providers to know about scale sets.
		PoolID:            i.pseudoPoolID(),
		CACertBundle:      caBundles,
		GitHubRunnerGroup: i.scaleSet.GitHubRunnerGroup,
		JitConfigEnabled:  true,
	}

	if i.scaleSet.ProxyID != 0 {
		proxy, ok := cache.GetProxy(i.scaleSet.ProxyID)
		if !ok {
			return fmt.Errorf("proxy %d (%s) set on scale set %d was not found in cache", i.scaleSet.ProxyID, i.scaleSet.ProxyName, i.scaleSet.ID)
		}
		bootstrapArgs.ProxyConfig = proxy.ProxyConfig()
	}

	// We use the template management system unless:
	//   * there is no template associated with the pool/scale set
	//   * user explicitly overwrites the install template via extra specs
	bootstrapArgs = garmUtil.MaybeAddWrapperToExtraSpecs(i.ctx, bootstrapArgs)

	var instanceIDToDelete string
	baseParams, err := i.getProviderBaseParams()
	if err != nil {
		return fmt.Errorf("getting provider base params: %w", err)
	}

	defer func() {
		if instanceIDToDelete != "" {
			deleteInstanceParams := common.DeleteInstanceParams{
				DeleteInstanceV011: common.DeleteInstanceV011Params{
					ProviderBaseParams: baseParams,
				},
			}
			if err := i.provider.DeleteInstance(i.ctx, instanceIDToDelete, deleteInstanceParams); err != nil {
				if !errors.Is(err, runnerErrors.ErrNotFound) {
					slog.With(slog.Any("error", err)).ErrorContext(
						i.ctx, "failed to cleanup instance",
						"provider_id", instanceIDToDelete)
				}
			}
		}
	}()

	createInstanceParams := common.CreateInstanceParams{
		CreateInstanceV011: common.CreateInstanceV011Params{
			ProviderBaseParams: baseParams,
		},
	}

	// Bound the create call by the scale set's bootstrap timeout. Past that
	// point the runner is considered failed anyway, and reaping skips
	// instances in creating, so allowing the provider call to outlive the
	// bootstrap timeout would leave the instance stuck until the binary
	// resolved on its own. The provider-level exec timeout (if configured and
	// smaller) still applies; the effective deadline is whichever is sooner.
	createCtx, cancel := context.WithTimeout(i.ctx, time.Duration(i.scaleSet.RunnerTimeout())*time.Minute)
	defer cancel()
	admitted := false
	if guard, ok := i.helper.(interface {
		beginAdmittedCreate(context.Context, string, string, string, params.AdmissionCreateShape) (params.AdmissionReservation, bool, error)
	}); ok {
		shape := params.AdmissionCreateShape{Image: bootstrapArgs.Image, Flavor: bootstrapArgs.Flavor, OSArch: string(bootstrapArgs.OSArch), Generation: i.scaleSet.Generation}
		var reservation params.AdmissionReservation
		reservation, admitted, err = guard.beginAdmittedCreate(createCtx, i.scaleSet.ProviderName, fmt.Sprintf("scaleset:%d", i.scaleSet.ID), instance.Name, shape)
		if err != nil {
			return fmt.Errorf("authorizing provider create: %w", err)
		}
		if admitted {
			bootstrapArgs.ExtraSpecs, err = withAdmissionResources(bootstrapArgs.ExtraSpecs, reservation.Vector, reservation.BackendIdentity)
			if err != nil {
				return fmt.Errorf("adding admission resources: %w", err)
			}
		}
	}
	providerInstance, err := i.provider.CreateInstance(createCtx, bootstrapArgs, createInstanceParams)
	if err != nil {
		if !admitted {
			instanceIDToDelete = instance.Name
		}
		return fmt.Errorf("creating instance in provider: %w", err)
	}

	if admitted && providerInstance.Status != commonParams.InstanceRunning {
		return fmt.Errorf("admitted create did not report completed running instance")
	}
	if admitted {
		guard, ok := i.helper.(interface {
			markAdmissionPresent(context.Context, string, string) error
		})
		if !ok {
			return fmt.Errorf("admission presence recording unavailable")
		}
		if err := guard.markAdmissionPresent(createCtx, i.scaleSet.ProviderName, instance.Name); err != nil {
			return fmt.Errorf("recording admitted instance presence: %w", err)
		}
	}

	if providerInstance.Status == commonParams.InstanceError && !admitted {
		instanceIDToDelete = instance.ProviderID
		if instanceIDToDelete == "" {
			instanceIDToDelete = instance.Name
		}
	}

	updated, err := i.persistProviderState(instance.Name, providerInstance)
	if err != nil {
		return fmt.Errorf("updating instance args: %w", err)
	}

	// A newer update (like a user requested force delete) may have landed on
	// i.instance while the provider call was in flight. Overwriting it would
	// revert state that no future event will re-deliver, so only store the
	// result of our own DB write if the cached instance is not newer.
	i.mux.Lock()
	if !i.instance.UpdatedAt.After(updated.UpdatedAt) {
		i.instance = updated
	}
	i.mux.Unlock()

	return nil
}

func (i *instanceManager) getProviderBaseParams() (common.ProviderBaseParams, error) {
	info, err := i.controllerInfo()
	if err != nil {
		return common.ProviderBaseParams{}, fmt.Errorf("getting controller info: %w", err)
	}

	return common.ProviderBaseParams{
		ControllerInfo: info,
		EntityType:     i.scaleSetEntity.EntityType,
		EntityID:       i.scaleSetEntity.ID,
		ScaleSetID:     i.scaleSet.ID,
	}, nil
}

func (i *instanceManager) handleDeleteInstanceInProvider(instance params.Instance) error {
	if guard, ok := i.helper.(interface {
		admissionEnrolled(context.Context, string) (bool, error)
	}); ok {
		enrolled, err := guard.admissionEnrolled(i.ctx, i.scaleSet.ProviderName)
		if err != nil {
			return fmt.Errorf("checking admission enrollment: %w", err)
		}
		if enrolled {
			return i.deleteAdmittedInstance(instance)
		}
	}
	identifier := instance.ProviderID
	if identifier == "" {
		identifier = instance.Name
	}
	base, err := i.getProviderBaseParams()
	if err != nil {
		return fmt.Errorf("getting provider base params: %w", err)
	}
	args := common.DeleteInstanceParams{DeleteInstanceV011: common.DeleteInstanceV011Params{ProviderBaseParams: base}}
	if err := i.provider.DeleteInstance(i.ctx, identifier, args); err != nil {
		return fmt.Errorf("deleting instance in provider: %w", err)
	}
	return nil
}

func (i *instanceManager) deleteAdmittedInstance(instance params.Instance) error {
	policySource, ok := i.helper.(interface {
		admissionPolicy(context.Context, string) (params.AdmissionConfig, error)
	})
	if !ok {
		return fmt.Errorf("admission policy unavailable")
	}
	policy, err := policySource.admissionPolicy(i.ctx, i.scaleSet.ProviderName)
	if err != nil {
		return fmt.Errorf("reading admission policy: %w", err)
	}
	deleter, ok := i.provider.(common.AdmittedDeleteProvider)
	if !ok {
		return fmt.Errorf("provider does not support identity-bound deletion")
	}
	delReq := common.AdmittedDeleteRequest{Version: 1, RequestID: util.NewID(), ExpectedBackendIdentity: policy.BackendIdentity, Project: policy.Project, InstanceName: instance.Name}
	delRes, err := deleter.DeleteAdmitted(i.ctx, delReq)
	if err != nil {
		return fmt.Errorf("deleting admitted instance: %w", err)
	}
	if delRes.Request != delReq || !delRes.Deleted || delRes.BackendIdentity != policy.BackendIdentity || policy.BackendIdentity == "" {
		return fmt.Errorf("provider did not confirm identity-bound deletion")
	}
	authority, ok := i.provider.(common.AuthoritativeAbsenceProvider)
	if !ok {
		return fmt.Errorf("provider does not support authoritative absence confirmation")
	}
	req := common.AbsenceRequest{ExpectedBackendIdentity: policy.BackendIdentity, Version: 1, RequestID: util.NewID(), Generation: fmt.Sprint(policy.Generation), Domain: policy.Domain, Project: policy.Project, InstanceName: instance.Name}
	response, err := authority.ConfirmAuthoritativeAbsence(i.ctx, req)
	if err != nil {
		return fmt.Errorf("confirming authoritative absence: %w", err)
	}
	if err := common.ValidateAbsence(req, response, time.Now()); err != nil {
		return err
	}
	confirmation, ok := i.helper.(interface {
		confirmAdmissionAbsent(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("admission absence confirmation unavailable")
	}
	if err := confirmation.confirmAdmissionAbsent(i.ctx, i.scaleSet.ProviderName, instance.Name); err != nil {
		return fmt.Errorf("recording confirmed absence: %w", err)
	}
	return nil
}

// recordLifecycleEvent counts a runner removal decision made by this manager,
// by outcome. The entity String() form is used for the owner label; a bare
// repo name would be ambiguous across owners.
func (i *instanceManager) recordLifecycleEvent(outcome string) {
	var owner string
	if cachedEntity, ok := cache.GetEntity(i.scaleSetEntity.ID); ok {
		owner = cachedEntity.String()
	}
	metrics.RunnerLifecycleCount.WithLabelValues(
		outcome,                           // label: outcome
		i.scaleSet.ProviderName,           // label: provider
		owner,                             // label: pool_owner
		string(i.scaleSet.ScaleSetType()), // label: pool_type
		"",                                // label: pool_id
		strconv.FormatUint(uint64(i.scaleSet.ID), 10), // label: scaleset_id
	).Inc()
}

func (i *instanceManager) consolidateState() error {
	i.consolidateMux.Lock()
	defer i.consolidateMux.Unlock()

	if !i.running.Load() {
		return nil
	}

	// Work on a snapshot of the instance. Provider operations can take a long
	// time and cannot be canceled once started, and holding i.mux for the
	// duration of this function would block handleUpdate(). Updates land on
	// i.instance while we run; the next consolidation acts on the new state.
	i.mux.Lock()
	instance := i.instance
	i.mux.Unlock()

	switch instance.Status {
	case commonParams.InstancePendingCreate:
		// kick off the creation process
		if err := i.setInstanceStatus(instance.Name, commonParams.InstanceCreating, nil, false); err != nil {
			return fmt.Errorf("setting instance status to creating: %w", err)
		}
		if err := i.handleCreateInstanceInProvider(instance); err != nil {
			slog.ErrorContext(i.ctx, "creating instance in provider", "error", err)
			i.recordLifecycleEvent(metrics.OutcomeProviderError)
			if err := i.setInstanceStatus(instance.Name, commonParams.InstanceError, []byte(err.Error()), true); err != nil {
				return fmt.Errorf("setting instance status to error: %w", err)
			}
		}
	case commonParams.InstanceCreating:
		// Our own create pass returned without recording an outcome, so the
		// instance is orphaned rather than in progress. Consolidation is
		// serialized and runs on the same goroutine that performs the create,
		// so no create call can be in flight while we are in this switch.
		// Fail it and let the error lane below clean it up.
		//
		// Not forced on purpose. If the runner booted and registered anyway
		// the row is already running, which does not allow a move to error,
		// so the state machine refuses us instead of killing a live runner.
		slog.ErrorContext(
			i.ctx, "create left instance in creating; failing it",
			"runner_name", instance.Name)
		err := i.setInstanceStatus(
			instance.Name, commonParams.InstanceError,
			[]byte("create was interrupted before the provider result was recorded"), false)
		if err != nil {
			var te *runnerErrors.InstanceTransitionError
			if errors.As(err, &te) || errors.Is(err, runnerErrors.ErrNotFound) {
				// The row moved on without us. The watcher delivers the new
				// state, so there is nothing to do here.
				return nil
			}
			return fmt.Errorf("failing interrupted instance: %w", err)
		}
	case commonParams.InstanceRunning:
		// Nothing to do. The provider finished creating the instance.
	case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete, commonParams.InstanceDeleting:
		canceled, err := i.authorizeAdmissionDelete(instance.Name)
		if err != nil {
			return err
		}
		if canceled {
			return nil
		}
		// Remove or force remove the runner. When force remove is specified, we ignore
		// IaaS errors.
		//
		// InstanceDeleting is an interrupted delete. Restarts are recovered
		// by the scale set worker on startup (deleting is moved back to
		// pending_delete), but the manager can strand its own cached state
		// here at runtime: a pass that errors out after setting deleting but
		// before recording the outcome (the deleted write or the
		// pending_delete requeue fails transiently) leaves the cached status
		// at deleting with no further event to move it. Providers report a
		// missing instance as success, so re-driving the delete is safe.
		forced, err := i.effectiveForceDelete(instance.Status)
		if err != nil {
			return err
		}
		if i.waitForDelete(forced) {
			return nil
		}

		if err := i.setInstanceStatus(instance.Name, commonParams.InstanceDeleting, nil, true); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("setting instance status to deleting: %w", err)
		}

		if err := i.processProviderDelete(instance, forced); err != nil {
			return err
		}
		if err := i.setInstanceStatus(instance.Name, commonParams.InstanceDeleted, nil, false); err != nil {
			var te *runnerErrors.InstanceTransitionError
			switch {
			case errors.Is(err, runnerErrors.ErrNotFound):
				// The record is already gone.
			case errors.As(err, &te) && garmErrors.InstanceIsBeingDeleted(te.From):
				// The row moved elsewhere on the deletion lane while the
				// provider delete ran. The provider resource is confirmed
				// gone at this point, so force the terminal state instead of
				// re-driving a delete against nothing.
				if err := i.setInstanceStatus(instance.Name, commonParams.InstanceDeleted, nil, true); err != nil && !errors.Is(err, runnerErrors.ErrNotFound) {
					return fmt.Errorf("setting instance status to deleted: %w", err)
				}
			default:
				return fmt.Errorf("setting instance status to deleted: %w", err)
			}
		}
		return ErrInstanceDeleted
	case commonParams.InstanceError:
		if err := i.checkAdmissionCreateError(); err != nil {
			return err
		}
		// Instance is in error state. We wait for next status or potentially retry
		// spawning the instance with a backoff timer.
		if err := i.setInstanceStatus(instance.Name, commonParams.InstancePendingDelete, nil, true); err != nil {
			return fmt.Errorf("setting instance status to error: %w", err)
		}
	case commonParams.InstanceDeleted:
		return ErrInstanceDeleted
	}
	return nil
}

func (i *instanceManager) handleUpdate(update dbCommon.ChangePayload) error {
	// We need a better way to handle instance state. Database updates may fail, and we
	// end up with an inconsistent state between what we know about the instance and what
	// is reflected in the database.
	if !i.running.Load() {
		return nil
	}

	instance, ok := update.Payload.(params.Instance)
	if !ok {
		return runnerErrors.NewBadRequestError("invalid payload type")
	}

	i.mux.Lock()
	i.instance = instance
	i.mux.Unlock()
	return nil
}

func (i *instanceManager) Update(instance dbCommon.ChangePayload) error {
	if !i.running.Load() {
		return runnerErrors.NewBadRequestError("instance manager is not running")
	}

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	slog.DebugContext(i.ctx, "sending update to instance manager")
	select {
	case i.updates <- instance:
	case <-i.quit:
		return nil
	case <-i.ctx.Done():
		return nil
	case <-timer.C:
		return fmt.Errorf("timeout while sending update to instance manager")
	}
	return nil
}

func (i *instanceManager) updatesLoop() {
	defer i.Stop()
	for {
		select {
		case <-i.quit:
			return
		case <-i.ctx.Done():
			return
		case update, ok := <-i.updates:
			if !ok {
				slog.InfoContext(i.ctx, "updates channel closed")
				return
			}
			slog.DebugContext(i.ctx, "received update")
			if err := i.handleUpdate(update); err != nil {
				if errors.Is(err, ErrInstanceDeleted) {
					// instance had been deleted, we can exit the loop.
					return
				}
				slog.ErrorContext(i.ctx, "handling update", "error", err)
			}
		}
	}
}

func (i *instanceManager) loop() {
	defer i.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-i.quit:
			return
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			if err := i.consolidateState(); err != nil {
				if errors.Is(err, ErrInstanceDeleted) {
					// instance had been deleted, we can exit the loop.
					return
				}
				slog.ErrorContext(i.ctx, "consolidating state", "error", err)
			}
		}
	}
}

// withAdmissionResources copies the opaque object and reserves the admission
// namespace for immutable values authorized by the ledger.
func withAdmissionResources(extra json.RawMessage, vector params.AdmissionVector, backendIdentity string) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(extra) > 0 && string(extra) != "null" {
		if err := json.Unmarshal(extra, &values); err != nil {
			return nil, fmt.Errorf("invalid extra specs: %w", err)
		}
		if values == nil {
			return nil, fmt.Errorf("extra specs must be an object")
		}
	}
	if _, exists := values["admission_resources"]; exists {
		return nil, fmt.Errorf("admission_resources is reserved")
	}
	resources := struct {
		common.ReadinessResources
		BackendIdentity string `json:"backend_identity"`
	}{common.ReadinessResources{CPU: vector.CPU, MemoryBytes: vector.RAM, DiskBytes: vector.Disk, VMSlots: vector.VMs}, backendIdentity}
	if backendIdentity == "" {
		return nil, fmt.Errorf("admission backend identity is missing")
	}
	encoded, err := json.Marshal(resources)
	if err != nil {
		return nil, err
	}
	values["admission_resources"] = encoded
	return json.Marshal(values)
}

func (i *instanceManager) authorizeAdmissionDelete(name string) (bool, error) {
	guard, ok := i.helper.(interface {
		admissionEnrolled(context.Context, string) (bool, error)
	})
	if !ok {
		return false, nil
	}
	enrolled, err := guard.admissionEnrolled(i.ctx, i.scaleSet.ProviderName)
	if err != nil {
		return false, fmt.Errorf("checking admission enrollment: %w", err)
	}
	if !enrolled {
		return false, nil
	}
	cancelGuard, ok := i.helper.(interface {
		cancelUnsentAdmission(context.Context, string, string) (bool, error)
	})
	if !ok {
		return false, fmt.Errorf("admission cancellation unavailable")
	}
	canceled, err := cancelGuard.cancelUnsentAdmission(i.ctx, i.scaleSet.ProviderName, name)
	if err != nil {
		return false, fmt.Errorf("canceling unsent admission: %w", err)
	}
	if canceled {
		return true, nil
	}
	deleteGuard, ok := i.helper.(interface {
		beginAdmissionDelete(context.Context, string, string) error
	})
	if !ok {
		return false, fmt.Errorf("admission delete authorization unavailable")
	}
	if err := deleteGuard.beginAdmissionDelete(i.ctx, i.scaleSet.ProviderName, name); err != nil {
		return false, fmt.Errorf("authorizing admitted delete: %w", err)
	}
	return false, nil
}

func (i *instanceManager) checkAdmissionCreateError() error {
	guard, ok := i.helper.(interface {
		admissionEnrolled(context.Context, string) (bool, error)
	})
	if !ok {
		return nil
	}
	enrolled, err := guard.admissionEnrolled(i.ctx, i.scaleSet.ProviderName)
	if err != nil {
		return fmt.Errorf("checking admission enrollment: %w", err)
	}
	if enrolled {
		return fmt.Errorf("admitted create outcome requires reconciliation")
	}
	return nil
}

func (i *instanceManager) effectiveForceDelete(status commonParams.InstanceStatus) (bool, error) {
	forced := status == commonParams.InstancePendingForceDelete
	guard, ok := i.helper.(interface {
		admissionEnrolled(context.Context, string) (bool, error)
	})
	if !ok {
		return forced, nil
	}
	enrolled, err := guard.admissionEnrolled(i.ctx, i.scaleSet.ProviderName)
	if err != nil {
		return false, fmt.Errorf("checking admission enrollment: %w", err)
	}
	return forced && !enrolled, nil
}

func (i *instanceManager) waitForDelete(forced bool) bool {
	if forced {
		return false
	}
	return i.sleepForBackOffOrCanceled()
}

func (i *instanceManager) processProviderDelete(instance params.Instance, forced bool) error {
	if err := i.handleDeleteInstanceInProvider(instance); err != nil {
		slog.ErrorContext(i.ctx, "deleting instance in provider", "error", err, "forced", forced)
		if forced {
			return nil
		}
		i.incrementBackOff()
		if updateErr := i.setInstanceStatus(instance.Name, commonParams.InstancePendingDelete, []byte(err.Error()), true); updateErr != nil {
			return fmt.Errorf("setting instance status to error: %w", updateErr)
		}
		return fmt.Errorf("error removing instance. Will retry: %w", err)
	}
	return nil
}
