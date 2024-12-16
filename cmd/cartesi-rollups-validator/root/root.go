// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
	"github.com/cartesi/rollups-node/internal/validator"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/spf13/cobra"
)

const CMD_NAME = "validator"

var (
	buildVersion     = "devel"
	validatorService = validator.Service{}
	createInfo       = validator.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:                 "validator",
			EnableSignalHandling: true,
			TelemetryCreate:      true,
			TelemetryAddress:     ":10003",
			Impl:                 &validatorService,
		},
	}
)

var Cmd = &cobra.Command{
	Use:   createInfo.Name,
	Short: "Runs Validator",
	Long:  "Runs Validator in standalone mode",
	Run:   run,
}

func init() {
	createInfo.LoadEnv()
	Cmd.Flags().StringVar(&createInfo.TelemetryAddress,
		"telemetry-address", createInfo.TelemetryAddress,
		"health check and metrics address and port")
	Cmd.Flags().DurationVar(&createInfo.PollInterval,
		"poll-interval", createInfo.PollInterval,
		"poll interval")
	Cmd.Flags().Var(&createInfo.LogLevel,
		"log-level",
		"log level: debug, info, warn or error")
	Cmd.Flags().BoolVar(&createInfo.LogPretty,
		"log-color", createInfo.LogPretty,
		"tint the logs (colored output)")
	Cmd.Flags().StringVar(&createInfo.PostgresEndpoint.Value,
		"postgres-endpoint", createInfo.PostgresEndpoint.Value,
		"Postgres endpoint")
}

func run(cmd *cobra.Command, args []string) {
	cobra.CheckErr(validator.Create(&createInfo, &validatorService))
	validatorService.CreateDefaultHandlers("")
	cobra.CheckErr(validatorService.Serve())
}
