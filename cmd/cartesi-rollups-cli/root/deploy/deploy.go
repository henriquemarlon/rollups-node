// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package deploy

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var (
	epochLengthParam uint64
	saltParam        string
	asJson           bool
)

var Cmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy Contracts and Applications",
	Run:   run,
}

func init() {
	Cmd.PersistentFlags().Uint64VarP(&epochLengthParam, "epoch-length", "", 10, // nolint: mnd
		"Epoch length")
	Cmd.PersistentFlags().StringVar(&saltParam, "salt", "0000000000000000000000000000000000000000000000000000000000000000",
		"Salt value for contract deployment")
	Cmd.PersistentFlags().BoolVarP(&asJson, "json", "", false,
		"Print results as JSON")

	Cmd.AddCommand(applicationCmd)
	Cmd.AddCommand(authorityCmd)
}

func run(cmd *cobra.Command, args []string) {
	// If no subcommand is provided, show help
	err := cmd.Help()
	cobra.CheckErr(err)
}

// parse common.Address with error checking
func parseHexAddress(address string) (common.Address, error) {
	if !common.IsHexAddress(address) {
		return common.Address{}, fmt.Errorf("failed to parse hex address")
	}
	return common.HexToAddress(address), nil
}
