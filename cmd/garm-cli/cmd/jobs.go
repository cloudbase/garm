// Copyright 2023 Cloudbase Solutions SRL
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
	"fmt"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientJobs "github.com/cloudbase/garm/client/jobs"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	jobsAll      bool
	jobsPage     int64
	jobsPageSize int64
	jobsSince    string
	jobsUntil    string
)

// runnerCmd represents the runner command
var jobsCmd = &cobra.Command{
	Use:          "job",
	SilenceUsage: true,
	Short:        "Information about jobs",
	Long:         `Query information about jobs.`,
	Run:          nil,
}

var jobsListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List jobs",
	Long:         `List jobs currently recorded in the system. Only queued and in progress jobs are shown unless --all is set.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listJobsReq := apiClientJobs.NewListJobsParams()
		if err := applyJobListFlags(jobsAll, jobsPage, jobsPageSize, jobsSince, jobsUntil,
			&listJobsReq.All, &listJobsReq.Page, &listJobsReq.PageSize, &listJobsReq.Since, &listJobsReq.Until); err != nil {
			return err
		}
		response, err := apiCli.Jobs.ListJobs(listJobsReq, authToken)
		if err != nil {
			return err
		}
		formatJobs(response.Payload)
		return nil
	},
}

// parseTimeFlag turns a --since or --until value into a timestamp. It
// accepts a duration relative to now (24h), an RFC3339 timestamp or a date.
func parseTimeFlag(val string) (time.Time, error) {
	if d, err := time.ParseDuration(val); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q, use a duration (24h), an RFC3339 timestamp or YYYY-MM-DD", val)
}

// applyJobListFlags copies the shared job listing flags into the generated
// client parameters of a listing request.
func applyJobListFlags(all bool, page, pageSize int64, since, until string, allArg **bool, pageArg, pageSizeArg **int64, sinceArg, untilArg **strfmt.DateTime) error {
	if all {
		*allArg = &all
	}
	if page > 0 {
		*pageArg = &page
	}
	if pageSize > 0 {
		*pageSizeArg = &pageSize
	}
	if since != "" {
		ts, err := parseTimeFlag(since)
		if err != nil {
			return err
		}
		dt := strfmt.DateTime(ts)
		*sinceArg = &dt
	}
	if until != "" {
		ts, err := parseTimeFlag(until)
		if err != nil {
			return err
		}
		dt := strfmt.DateTime(ts)
		*untilArg = &dt
	}
	return nil
}

func formatJobs(jobs params.JobsPaginatedResponse) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(jobs)
		return
	}
	if len(jobs.Results) == 0 {
		fmt.Println("No jobs found.")
		return
	}
	t := table.NewWriter()
	header := table.Row{"Workflow Job ID", "Name", "Status", "Conclusion", "Runner Name", "Repository", "Requested Labels", "Locked by", "Workflow job run URL"}
	pageHeader, pageHeaderCfg := paginatedListHeader(len(header), jobs.TotalCount, jobs.Pages, jobs.CurrentPage)
	t.AppendHeader(pageHeader, pageHeaderCfg)
	t.AppendHeader(header)

	for _, job := range jobs.Results {
		lockedBy := ""
		repo := fmt.Sprintf("%s/%s", job.RepositoryOwner, job.RepositoryName)
		if job.LockedBy != uuid.Nil {
			lockedBy = job.LockedBy.String()
		}
		t.AppendRow(table.Row{job.WorkflowJobID, job.Name, job.Status, job.Conclusion, job.RunnerName, repo, strings.Join(job.Labels, " "), lockedBy, job.WorkflowRunURL})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func init() {
	jobsListCmd.Flags().BoolVar(&jobsAll, "all", false, "Also list completed jobs.")
	jobsListCmd.Flags().Int64Var(&jobsPage, "page", 0, "The page of results to fetch.")
	jobsListCmd.Flags().Int64Var(&jobsPageSize, "page-size", 0, "Number of results per page. The server defaults to 25.")
	jobsListCmd.Flags().StringVar(&jobsSince, "since", "", "Only list jobs recorded at or after this point. Accepts a duration (24h), an RFC3339 timestamp or YYYY-MM-DD.")
	jobsListCmd.Flags().StringVar(&jobsUntil, "until", "", "Only list jobs recorded at or before this point. Accepts a duration (24h), an RFC3339 timestamp or YYYY-MM-DD.")

	jobsCmd.AddCommand(
		jobsListCmd,
	)

	rootCmd.AddCommand(jobsCmd)
}
