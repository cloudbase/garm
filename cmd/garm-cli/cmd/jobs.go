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

	"github.com/cloudbase/garm/params"
	"github.com/google/uuid"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
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
	Long:         `List all jobs currently recorded in the system.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		jobs, err := cli.ListAllJobs()
		if err != nil {
			return err
		}
		formatJobs(jobs)
		return nil
	},
}

func formatJobs(jobs []params.Job) {
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Status", "Conclusion", "Runner Name", "Locked by"}
	t.AppendHeader(header)

	for _, job := range jobs {
		lockedBy := ""
		if job.LockedBy != uuid.Nil {
			lockedBy = job.LockedBy.String()
		}
		t.AppendRow(table.Row{job.ID, job.Name, job.Status, job.Conclusion, job.RunnerName, lockedBy})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func init() {
	jobsCmd.AddCommand(
		jobsListCmd,
	)

	rootCmd.AddCommand(jobsCmd)
}
