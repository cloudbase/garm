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
	"fmt"

	"github.com/spf13/cobra"
)

// repoPoolCmd represents the pool command
var repoInstancesCmd = &cobra.Command{
	Use:          "runner",
	SilenceUsage: true,
	Short:        "List runners",
	Long:         `List runners from all pools defined in this repository.`,
	Run:          nil,
}

var repoRunnerListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List repository runners",
	Long:         `List all runners for a given repository.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		instances, err := cli.ListRepoInstances(args[0])
		if err != nil {
			return err
		}
		formatInstances(instances)
		return nil
	},
}

func init() {
	repoInstancesCmd.AddCommand(
		repoRunnerListCmd,
	)

	repositoryCmd.AddCommand(repoInstancesCmd)
}
