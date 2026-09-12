// Copyright 2026 Cloudbase Solutions SRL
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
package errors

import (
	"fmt"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

// NewRunnerTransitionError returns a RunnerTransitionError describing an invalid
// runner status state machine transition.
func NewRunnerTransitionError(from, to params.RunnerStatus) error {
	return &RunnerTransitionError{
		From: from,
		To:   to,
	}
}

// RunnerTransitionError is returned when a requested runner status transition is
// rejected by the state machine. It carries the current (From) and requested
// (To) statuses as their proper type, so callers can compare them directly (via
// errors.As) without casting through strings. It also reports as a
// BadRequestError, so it maps to an HTTP 400.
type RunnerTransitionError struct {
	From params.RunnerStatus
	To   params.RunnerStatus
}

func (e *RunnerTransitionError) Error() string {
	return fmt.Sprintf("invalid runner status transition from %s to %s", e.From, e.To)
}

func (e *RunnerTransitionError) Is(target error) bool {
	if target == nil {
		return false
	}

	switch target.(type) {
	case *RunnerTransitionError, *runnerErrors.BadRequestError:
		return true
	default:
		return false
	}
}

// InstanceIsBeingDeleted reports whether an instance status is on the deletion
// lane, meaning the runner is already on its way out. Note that the lane is
// not strictly monotonic: a failed provider delete moves the instance from
// deleting to error. Tolerating a status observed here is still safe because
// reconciliation re-drives error back to pending_delete until the delete
// succeeds — the system converges by retry, and the intent behind a rejected
// transition to pending_delete (the runner should be removed) remains
// satisfied. Pair it with InstanceTransitionError.From.
func InstanceIsBeingDeleted(s commonParams.InstanceStatus) bool {
	switch s {
	case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
		commonParams.InstanceDeleting, commonParams.InstanceDeleted:
		return true
	default:
		return false
	}
}

// InstanceIsProvisioning reports whether an instance status indicates the
// provider is still creating the instance (or is about to). The provider
// worker owns this stage of the lifecycle: deletion cannot be requested until
// the create call returns (creating only transitions to error or running).
// Pair it with InstanceTransitionError.From to decide whether a rejected
// transition to pending_delete should be deferred to a later reconciliation
// pass instead of being treated as fatal.
func InstanceIsProvisioning(s commonParams.InstanceStatus) bool {
	switch s {
	case commonParams.InstancePendingCreate, commonParams.InstanceCreating:
		return true
	default:
		return false
	}
}

// RunnerIsTerminal reports whether a runner status is terminal, meaning a
// transition to active can no longer succeed and a late "started" message for
// the runner is moot. Pair it with RunnerTransitionError.From.
func RunnerIsTerminal(s params.RunnerStatus) bool {
	switch s {
	case params.RunnerTerminated, params.RunnerFailed:
		return true
	default:
		return false
	}
}

// NewForbiddenError returns a ForbiddenError carrying the message the forge
// replied with.
func NewForbiddenError(msg string, a ...any) error {
	return &ForbiddenError{
		msg: fmt.Sprintf(msg, a...),
	}
}

// ForbiddenError is returned when a forge answers a request with 403 rather
// than 401. Both mean the request was refused, but only 401 implies the
// credentials are wrong: GitHub also answers 403 for a secondary rate limit or
// for SSO enforcement on an organization, neither of which says anything about
// the credentials, and both of which clear on their own or with an operator
// action that is not a credential rotation.
//
// It reports as a runnerErrors.UnauthorizedError so that callers which only
// ask "was this refused?" keep working unchanged. Callers that need to act on
// the difference — retrying a transient refusal instead of declaring the
// credentials dead — can single it out with errors.Is(err, &ForbiddenError{}),
// which must be checked before the broader unauthorized case.
type ForbiddenError struct {
	msg string
}

func (e *ForbiddenError) Error() string {
	return e.msg
}

func (e *ForbiddenError) Is(target error) bool {
	if target == nil {
		return false
	}

	switch target.(type) {
	case *ForbiddenError, *runnerErrors.UnauthorizedError:
		return true
	default:
		return false
	}
}
