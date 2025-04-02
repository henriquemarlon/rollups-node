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
	Use:     "epochs [application-name-or-address]",
	Short:   "Reads epochs",
	Example: examples,
	Args:    cobra.RangeArgs(1, 2), // nolint: mnd
	Run:     run,
}

const examples = `# Read all epochs:
cartesi-rollups-cli read epochs echo-dapp

# Read specific epoch by index:
cartesi-rollups-cli read epochs echo-dapp 2`

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	nameOrAddress, err := config.ToApplicationNameOrAddressFromString(args[0])
	cobra.CheckErr(err)

	dsn, err := config.GetDatabaseConnection()
	cobra.CheckErr(err)

	repo, err := factory.NewRepositoryFromConnectionString(ctx, dsn.String())
	cobra.CheckErr(err)
	defer repo.Close()

	var result []byte
	if len(args) == 1 {
		epochs, _, err := repo.ListEpochs(ctx, nameOrAddress, repository.EpochFilter{}, repository.Pagination{})
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(epochs, "", "    ")
		cobra.CheckErr(err)
	} else {
		epochIndex, err := config.ToUint64FromDecimalOrHexString(args[1])
		if err != nil {
			cobra.CheckErr(fmt.Errorf("invalid value for epoch-index: %w", err))
		}
		epoch, err := repo.GetEpoch(ctx, nameOrAddress, epochIndex)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(epoch, "", "    ")
		cobra.CheckErr(err)
	}

	fmt.Println(string(result))
}
