// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package app

import (
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/app/deploy"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/app/execution-parameters"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/app/list"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/app/register"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/app/remove"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/app/status"
	"github.com/cartesi/rollups-node/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "app",
	Short: "Application management related commands",
}

var (
	databaseConnection string
)

func init() {

	Cmd.Flags().StringVar(&databaseConnection, "database-connection", "",
		"Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	err := viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.Flags().Lookup("database-connection"))
	cobra.CheckErr(err)

	Cmd.AddCommand(register.Cmd)
	Cmd.AddCommand(deploy.Cmd)
	Cmd.AddCommand(list.Cmd)
	Cmd.AddCommand(status.Cmd)
	Cmd.AddCommand(remove.Cmd)
	Cmd.AddCommand(execution.Cmd)
}
