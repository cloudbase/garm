package cmd

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:          "debug-log",
	SilenceUsage: true,
	Short:        "Stream garm log",
	Long:         `Stream all garm logging to the terminal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		log.Printf("connecting to %s", u.String())

		header := http.Header{}
		header.Add("Authorization", fmt.Sprintf("Bearer %s", mgr.Token))

		c, _, err := websocket.DefaultDialer.Dial(u.String(), header)
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer c.Close()

		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Print("read:", err)
					return
				}
				log.Print(message)
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
