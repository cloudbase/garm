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

import (
	"context"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
)

var (
	eventsFilters string
	logLevel      string
	filters       []string
	highlights    []string
	filterMode    string
	enableColor   bool
)

var logCmd = &cobra.Command{
	Use:          "debug-log",
	SilenceUsage: true,
	Short:        "Stream garm log",
	Long:         `Stream all garm logging to the terminal.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), signals...)
		defer stop()

		// Parse filters into map
		attributeFilters := make(map[string]string)
		for _, filter := range filters {
			parts := strings.SplitN(filter, "=", 2)
			if len(parts) == 2 {
				attributeFilters[parts[0]] = parts[1]
			}
		}

		// Parse highlights as key names
		attributeHighlights := make(map[string]bool)
		for _, highlight := range highlights {
			attributeHighlights[highlight] = true
		}

		// Create log formatter with filters and highlights
		logFormatter := common.NewLogFormatter(logLevel, attributeFilters, attributeHighlights, filterMode, enableColor)

		reader, err := garmWs.NewReader(ctx, mgr.BaseURL, "/api/v1/ws/logs", mgr.Token, logFormatter.FormatWebsocketMessage)
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
	logCmd.Flags().StringVar(&logLevel, "log-level", "", "Minimum log level to display (DEBUG, INFO, WARN, ERROR)")
	logCmd.Flags().StringArrayVar(&filters, "filter", []string{}, "Filter logs by attribute (format: key=value) or message content (msg=text). You can specify this option multiple times. The filter will return true for any of the attributes you set.")
	logCmd.Flags().StringArrayVar(&highlights, "highlight", []string{}, "Highlight attribute keys in the output (format: key). You can specify this option multiple times.")
	logCmd.Flags().StringVar(&filterMode, "filter-mode", "any", "How multiple filters are combined: \"any\" (OR, match at least one) or \"all\" (AND, match every filter)")
	logCmd.Flags().BoolVar(&enableColor, "enable-color", true, "Enable color logging (auto-detects terminal support)")

	rootCmd.AddCommand(logCmd)
}
