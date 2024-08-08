package cmd

import "github.com/spf13/cobra"

var (
	endpointName        string
	endpointBaseURL     string
	endpointUploadURL   string
	endpointAPIBaseURL  string
	endpointCACertPath  string
	endpointDescription string
)

// githubCmd represents the the github command. This command has a set
// of subcommands that allow configuring and managing GitHub endpoints
// and credentials.
var githubCmd = &cobra.Command{
	Use:          "github",
	Aliases:      []string{"gh"},
	SilenceUsage: true,
	Short:        "Manage GitHub resources",
	Long: `Manage GitHub related resources.

This command allows you to configure and manage GitHub endpoints and credentials`,
	Run: nil,
}

func init() {
	rootCmd.AddCommand(githubCmd)
}
