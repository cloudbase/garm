// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	apiClientEnterprises "github.com/cloudbase/garm/client/enterprises"
	apiClientInstances "github.com/cloudbase/garm/client/instances"
	apiClientOrgs "github.com/cloudbase/garm/client/organizations"
	apiClientRepos "github.com/cloudbase/garm/client/repositories"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/workers/websocket/agent/messaging"
)

var (
	runnerRepository     string
	runnerOrganization   string
	runnerEnterprise     string
	runnerAll            bool
	forceRemove          bool
	bypassGHUnauthorized bool
	long                 bool
)

// runnerCmd represents the runner command
var runnerCmd = &cobra.Command{
	Use:          "runner",
	Aliases:      []string{"run"},
	SilenceUsage: true,
	Short:        "List runners in a pool",
	Long: `Given a pool ID, of either a repository or an organization,
list all instances.`,
	Run: nil,
}

type handlerErr struct {
	done chan struct{}
	once sync.Once
}

func (h *handlerErr) Close() {
	h.once.Do(func() { close(h.done) })
}

var agentShellCmd = &cobra.Command{
	Use:          "shell",
	Short:        "Execute an interactive shell",
	Long:         `Execute an interactive shell on the runner.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) != 1 {
			return fmt.Errorf("requires a runner name")
		}

		var sessionID uuid.UUID

		handlerErr := handlerErr{
			done: make(chan struct{}),
		}
		resizeCh := make(chan [2]int, 1)
		defer close(resizeCh)
		handler := func(msgType int, msg []byte) error {
			switch msgType {
			case websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseMessage:
				os.Stderr.Write([]byte("remote server closed the connection"))
				handlerErr.Close()
			case websocket.BinaryMessage, websocket.TextMessage:
				agentMsg, err := messaging.UnmarshalAgentMessage(msg)
				if err != nil {
					os.Stderr.Write([]byte("failed to unmarshal message"))
					handlerErr.Close()
				}
				switch agentMsg.Type {
				case messaging.MessageTypeShellReady:
					shellReady, err := messaging.Unmarshal[messaging.ShellReadyMessage](agentMsg)
					if err != nil {
						os.Stderr.Write(fmt.Appendf(nil, "failed to unmarshal shell ready: %q", err))
						handlerErr.Close()
					}
					sessionID = shellReady.SessionID
					if shellReady.IsError == 1 {
						if len(shellReady.Message) > 0 {
							os.Stderr.Write(fmt.Appendf(shellReady.Message, "\r\n"))
						}
						handlerErr.Close()
						return nil
					}
					if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
						resizeCh <- [2]int{w, h}
					}
				case messaging.MessageTypeShellExit:
					handlerErr.Close()
				case messaging.MessageTypeShellData:
					shellData, err := messaging.Unmarshal[messaging.ShellDataMessage](agentMsg)
					if err != nil {
						os.Stderr.Write([]byte("failed to unmarshal shell data message"))
						handlerErr.Close()
					}
					os.Stdout.Write(shellData.Data)
				default:
					os.Stdout.Write(fmt.Appendf(nil, "invalid agentMsg.Type: %v", agentMsg.Type))
				}
			default:
				os.Stdout.Write(fmt.Appendf(nil, "invalid message type: %v", msgType))
			}
			return nil
		}

		// Put terminal in raw mode
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)
		// Channel to stop on Ctrl+C
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)

		ctx, stop := signal.NotifyContext(context.Background(), signals...)
		defer stop()

		reader, err := garmWs.NewReader(ctx, mgr.BaseURL, fmt.Sprintf("/api/v1/ws/agent/%s/shell", args[0]), mgr.Token, handler)
		if err != nil {
			return err
		}

		if err := reader.Start(); err != nil {
			return err
		}

		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil {
					os.Stderr.Write(fmt.Appendf(nil, "failed to write message: %q", err))
					handlerErr.Close()
					return
				}

				if n > 0 && sessionID != uuid.Nil {
					msg := messaging.ShellDataMessage{
						SessionID: sessionID,
						Data:      buf[:n],
					}
					if err := reader.WriteMessage(websocket.BinaryMessage, msg.Marshal()); err != nil {
						os.Stderr.Write(fmt.Appendf(nil, "failed to write message: %q", err))
						handlerErr.Close()
						return
					}
				}
			}
		}()

		// ---- Watch terminal resize ----
		go watchTermResize(ctx, resizeCh, sessionID)

		// ---- Send resize messages ----
		go func() {
			for {
				select {
				case size := <-resizeCh:
					if sessionID == uuid.Nil {
						continue
					}
					msg := messaging.ShellResizeMessage{
						SessionID: sessionID,
						Cols:      uint16(size[0]),
						Rows:      uint16(size[1]),
					}
					reader.WriteMessage(websocket.BinaryMessage, msg.Marshal())
				case <-ctx.Done():
					return
				case <-reader.Done():
					return
				case <-handlerErr.done:
					return
				}
			}
		}()

		select {
		case <-ctx.Done():
		case <-reader.Done():
		case <-handlerErr.done:
		}
		return nil
	},
}

type instancesPayloadGetter interface {
	GetPayload() params.Instances
}

var runnerListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List runners",
	Long: `List runners of pools, scale sets, repositories, orgs or all of the above.

This command expects to get either a pool ID (UUID) or scale set ID (integer) as a
positional parameter, or it expects that one of the supported switches be used to
fetch runners of --repo, --org or --all

Example:

	List runners from one pool:
	garm-cli runner list e87e70bd-3d0d-4b25-be9a-86b85e114bcb

	List runners from one scale set:
	garm-cli runner list 42

	List runners from one repo:
	garm-cli runner list --repo=05e7eac6-4705-486d-89c9-0170bbb576af

	List runners from one org:
	garm-cli runner list --org=5493e51f-3170-4ce3-9f05-3fe690fc6ec6

	List runners from one enterprise:
	garm-cli runner list --enterprise=a966188b-0e05-4edc-9b82-bc81a1fd38ed

	List all runners from all pools belonging to all repos and orgs:
	garm-cli runner list --all

`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		var response instancesPayloadGetter
		var err error

		switch len(args) {
		case 1:
			if cmd.Flags().Changed("repo") ||
				cmd.Flags().Changed("org") ||
				cmd.Flags().Changed("enterprise") ||
				cmd.Flags().Changed("all") {

				return fmt.Errorf("specifying a pool/scaleset ID and any of [all org repo enterprise] are mutually exclusive")
			}
			if _, parseErr := uuid.Parse(args[0]); parseErr == nil {
				listPoolInstancesReq := apiClientInstances.NewListPoolInstancesParams()
				listPoolInstancesReq.PoolID = args[0]
				response, err = apiCli.Instances.ListPoolInstances(listPoolInstancesReq, authToken)
			} else {
				listScaleSetReq := apiClientInstances.NewListScaleSetInstancesParams()
				listScaleSetReq.ScalesetID = args[0]
				response, err = apiCli.Instances.ListScaleSetInstances(listScaleSetReq, authToken)
			}
		case 0:
			if cmd.Flags().Changed("repo") {
				runnerRepo, resErr := resolveRepository(runnerRepository, endpointName)
				if resErr != nil {
					return resErr
				}
				listRepoInstancesReq := apiClientRepos.NewListRepoInstancesParams()
				listRepoInstancesReq.RepoID = runnerRepo
				response, err = apiCli.Repositories.ListRepoInstances(listRepoInstancesReq, authToken)
			} else if cmd.Flags().Changed("org") {
				runnerOrg, resErr := resolveOrganization(runnerOrganization, endpointName)
				if resErr != nil {
					return resErr
				}
				listOrgInstancesReq := apiClientOrgs.NewListOrgInstancesParams()
				listOrgInstancesReq.OrgID = runnerOrg
				response, err = apiCli.Organizations.ListOrgInstances(listOrgInstancesReq, authToken)
			} else if cmd.Flags().Changed("enterprise") {
				runnerEnt, resErr := resolveEnterprise(runnerEnterprise, endpointName)
				if resErr != nil {
					return resErr
				}
				listEnterpriseInstancesReq := apiClientEnterprises.NewListEnterpriseInstancesParams()
				listEnterpriseInstancesReq.EnterpriseID = runnerEnt
				response, err = apiCli.Enterprises.ListEnterpriseInstances(listEnterpriseInstancesReq, authToken)
			} else {
				listInstancesReq := apiClientInstances.NewListInstancesParams()
				response, err = apiCli.Instances.ListInstances(listInstancesReq, authToken)
			}
		default:
			cmd.Help() //nolint
			os.Exit(0)
		}

		if err != nil {
			return err
		}

		instances := response.GetPayload()
		formatInstances(instances, long, true)
		return nil
	},
}

var runnerShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for a runner",
	Long:         `Displays a detailed view of a single runner.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a runner name")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		showInstanceReq := apiClientInstances.NewGetInstanceParams()
		showInstanceReq.InstanceName = args[0]
		response, err := apiCli.Instances.GetInstance(showInstanceReq, authToken)
		if err != nil {
			return err
		}
		formatSingleInstance(response.Payload)
		return nil
	},
}

var runnerDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Remove a runner",
	Aliases: []string{"remove", "rm", "del"},
	Long: `Remove a runner.

This command deletes an existing runner. If it registered in Github
and we recorded an agent ID for it, we will attempt to remove it from
Github first, then mark the runner as pending_delete so it will be
cleaned up by the provider.

NOTE: An active runner cannot be removed from Github. You will have
to either cancel the workflow or wait for it to finish.
`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a runner name")
		}

		deleteInstanceReq := apiClientInstances.NewDeleteInstanceParams()
		deleteInstanceReq.InstanceName = args[0]
		deleteInstanceReq.ForceRemove = &forceRemove
		deleteInstanceReq.BypassGHUnauthorized = &bypassGHUnauthorized
		if err := apiCli.Instances.DeleteInstance(deleteInstanceReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	runnerListCmd.Flags().StringVarP(&runnerRepository, "repo", "r", "", "List all runners from all pools within this repository.")
	runnerListCmd.Flags().StringVarP(&runnerOrganization, "org", "o", "", "List all runners from all pools within this organization.")
	runnerListCmd.Flags().StringVarP(&runnerEnterprise, "enterprise", "e", "", "List all runners from all pools within this enterprise.")
	runnerListCmd.Flags().BoolVarP(&runnerAll, "all", "a", true, "List all runners, regardless of org or repo. (deprecated)")
	runnerListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")
	runnerListCmd.MarkFlagsMutuallyExclusive("repo", "org", "enterprise", "all")
	runnerListCmd.Flags().StringVar(&endpointName, "endpoint", "", "When using the name of an entity, the endpoint must be specified when multiple entities with the same name exist.")

	runnerListCmd.Flags().MarkDeprecated("all", "all runners are listed by default in the absence of --repo, --org or --enterprise.")

	runnerDeleteCmd.Flags().BoolVarP(&forceRemove, "force-remove-runner", "f", false, "Forcefully remove a runner. If set to true, GARM will ignore provider errors when removing the runner.")
	runnerDeleteCmd.Flags().BoolVarP(&bypassGHUnauthorized, "bypass-github-unauthorized", "b", false, "Ignore Unauthorized errors from GitHub and proceed with removing runner from provider and DB. This is useful when credentials are no longer valid and you want to remove your runners. Warning, this has the potential to leave orphaned runners in GitHub. You will need to update your credentials to properly consolidate.")
	runnerDeleteCmd.MarkFlagsMutuallyExclusive("force-remove-runner")

	runnerCmd.AddCommand(
		runnerListCmd,
		runnerShowCmd,
		runnerDeleteCmd,
		agentShellCmd,
	)

	rootCmd.AddCommand(runnerCmd)
}

func formatInstances(param []params.Instance, detailed bool, includeParent bool) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(param)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Nr", "Name", "Status", "Runner Status"}
	if includeParent {
		header = append(header, "Pool / Scale Set")
	}
	if detailed {
		header = append(header, "Created At", "Updated At", "Job Name", "Started At", "Run ID", "Repository")
	}
	t.AppendHeader(header)

	for idx, inst := range param {
		row := table.Row{idx + 1, inst.Name, inst.Status, inst.RunnerStatus}
		if includeParent {
			poolOrScaleSet := fmt.Sprintf("Pool: %v", inst.PoolID)
			if inst.ScaleSetID > 0 {
				poolOrScaleSet = fmt.Sprintf("Scale Set: %d", inst.ScaleSetID)
			}
			row = append(row, poolOrScaleSet)
		}
		if detailed {
			row = append(row, inst.CreatedAt, inst.UpdatedAt)
			if inst.Job != nil {
				repo := fmt.Sprintf("%s/%s", inst.Job.RepositoryOwner, inst.Job.RepositoryName)
				row = append(row, inst.Job.Name, inst.Job.StartedAt, inst.Job.RunID, repo)
			}
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatSingleInstance(instance params.Instance) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(instance)
		return
	}
	t := table.NewWriter()

	header := table.Row{"Field", "Value"}

	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", instance.ID}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Created At", instance.CreatedAt})
	t.AppendRow(table.Row{"Updated At", instance.UpdatedAt})
	t.AppendRow(table.Row{"Provider ID", instance.ProviderID}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Name", instance.Name}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Type", instance.OSType}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Architecture", instance.OSArch}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Name", instance.OSName}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Version", instance.OSVersion}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Status", instance.Status}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Runner Status", instance.RunnerStatus}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Capabilities", fmt.Sprintf("Shell: %v", instance.Capabilities.Shell)}, table.RowConfig{AutoMerge: true})

	if instance.PoolID != "" {
		t.AppendRow(table.Row{"Pool ID", instance.PoolID}, table.RowConfig{AutoMerge: false})
	} else if instance.ScaleSetID != 0 {
		t.AppendRow(table.Row{"Scale Set ID", instance.ScaleSetID}, table.RowConfig{AutoMerge: false})
	}

	if len(instance.Addresses) > 0 {
		for _, addr := range instance.Addresses {
			t.AppendRow(table.Row{"Addresses", addr.Address}, table.RowConfig{AutoMerge: true})
		}
	}

	if len(instance.ProviderFault) > 0 {
		t.AppendRow(table.Row{"Provider Fault", string(instance.ProviderFault)}, table.RowConfig{AutoMerge: true})
	}

	if len(instance.StatusMessages) > 0 {
		for _, msg := range instance.StatusMessages {
			t.AppendRow(table.Row{"Status Updates", fmt.Sprintf("%s: %s", msg.CreatedAt.Format("2006-01-02T15:04:05"), msg.Message)}, table.RowConfig{AutoMerge: true})
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}
