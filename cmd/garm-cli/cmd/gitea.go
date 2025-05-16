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
