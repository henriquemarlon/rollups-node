// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
	"context"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/node"
	"github.com/cartesi/rollups-node/internal/repository/factory"
	"github.com/cartesi/rollups-node/internal/version"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const serviceName = "node"

var (
	logLevel               string
	logColor               bool
	defaultBlockString     string
	blockchainHttpEndpoint string
	blockchainWsEndpoint   string
	databaseConnection     string
	advancerPollInterval   string
	validatorPollInterval  string
	claimerPollInterval    string
	maxStartupTime         string
	inputBoxAddress        string
	inputBoxBlockNumber    uint64
	enableInputReader      bool
	enableInspect          bool
	enableJsonrpc          bool
	enableSubmission       bool
	enableMachineHashCheck bool
	jsonrpcApiAddress      string
	inspectAddress         string
	telemetryAddress       string
	machinelogLevel        string
	cfg                    *config.Config
)

var Cmd = &cobra.Command{
	Use:     "cartesi-rollups-" + serviceName,
	Short:   "Runs cartesi-rollups-" + serviceName,
	Long:    "Runs cartesi-rollups-" + serviceName + " in standalone mode",
	Run:     run,
	Version: version.BuildVersion,
}

func init() {
	Cmd.Flags().StringVarP(&defaultBlockString, "default-block", "d", "finalized", "Default block to be used when fetching new blocks.\nOne of 'latest', 'safe', 'pending', 'finalized'")
	viper.BindPFlag(config.BLOCKCHAIN_DEFAULT_BLOCK, Cmd.Flags().Lookup("default-block"))

	Cmd.Flags().StringVar(&inputBoxAddress, "inputbox-address", "", "Input Box contract address")
	viper.BindPFlag(config.CONTRACTS_INPUT_BOX_ADDRESS, Cmd.Flags().Lookup("inputbox-address"))

	Cmd.Flags().Uint64Var(&inputBoxBlockNumber, "inputbox-block-number", 0, "Input Box deployment block number")
	viper.BindPFlag(config.CONTRACTS_INPUT_BOX_DEPLOYMENT_BLOCK_NUMBER, Cmd.Flags().Lookup("inputbox-block-number"))

	Cmd.Flags().StringVar(&jsonrpcApiAddress, "jsonrpc-address", ":10011", "Jsonrpc API service address and port")
	viper.BindPFlag(config.JSONRPC_API_ADDRESS, Cmd.Flags().Lookup("jsonrpc-address"))

	Cmd.Flags().StringVar(&inspectAddress, "inspect-address", ":10012", "Inspect service address and port")
	viper.BindPFlag(config.INSPECT_ADDRESS, Cmd.Flags().Lookup("inspect-address"))

	Cmd.Flags().StringVar(&telemetryAddress, "telemetry-address", ":10000", "Health check and metrics address and port")
	viper.BindPFlag(config.TELEMETRY_ADDRESS, Cmd.Flags().Lookup("telemetry-address"))

	Cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn or error")
	viper.BindPFlag(config.LOG_LEVEL, Cmd.Flags().Lookup("log-level"))

	Cmd.Flags().BoolVar(&logColor, "log-color", true, "Tint the logs (colored output)")
	viper.BindPFlag(config.LOG_COLOR, Cmd.Flags().Lookup("log-color"))

	Cmd.Flags().StringVar(&databaseConnection, "database-connection", "", "Database connection string in the URL format\n(eg.: 'postgres://user:password@hostname:port/database') ")
	viper.BindPFlag(config.DATABASE_CONNECTION, Cmd.Flags().Lookup("database-connection"))

	Cmd.Flags().StringVar(&blockchainHttpEndpoint, "blockchain-http-endpoint", "", "Blockchain HTTP endpoint")
	viper.BindPFlag(config.BLOCKCHAIN_HTTP_ENDPOINT, Cmd.Flags().Lookup("blockchain-http-endpoint"))

	Cmd.Flags().StringVar(&blockchainWsEndpoint, "blockchain-ws-endpoint", "", "Blockchain WS Endpoint")
	viper.BindPFlag(config.BLOCKCHAIN_WS_ENDPOINT, Cmd.Flags().Lookup("blockchain-ws-endpoint"))

	Cmd.Flags().StringVar(&advancerPollInterval, "advancer-poll-interval", "3", "Advancer poll interval")
	viper.BindPFlag(config.ADVANCER_POLLING_INTERVAL, Cmd.Flags().Lookup("advancer-poll-interval"))

	Cmd.Flags().StringVar(&validatorPollInterval, "validator-poll-interval", "3", "Validator poll interval")
	viper.BindPFlag(config.VALIDATOR_POLLING_INTERVAL, Cmd.Flags().Lookup("validator-poll-interval"))

	Cmd.Flags().StringVar(&claimerPollInterval, "claimer-poll-interval", "3", "Claimer poll interval")
	viper.BindPFlag(config.CLAIMER_POLLING_INTERVAL, Cmd.Flags().Lookup("claimer-poll-interval"))

	Cmd.Flags().StringVar(&maxStartupTime, "max-startup-time", "15", "Maximum startup time in seconds")
	viper.BindPFlag(config.MAX_STARTUP_TIME, Cmd.Flags().Lookup("max-startup-time"))

	Cmd.Flags().BoolVar(&enableInputReader, "input-reader", true, "Enable or disable the input reader (for external input readers)")
	viper.BindPFlag(config.FEATURE_INPUT_READER_ENABLED, Cmd.Flags().Lookup("input-reader"))

	Cmd.Flags().BoolVar(&enableInspect, "inspect-enabled", true, "Enable or disable the inspect service")
	viper.BindPFlag(config.FEATURE_INSPECT_ENABLED, Cmd.Flags().Lookup("inspect-enabled"))

	Cmd.Flags().BoolVar(&enableJsonrpc, "jsonrpc-enabled", true, "Enable or disable the jsonrpc api service")
	viper.BindPFlag(config.FEATURE_JSONRPC_API_ENABLED, Cmd.Flags().Lookup("jsonrpc-enabled"))

	Cmd.Flags().BoolVar(&enableMachineHashCheck, "machine-hash-check", true, "Enable or disable machine hash check (DO NOT USE IN PRODUCTION)")
	viper.BindPFlag(config.FEATURE_MACHINE_HASH_CHECK_ENABLED, Cmd.Flags().Lookup("machine-hash-check"))

	Cmd.Flags().BoolVar(&enableSubmission, "claim-submission", true, "Enable or disable claim submission (reader mode)")
	viper.BindPFlag(config.FEATURE_CLAIM_SUBMISSION_ENABLED, Cmd.Flags().Lookup("claim-submission"))

	Cmd.Flags().StringVar(&machinelogLevel, "machine-log-level", "info",
		"Remote Machine log level: trace, debug, info, warning, error, fatal")
	viper.BindPFlag(config.REMOTE_MACHINE_LOG_LEVEL, Cmd.Flags().Lookup("machine-log-level"))

	// TODO: validate on preRunE
	Cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		return nil
	}
}

func run(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.MaxStartupTime)
	defer cancel()

	createInfo := node.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:                 serviceName,
			LogLevel:             cfg.LogLevel,
			LogColor:             cfg.LogColor,
			EnableSignalHandling: true,
			TelemetryCreate:      true,
			TelemetryAddress:     cfg.TelemetryAddress,
		},
		Config: *cfg,
	}

	var err error
	createInfo.ReaderClient, err = ethclient.DialContext(ctx, cfg.BlockchainHttpEndpoint.String())
	cobra.CheckErr(err)

	createInfo.ReaderWSClient, err = ethclient.DialContext(ctx, cfg.BlockchainWsEndpoint.String())
	cobra.CheckErr(err)

	createInfo.ClaimerClient, err = ethclient.DialContext(ctx, cfg.BlockchainHttpEndpoint.String())
	cobra.CheckErr(err)

	createInfo.Repository, err = factory.NewRepositoryFromConnectionString(ctx, cfg.DatabaseConnection.String())
	cobra.CheckErr(err)
	defer createInfo.Repository.Close()

	nodeService, err := node.Create(ctx, &createInfo)
	cobra.CheckErr(err)

	cobra.CheckErr(nodeService.Serve())
}
