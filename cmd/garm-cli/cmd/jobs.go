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
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientJobs "github.com/cloudbase/garm/client/jobs"
	apiClientScaleSets "github.com/cloudbase/garm/client/scalesets"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	jobsAll   bool
	jobsSince string
	jobsUntil string

	webhookJobsAll      bool
	webhookJobsPage     int64
	webhookJobsPageSize int64
	webhookJobsSince    string
	webhookJobsUntil    string
)

// Jobs are recorded in two listings. Jobs that arrive by webhook and may be
// serviced by a pool live in one, jobs GitHub routes to a scale set in the
// other. The job listing shows both and names the source of each row.
const (
	jobSourceWebhook  = "webhook"
	jobSourceScaleSet = "scaleset"
)

// fetchPageSize is the page size used when walking a listing to its end.
const fetchPageSize = 200

// runnerCmd represents the runner command
var jobsCmd = &cobra.Command{
	Use:          "job",
	SilenceUsage: true,
	Short:        "Information about jobs",
	Long:         `Query information about jobs.`,
	Run:          nil,
}

var jobsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List jobs",
	Long: `List the jobs GARM is tracking.

Jobs that arrive by webhook and jobs GitHub routes to a scale set are kept
in separate listings. Both are listed here and the source column tells them
apart. Completed jobs are kept for a day and are left out unless --all is
set. Use "garm-cli webhook job ls" or "garm-cli scaleset job ls" for one
listing on its own, with pagination.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		// Both listings are walked to their end so the table is not cut
		// short by a page boundary.
		listJobsReq := apiClientJobs.NewListJobsParams()
		if jobsAll {
			listJobsReq.All = &jobsAll
		}
		if err := applyJobTimeFlags(jobsSince, jobsUntil, &listJobsReq.Since, &listJobsReq.Until); err != nil {
			return err
		}
		workflowJobs, err := fetchAllJobs(listJobsReq)
		if err != nil {
			return err
		}

		listScaleSetJobsReq := apiClientScaleSets.NewListAllScaleSetJobsParams()
		if jobsAll {
			listScaleSetJobsReq.All = &jobsAll
		}
		if err := applyJobTimeFlags(jobsSince, jobsUntil, &listScaleSetJobsReq.Since, &listScaleSetJobsReq.Until); err != nil {
			return err
		}
		scaleSetJobs, err := fetchAllScaleSetJobs(listScaleSetJobsReq)
		if err != nil {
			return err
		}

		formatJobs(workflowJobs, scaleSetJobs)
		return nil
	},
}

var webhookCmd = &cobra.Command{
	Use:          "webhook",
	SilenceUsage: true,
	Short:        "Information about webhooks",
	Long:         `Query information about data GARM recorded from webhooks.`,
	Run:          nil,
}

var webhookJobsCmd = &cobra.Command{
	Use:          "job",
	SilenceUsage: true,
	Short:        "Information about webhook jobs",
	Long:         `Query information about jobs GARM recorded from webhooks.`,
	Run:          nil,
}

var webhookJobsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List webhook jobs",
	Long: `List jobs GARM recorded from webhooks.

These are the jobs pools act on. Jobs GitHub routes to a scale set are
listed by "garm-cli scaleset job ls". Only queued and in progress jobs are
shown unless --all is set.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listJobsReq := apiClientJobs.NewListJobsParams()
		if err := applyJobListFlags(webhookJobsAll, webhookJobsPage, webhookJobsPageSize, webhookJobsSince, webhookJobsUntil,
			&listJobsReq.All, &listJobsReq.Page, &listJobsReq.PageSize, &listJobsReq.Since, &listJobsReq.Until); err != nil {
			return err
		}
		response, err := apiCli.Jobs.ListJobs(listJobsReq, authToken)
		if err != nil {
			return err
		}
		formatWebhookJobs(response.Payload)
		return nil
	},
}

// nextPage returns the page the listing points at next, or 0 once there is
// nothing left. The listing counts pages in uint64 while the client takes an
// int64, so a page that does not fit is refused rather than truncated. A page
// that does not move forward is also refused, because the walk would never
// end.
func nextPage(current int64, next *uint64) (int64, error) {
	if next == nil {
		return 0, nil
	}
	if *next > math.MaxInt64 {
		return 0, fmt.Errorf("next page %d exceeds max int64 size", *next)
	}
	asInt := int64(*next)
	if asInt <= current {
		return 0, fmt.Errorf("listing points back at page %d while on page %d", asInt, current)
	}
	return asInt, nil
}

// fetchAllJobs walks every page of a webhook job listing.
func fetchAllJobs(req *apiClientJobs.ListJobsParams) ([]params.Job, error) {
	var pageSize int64 = fetchPageSize
	req.PageSize = &pageSize

	jobs := []params.Job{}
	var page int64 = 1
	for {
		req.Page = &page
		response, err := apiCli.Jobs.ListJobs(req, authToken)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, response.Payload.Results...)

		next, err := nextPage(page, response.Payload.NextPage)
		if err != nil {
			return nil, err
		}
		if next == 0 {
			return jobs, nil
		}
		page = next
	}
}

// fetchAllScaleSetJobs walks every page of a scale set job listing.
func fetchAllScaleSetJobs(req *apiClientScaleSets.ListAllScaleSetJobsParams) ([]params.ScaleSetJob, error) {
	var pageSize int64 = fetchPageSize
	req.PageSize = &pageSize

	jobs := []params.ScaleSetJob{}
	var page int64 = 1
	for {
		req.Page = &page
		response, err := apiCli.Scalesets.ListAllScaleSetJobs(req, authToken)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, response.Payload.Results...)

		next, err := nextPage(page, response.Payload.NextPage)
		if err != nil {
			return nil, err
		}
		if next == 0 {
			return jobs, nil
		}
		page = next
	}
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

// applyJobTimeFlags copies the job listing window flags into the generated
// client parameters of a listing request.
func applyJobTimeFlags(since, until string, sinceArg, untilArg **strfmt.DateTime) error {
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

// applyJobListFlags copies the shared job listing flags into the generated
// client parameters of a paginated listing request.
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
	return applyJobTimeFlags(since, until, sinceArg, untilArg)
}

// mergedJob is one row of the merged listing. The two job kinds are stored
// separately and carry different fields, so each is mapped onto the columns
// they have in common.
// We used to shoehorn both job types in the same table. This just mirrors the
// old way we stored jobs, but in the CLI.
type mergedJob struct {
	source     string
	id         string
	name       string
	status     string
	conclusion string
	runner     string
	repo       string
	labels     string
	lockedBy   string
	runURL     string
	createdAt  time.Time
}

func mergedJobRows(workflowJobs []params.Job, scaleSetJobs []params.ScaleSetJob) []mergedJob {
	var rows []mergedJob
	for _, job := range workflowJobs {
		lockedBy := ""
		if job.LockedBy != uuid.Nil {
			lockedBy = job.LockedBy.String()
		}
		rows = append(rows, mergedJob{
			source:     jobSourceWebhook,
			id:         strconv.FormatInt(job.WorkflowJobID, 10),
			name:       job.Name,
			status:     job.Status,
			conclusion: job.Conclusion,
			runner:     job.RunnerName,
			repo:       fmt.Sprintf("%s/%s", job.RepositoryOwner, job.RepositoryName),
			labels:     strings.Join(job.Labels, " "),
			lockedBy:   lockedBy,
			runURL:     job.WorkflowRunURL,
			createdAt:  job.CreatedAt,
		})
	}
	for _, job := range scaleSetJobs {
		rows = append(rows, mergedJob{
			source: jobSourceScaleSet,
			// The record ID, the same one the scale set listing shows.
			// Scale set jobs have no workflow job ID.
			id:         strconv.FormatUint(uint64(job.ID), 10),
			name:       job.Name,
			status:     job.Status,
			conclusion: job.Result,
			runner:     job.RunnerName,
			repo:       fmt.Sprintf("%s/%s", job.RepositoryOwner, job.RepositoryName),
			labels:     strings.Join(job.RequestLabels, " "),
			runURL:     job.WorkflowRunURL,
			createdAt:  job.CreatedAt,
		})
	}

	// Seeing as we're merging jobs from 2 sources in the same table,
	// sort them by creation date.
	// Otherwise the ordering makes little sense.
	sort.SliceStable(rows, func(i, j int) bool {
		if !rows[i].createdAt.Equal(rows[j].createdAt) {
			return rows[i].createdAt.After(rows[j].createdAt)
		}
		return rows[i].name < rows[j].name
	})
	return rows
}

// scaleSetJobAsJob renders a scale set job the way the job listing used to
// print it, back when both kinds shared one table. The field pairs are the
// ones migration 0010 moved across, read in reverse.
//
// The record ID is left out on purpose. The two tables number their rows
// independently, so carrying it over would hand out IDs that collide with
// webhook jobs. Scale set jobs have no workflow job ID either, which is
// what told them apart in this output before.
func scaleSetJobAsJob(job params.ScaleSetJob) params.Job {
	return params.Job{
		RunID:           job.WorkflowRunID,
		Action:          job.EventName,
		Status:          job.Status,
		Conclusion:      job.Result,
		Name:            job.Name,
		StartedAt:       job.RunnerAssignTime,
		CompletedAt:     job.FinishTime,
		GithubRunnerID:  job.RunnerID,
		RunnerName:      job.RunnerName,
		RunnerGroupName: job.RunnerGroupName,
		RepositoryName:  job.RepositoryName,
		RepositoryOwner: job.RepositoryOwner,
		WorkflowRunURL:  job.WorkflowRunURL,
		Labels:          job.RequestLabels,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}
}

// mergedJSONJobs builds the array this command printed before scale set
// jobs moved out, so anything parsing it keeps working. An empty listing
// stays [] rather than becoming null.
func mergedJSONJobs(workflowJobs []params.Job, scaleSetJobs []params.ScaleSetJob) []params.Job {
	jobs := make([]params.Job, 0, len(workflowJobs)+len(scaleSetJobs))
	jobs = append(jobs, workflowJobs...)
	for _, job := range scaleSetJobs {
		jobs = append(jobs, scaleSetJobAsJob(job))
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		if !jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
		}
		return jobs[i].Name < jobs[j].Name
	})
	return jobs
}

func formatJobs(workflowJobs []params.Job, scaleSetJobs []params.ScaleSetJob) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(mergedJSONJobs(workflowJobs, scaleSetJobs))
		return
	}

	rows := mergedJobRows(workflowJobs, scaleSetJobs)
	if len(rows) == 0 {
		fmt.Println("No jobs found.")
		return
	}

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Source", "ID", "Name", "Status", "Conclusion", "Runner Name", "Repository", "Requested Labels", "Locked by", "Workflow job run URL"})
	for _, row := range rows {
		t.AppendRow(table.Row{row.source, row.id, row.name, row.status, row.conclusion, row.runner, row.repo, row.labels, row.lockedBy, row.runURL})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())

	if len(scaleSetJobs) > 0 {
		fmt.Println("Scale set jobs are recorded separately. Use 'garm-cli scaleset job ls' to list only those.")
	}
}

func formatWebhookJobs(jobs params.JobsPaginatedResponse) {
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
	jobsListCmd.Flags().StringVar(&jobsSince, "since", "", "Only list jobs recorded at or after this point. Accepts a duration (24h), an RFC3339 timestamp or YYYY-MM-DD.")
	jobsListCmd.Flags().StringVar(&jobsUntil, "until", "", "Only list jobs recorded at or before this point. Accepts a duration (24h), an RFC3339 timestamp or YYYY-MM-DD.")

	jobsCmd.AddCommand(
		jobsListCmd,
	)
	rootCmd.AddCommand(jobsCmd)

	webhookJobsListCmd.Flags().BoolVar(&webhookJobsAll, "all", false, "Also list completed jobs.")
	webhookJobsListCmd.Flags().Int64Var(&webhookJobsPage, "page", 0, "The page of results to fetch.")
	webhookJobsListCmd.Flags().Int64Var(&webhookJobsPageSize, "page-size", 0, "Number of results per page. The server defaults to 25.")
	webhookJobsListCmd.Flags().StringVar(&webhookJobsSince, "since", "", "Only list jobs recorded at or after this point. Accepts a duration (24h), an RFC3339 timestamp or YYYY-MM-DD.")
	webhookJobsListCmd.Flags().StringVar(&webhookJobsUntil, "until", "", "Only list jobs recorded at or before this point. Accepts a duration (24h), an RFC3339 timestamp or YYYY-MM-DD.")

	webhookJobsCmd.AddCommand(
		webhookJobsListCmd,
	)
	webhookCmd.AddCommand(
		webhookJobsCmd,
	)
	rootCmd.AddCommand(webhookCmd)
}
