// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
	"github.com/cartesi/rollups-node/internal/claimer"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/spf13/cobra"
)

var (
	// Should be overridden during the final release build with ldflags
	// to contain the actual version number
	buildVersion   = "devel"
	claimerService = claimer.Service{}
	createInfo     = claimer.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:                 "claimer",
			ProcOwner:            true,
			EnableSignalHandling: true,
			TelemetryCreate:      true,
			TelemetryAddress:     ":10003",
			Impl:                 &claimerService,
		},
		EnableSubmission: true,
	}
)

var Cmd = &cobra.Command{
	Use:   createInfo.Name,
	Short: "Runs " + createInfo.Name,
	Long:  "Runs " + createInfo.Name + " in standalone mode",
	Run:   run,
}

func init() {
	createInfo.LoadEnv()
	Cmd.Flags().StringVar(&createInfo.TelemetryAddress,
		"telemetry-address", createInfo.TelemetryAddress,
		"health check and metrics address and port")
	Cmd.Flags().StringVar(&createInfo.BlockchainHttpEndpoint.Value,
		"blockchain-http-endpoint", createInfo.BlockchainHttpEndpoint.Value,
		"blockchain http endpoint")
	Cmd.Flags().DurationVar(&createInfo.PollInterval,
		"poll-interval", createInfo.PollInterval,
		"poll interval")
	Cmd.Flags().Var(&createInfo.LogLevel,
		"log-level",
		"log level: debug, info, warn or error")
	Cmd.Flags().BoolVar(&createInfo.EnableSubmission,
		"claim-submission", createInfo.EnableSubmission,
		"enable or disable claim submission (reader mode)")
}

func run(cmd *cobra.Command, args []string) {
	cobra.CheckErr(claimer.Create(createInfo, &claimerService))
	claimerService.CreateDefaultHandlers("/" + claimerService.Name)
	cobra.CheckErr(claimerService.Serve())
}
