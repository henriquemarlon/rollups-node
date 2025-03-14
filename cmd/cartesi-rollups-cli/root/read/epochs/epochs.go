// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package epochs

import (
	"encoding/json"
	"fmt"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/repository/factory"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "epochs",
	Short:   "Reads epochs",
	Example: examples,
	Run:     run,
}

const examples = `# Read all epochs:
cartesi-rollups-cli read epochs -n echo-dapp`

var (
	epochIndex uint64
)

func init() {
	Cmd.Flags().Uint64Var(&epochIndex, "epoch-index", 0, "index of the epoch")
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	var nameOrAddress string
	pFlags := cmd.Flags()
	if pFlags.Changed("name") {
		nameOrAddress = pFlags.Lookup("name").Value.String()
	} else if pFlags.Changed("address") {
		nameOrAddress = pFlags.Lookup("address").Value.String()
	}

	var result []byte
	if cmd.Flags().Changed("epoch-index") {
		epoch, err := repo.GetEpoch(ctx, nameOrAddress, epochIndex)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(epoch, "", "    ")
		cobra.CheckErr(err)
	} else {
		epochs, _, err := repo.ListEpochs(ctx, nameOrAddress, repository.EpochFilter{}, repository.Pagination{})
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(epochs, "", "    ")
		cobra.CheckErr(err)
	}

	fmt.Println(string(result))
}
