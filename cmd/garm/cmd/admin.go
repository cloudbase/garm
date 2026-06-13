// Copyright 2026 Cloudbase Solutions SRL
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
	"context"
	"errors"
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nbutton23/zxcvbn-go"
	"github.com/spf13/cobra"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

var newPassword string

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands for managing the GARM server",
}

var adminListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List admin users",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		db, err := openDatabase(ctx)
		if err != nil {
			return err
		}

		user, err := db.GetAdminUser(ctx)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				fmt.Println("No admin users found.")
				return nil
			}
			return fmt.Errorf("fetching admin user: %w", err)
		}

		t := table.NewWriter()
		t.AppendHeader(table.Row{"ID", "Username", "Email", "Enabled"})
		t.AppendRow(table.Row{user.ID, user.Username, user.Email, user.Enabled})
		fmt.Println(t.Render())
		return nil
	},
}

var adminPasswordResetCmd = &cobra.Command{
	Use:          "password-reset [flags] <username>",
	Short:        "Reset the password for an admin user",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		username := args[0]
		ctx := cmd.Context()

		db, err := openDatabase(ctx)
		if err != nil {
			return err
		}

		user, err := db.GetUser(ctx, username)
		if err != nil {
			return fmt.Errorf("fetching user %q: %w", username, err)
		}
		if !user.IsAdmin {
			return fmt.Errorf("user %q is not an admin", username)
		}

		strength := zxcvbn.PasswordStrength(newPassword, nil)
		if strength.Score < 4 {
			return fmt.Errorf("password is too weak (score: %d/4, minimum required: 4)", strength.Score)
		}

		hashed, err := util.PaswsordToBcrypt(newPassword)
		if err != nil {
			return fmt.Errorf("hashing password: %w", err)
		}

		if _, err := db.UpdateUser(ctx, username, params.UpdateUserParams{
			Password: hashed,
		}); err != nil {
			return fmt.Errorf("updating password: %w", err)
		}

		fmt.Printf("Password for user %q has been reset successfully.\n", username)
		return nil
	},
}

func init() {
	adminPasswordResetCmd.Flags().StringVar(&newPassword, "new-password", "", "new password for the admin user")
	if err := adminPasswordResetCmd.MarkFlagRequired("new-password"); err != nil {
		panic(err)
	}

	adminCmd.AddCommand(adminListCmd)
	adminCmd.AddCommand(adminPasswordResetCmd)
	rootCmd.AddCommand(adminCmd)
}

// openDatabase loads the GARM config and opens a direct database connection.
func openDatabase(ctx context.Context) (dbCommon.Store, error) {
	cfg, err := config.NewConfig(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	watcher.InitWatcher(ctx)

	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return db, nil
}
