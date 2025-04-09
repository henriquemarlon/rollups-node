// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)
package db

import (
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/db/check"
	"github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/db/init"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "db",
	Short: "Database management related commands",
}

func init() {
	Cmd.AddCommand(initialize.Cmd)
	Cmd.AddCommand(check.Cmd)
}
