package cmd

import (
	"context"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/cloudbase/garm/cmd/garm-cli/common"
	garmWs "github.com/cloudbase/garm/websocket"
)

var eventsFilters string

var logCmd = &cobra.Command{
	Use:          "debug-log",
	SilenceUsage: true,
	Short:        "Stream garm log",
	Long:         `Stream all garm logging to the terminal.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), signals...)
		defer stop()

		reader, err := garmWs.NewReader(ctx, mgr.BaseURL, "/api/v1/ws/logs", mgr.Token, common.PrintWebsocketMessage)
		if err != nil {
			return err
		}

		if err := reader.Start(); err != nil {
			return err
		}

		<-reader.Done()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
