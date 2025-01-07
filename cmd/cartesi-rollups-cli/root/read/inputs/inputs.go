// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package inputs

import (
	"encoding/json"
	"fmt"

	cmdcommon "github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/common"
	"github.com/cartesi/rollups-node/internal/repository"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "inputs",
	Short:   "Reads inputs ordered by index",
	Example: examples,
	Run:     run,
}

const examples = `# Read inputs from GraphQL:
cartesi-rollups-cli read inputs -n echo-dapp`

var (
	index uint64
)

func init() {
	Cmd.Flags().Uint64Var(&index, "index", 0,
		"index of the input")
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	if cmdcommon.Repository == nil {
		panic("Repository was not initialized")
	}

	var nameOrAddress string
	pFlags := cmd.Flags()
	if pFlags.Changed("name") {
		nameOrAddress = pFlags.Lookup("name").Value.String()
	} else if pFlags.Changed("address") {
		nameOrAddress = pFlags.Lookup("address").Value.String()
	}

	var result []byte
	if cmd.Flags().Changed("index") {
		inputs, err := cmdcommon.Repository.GetInput(ctx, nameOrAddress, index)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(inputs, "", "    ")
		cobra.CheckErr(err)
	} else {
		inputs, err := cmdcommon.Repository.ListInputs(ctx, nameOrAddress, repository.InputFilter{}, repository.Pagination{})
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(inputs, "", "    ")
		cobra.CheckErr(err)
	}

	fmt.Println(string(result))
}
