// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package outputs

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cmdcommon "github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/common"
	"github.com/cartesi/rollups-node/internal/repository"
)

var Cmd = &cobra.Command{
	Use:     "outputs",
	Short:   "Reads outputs. If an input index is specified, reads all outputs from that input",
	Example: examples,
	Run:     run,
}

const examples = `# Read all notices:
cartesi-rollups-cli read outputs -n echo-dapp`

var (
	outputIndex uint64
	inputIndex  uint64
)

func init() {
	Cmd.Flags().Uint64Var(&inputIndex, "input-index", 0,
		"filter by input index")

	Cmd.Flags().Uint64Var(&outputIndex, "output-index", 0,
		"filter by output index")
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
	if cmd.Flags().Changed("output-index") {
		if cmd.Flags().Changed("input-index") {
			fmt.Fprintf(os.Stderr, "Error: Only one of 'output-index' or 'input-index' can be used at a time.\n")
			os.Exit(1)
		}
		outputs, err := cmdcommon.Repository.GetOutput(ctx, nameOrAddress, outputIndex)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(outputs, "", "    ")
		cobra.CheckErr(err)
	} else if cmd.Flags().Changed("input-index") {
		f := repository.OutputFilter{InputIndex: &inputIndex}
		p := repository.Pagination{}
		outputs, err := cmdcommon.Repository.ListOutputs(ctx, nameOrAddress, f, p)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(outputs, "", "    ")
		cobra.CheckErr(err)
	} else {
		f := repository.OutputFilter{}
		p := repository.Pagination{}
		outputs, err := cmdcommon.Repository.ListOutputs(ctx, nameOrAddress, f, p)
		cobra.CheckErr(err)
		result, err = json.MarshalIndent(outputs, "", "    ")
		cobra.CheckErr(err)
	}

	fmt.Println(string(result))
}
