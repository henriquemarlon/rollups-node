// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package db

import (
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/db/check"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/db/upgrade"
	"github.com/cartesi/rollups-node/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "db",
	Short: "Database management related commands",
}

var (
	databaseConnection string
)

func init() {

	Cmd.Flags().StringVar(&databaseConnection, "database-connection", "", "Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.Flags().Lookup("database-connection"))

	Cmd.AddCommand(upgrade.Cmd)
	Cmd.AddCommand(check.Cmd)
}
