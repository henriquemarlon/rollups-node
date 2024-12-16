// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
	"time"

	"github.com/cartesi/rollups-node/internal/advancer"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/spf13/cobra"
)

var (
	buildVersion    = "devel"
	advancerService = advancer.Service{}
	createInfo      = advancer.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:                 "advancer",
			EnableSignalHandling: true,
			TelemetryCreate:      true,
			TelemetryAddress:     ":10002",
			Impl:                 &advancerService,
		},
		MaxStartupTime: 10 * time.Second,
		InspectAddress: ":10012",
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
		"telemetry address")
	Cmd.Flags().Var(&createInfo.LogLevel,
		"log-level",
		"log level: debug, info, warn or error")
	Cmd.Flags().BoolVar(&createInfo.LogPretty,
		"log-color", createInfo.LogPretty,
		"tint the logs (colored output)")
	Cmd.Flags().DurationVar(&createInfo.MaxStartupTime,
		"max-startup-time", createInfo.MaxStartupTime,
		"maximum startup time in seconds")
	Cmd.Flags().StringVar(&createInfo.InspectAddress,
		"inspect-address", createInfo.InspectAddress,
		"inspect address")
}

func run(cmd *cobra.Command, args []string) {
	cobra.CheckErr(advancer.Create(&createInfo, &advancerService))
	advancerService.CreateDefaultHandlers("")
	cobra.CheckErr(advancerService.Serve())
}
