package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/cloudbase/garm-provider-common/util"
	apiParams "github.com/cloudbase/garm/apiserver/params"
	garmWs "github.com/cloudbase/garm/websocket"
)

var logCmd = &cobra.Command{
	Use:          "debug-log",
	SilenceUsage: true,
	Short:        "Stream garm log",
	Long:         `Stream all garm logging to the terminal.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		parsedURL, err := url.Parse(mgr.BaseURL)
		if err != nil {
			return err
		}

		wsScheme := "ws"
		if parsedURL.Scheme == "https" {
			wsScheme = "wss"
		}
		u := url.URL{Scheme: wsScheme, Host: parsedURL.Host, Path: "/api/v1/ws"}
		slog.Debug("connecting", "url", u.String())

		header := http.Header{}
		header.Add("Authorization", fmt.Sprintf("Bearer %s", mgr.Token))

		c, response, err := websocket.DefaultDialer.Dial(u.String(), header)
		if err != nil {
			var resp apiParams.APIErrorResponse
			var msg string
			var status string
			if response != nil {
				if response.Body != nil {
					if err := json.NewDecoder(response.Body).Decode(&resp); err == nil {
						msg = resp.Details
					}
				}
				status = response.Status
			}
			log.Fatalf("failed to stream logs: %q %s (%s)", err, msg, status)
		}
		defer c.Close()

		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					if garmWs.IsErrorOfInterest(err) {
						slog.With(slog.Any("error", err)).Error("reading log message")
					}
					return
				}
				fmt.Println(util.SanitizeLogEntry(string(message)))
			}
		}()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return nil
			case t := <-ticker.C:
				err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
				if err != nil {
					return err
				}
			case <-interrupt:
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					return err
				}
				select {
				case <-done:
				case <-time.After(time.Second):
				}
				return nil
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
