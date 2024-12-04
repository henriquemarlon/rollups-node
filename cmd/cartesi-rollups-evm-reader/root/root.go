// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
	"time"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/evmreader"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
)

var (
	// Should be overridden during the final release build with ldflags
	// to contain the actual version number
	buildVersion  = "devel"
	readerService = evmreader.Service{}
	createInfo    = evmreader.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:                 "evm-reader",
			ProcOwner:            true,
			EnableSignalHandling: true,
			TelemetryCreate:      true,
			TelemetryAddress:     ":10000",
			Impl:                 &readerService,
		},
		EvmReaderPersistentConfig: model.EvmReaderPersistentConfig{
			DefaultBlock: model.DefaultBlockStatusSafe,
		},
		MaxStartupTime: 10 * time.Second,
	}
	inputBoxAddress    service.EthAddress
	DefaultBlockString = "safe"
)

var Cmd = &cobra.Command{
	Use:   createInfo.Name,
	Short: "Runs " + createInfo.Name,
	Long:  "Runs " + createInfo.Name + " in standalone mode",
	Run:   run,
}

func init() {
	createInfo.LoadEnv()

	Cmd.Flags().StringVarP(&DefaultBlockString,
		"default-block", "d", DefaultBlockString,
		`Default block to be used when fetching new blocks.
		One of 'latest', 'safe', 'pending', 'finalized'`)

	Cmd.Flags().StringVarP(&createInfo.PostgresEndpoint.Value,
		"postgres-endpoint",
		"p",
		createInfo.PostgresEndpoint.Value,
		"Postgres endpoint")

	Cmd.Flags().StringVarP(&createInfo.BlockchainHttpEndpoint.Value,
		"blockchain-http-endpoint",
		"b",
		createInfo.BlockchainHttpEndpoint.Value,
		"Blockchain HTTP Endpoint")

	Cmd.Flags().StringVarP(&createInfo.BlockchainWsEndpoint.Value,
		"blockchain-ws-endpoint",
		"w",
		createInfo.BlockchainWsEndpoint.Value,
		"Blockchain WS Endpoint")

	Cmd.Flags().Var(&inputBoxAddress,
		"inputbox-address",
		"Input Box contract address")

	Cmd.Flags().Uint64VarP(&createInfo.InputBoxDeploymentBlock,
		"inputbox-block-number",
		"n",
		0,
		"Input Box deployment block number")
	Cmd.Flags().Var(&createInfo.LogLevel,
		"log-level",
		"log level: debug, info, warn or error")
	Cmd.Flags().BoolVar(&createInfo.LogPretty,
		"log-color", createInfo.LogPretty,
		"tint the logs (colored output)")
	Cmd.Flags().DurationVar(&createInfo.MaxStartupTime,
		"max-startup-time", createInfo.MaxStartupTime,
		"maximum startup time in seconds")
}

func run(cmd *cobra.Command, args []string) {
	if cmd.Flags().Changed("default-block") {
		var err error
		createInfo.DefaultBlock, err = config.ToDefaultBlockFromString(DefaultBlockString)
		cobra.CheckErr(err)
	}
	if cmd.Flags().Changed("inputbox-address") {
		createInfo.InputBoxAddress = common.Address(inputBoxAddress)
	}

	cobra.CheckErr(evmreader.Create(&createInfo, &readerService))
	readerService.CreateDefaultHandlers("/" + readerService.Name)
	cobra.CheckErr(readerService.Serve())
}
