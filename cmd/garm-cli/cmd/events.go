package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/cloudbase/garm-provider-common/util"
	garmWs "github.com/cloudbase/garm/websocket"
)

var eventsCmd = &cobra.Command{
	Use:          "debug-events",
	SilenceUsage: true,
	Short:        "Stream garm events",
	Long:         `Stream all garm events to the terminal.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		conn, err := getWebsocketConnection("/api/v1/events")
		if err != nil {
			return err
		}
		defer conn.Close()

		done := make(chan struct{})

		go func() {
			defer close(done)
			conn.SetReadDeadline(time.Now().Add(pongWait))
			conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					if garmWs.IsErrorOfInterest(err) {
						slog.With(slog.Any("error", err)).Error("reading event message")
					}
					return
				}
				fmt.Println(util.SanitizeLogEntry(string(message)))
			}
		}()

		if eventsFilters != "" {
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			err = conn.WriteMessage(websocket.TextMessage, []byte(eventsFilters))
			if err != nil {
				return err
			}
		}

		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				slog.Info("done")
				return nil
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					return err
				}
			case <-interrupt:
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					return err
				}
				slog.Info("waiting for server to close connection")
				select {
				case <-done:
					slog.Info("done")
				case <-time.After(time.Second):
					slog.Info("timeout")
				}
				return nil
			}
		}
	},
}

func init() {
	eventsCmd.Flags().StringVarP(&eventsFilters, "filters", "m", "", "Json with event filters you want to apply")
	rootCmd.AddCommand(eventsCmd)
}
