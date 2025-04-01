// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package db

import (
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/db/check"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/db/init"
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

	Cmd.PersistentFlags().StringVar(&databaseConnection, "database-connection", "",
		"Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	cobra.CheckErr(viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.PersistentFlags().Lookup("database-connection")))

	Cmd.AddCommand(initialize.Cmd)
	Cmd.AddCommand(check.Cmd)
}
