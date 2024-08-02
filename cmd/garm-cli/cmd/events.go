package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/cloudbase/garm/cmd/garm-cli/common"
	garmWs "github.com/cloudbase/garm/websocket"
)

var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

var eventsCmd = &cobra.Command{
	Use:          "debug-events",
	SilenceUsage: true,
	Short:        "Stream garm events",
	Long:         `Stream all garm events to the terminal.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), signals...)
		defer stop()

		reader, err := garmWs.NewReader(ctx, mgr.BaseURL, "/api/v1/ws/events", mgr.Token, common.PrintWebsocketMessage)
		if err != nil {
			return err
		}

		if err := reader.Start(); err != nil {
			return err
		}

		if eventsFilters != "" {
			if err := reader.WriteMessage(websocket.TextMessage, []byte(eventsFilters)); err != nil {
				return err
			}
		}
		<-reader.Done()
		return nil
	},
}

func init() {
	eventsCmd.Flags().StringVarP(&eventsFilters, "filters", "m", "", "Json with event filters you want to apply")
	rootCmd.AddCommand(eventsCmd)
}
