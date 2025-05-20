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
package cmd

import "github.com/spf13/cobra"

// giteaCmd represents the the gitea command. This command has a set
// of subcommands that allow configuring and managing Gitea endpoints
// and credentials.
var giteaCmd = &cobra.Command{
	Use:          "gitea",
	Aliases:      []string{"gt"},
	SilenceUsage: true,
	Short:        "Manage Gitea resources",
	Long: `Manage Gitea related resources.

This command allows you to configure and manage Gitea endpoints and credentials`,
	Run: nil,
}

func init() {
	rootCmd.AddCommand(giteaCmd)
}
