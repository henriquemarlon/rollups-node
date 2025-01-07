// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package register

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cmdcommon "github.com/cartesi/rollups-node/cmd/cartesi-rollups-cli/root/common"
	"github.com/cartesi/rollups-node/internal/advancer/snapshot"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "register",
	Short:   "Register an existing application on the node",
	Example: examples,
	Run:     run,
}

const examples = `# Adds an application to Rollups Node:
cartesi-rollups-cli app register -n echo-dapp -a 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF -c 0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` //nolint:lll

var (
	name                string
	applicationAddress  string
	consensusAddress    string
	templatePath        string
	templateHash        string
	inputBoxBlockNumber uint64
	disabled            bool
	printAsJSON         bool
)

func init() {
	Cmd.Flags().StringVarP(
		&name,
		"name",
		"n",
		"",
		"Application name",
	)
	cobra.CheckErr(Cmd.MarkFlagRequired("name"))

	Cmd.Flags().StringVarP(
		&applicationAddress,
		"address",
		"a",
		"",
		"Application contract address",
	)
	cobra.CheckErr(Cmd.MarkFlagRequired("address"))

	Cmd.Flags().StringVarP(
		&consensusAddress,
		"consensus",
		"c",
		"",
		"Application IConsensus Address",
	)
	cobra.CheckErr(Cmd.MarkFlagRequired("consensus"))

	Cmd.Flags().StringVarP(
		&templatePath,
		"template-path",
		"t",
		"",
		"Application template URI",
	)
	cobra.CheckErr(Cmd.MarkFlagRequired("template-path"))

	Cmd.Flags().StringVarP(
		&templateHash,
		"template-hash",
		"H",
		"",
		"Application template hash. If not provided, it will be read from the template URI",
	)

	Cmd.Flags().Uint64VarP(
		&inputBoxBlockNumber,
		"inputbox-block-number",
		"i",
		0,
		"InputBox deployment block number",
	)

	Cmd.Flags().BoolVarP(
		&disabled,
		"disabled",
		"d",
		false,
		"Sets the application state to disabled",
	)

	Cmd.Flags().BoolVarP(
		&printAsJSON,
		"print-json",
		"j",
		false,
		"Prints the application data as JSON",
	)
}

func run(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	if cmdcommon.Repository == nil {
		panic("Database was not initialized")
	}

	applicationState := model.ApplicationState_Enabled
	if disabled {
		applicationState = model.ApplicationState_Disabled
	}

	if templateHash == "" {
		var err error
		templateHash, err = snapshot.ReadHash(templatePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Read machine template hash failed: %v\n", err)
			os.Exit(1)
		}
	}

	application := model.Application{
		Name:                 name,
		IApplicationAddress:  strings.ToLower(common.HexToAddress(applicationAddress).String()),
		IConsensusAddress:    strings.ToLower(common.HexToAddress(consensusAddress).String()),
		TemplateURI:          templatePath,
		TemplateHash:         common.HexToHash(templateHash),
		State:                applicationState,
		LastProcessedBlock:   inputBoxBlockNumber,
		LastOutputCheckBlock: inputBoxBlockNumber,
		LastClaimCheckBlock:  inputBoxBlockNumber,
	}

	_, err := cmdcommon.Repository.CreateApplication(ctx, &application)
	cobra.CheckErr(err)

	if printAsJSON {
		jsonData, err := json.Marshal(application)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling application to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
	} else {
		fmt.Printf("Application %v successfully registered\n", application.IApplicationAddress)
	}
}
