// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package read

import (
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/read/epochs"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/read/inputs"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/read/outputs"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/read/reports"
	"github.com/cartesi/rollups-node/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "read",
	Short: "Read the node state from the database",
}

var (
	databaseConnection string
)

func init() {
	Cmd.PersistentFlags().StringVar(&databaseConnection, "database-connection", "",
		"Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	cobra.CheckErr(viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.PersistentFlags().Lookup("database-connection")))

	Cmd.AddCommand(epochs.Cmd)
	Cmd.AddCommand(inputs.Cmd)
	Cmd.AddCommand(outputs.Cmd)
	Cmd.AddCommand(reports.Cmd)
}
