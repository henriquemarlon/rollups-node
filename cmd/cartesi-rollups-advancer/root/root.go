// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
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
			ProcOwner:            true,
			EnableSignalHandling: true,
			TelemetryCreate:      true,
			TelemetryAddress:     ":10001",
			Impl:                 &advancerService,
		},
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
	Cmd.Flags().Var(&createInfo.LogLevel,
		"log-level",
		"log level: debug, info, warn or error")
}

func run(cmd *cobra.Command, args []string) {
	cobra.CheckErr(advancer.Create(&createInfo, &advancerService))
	advancerService.CreateDefaultHandlers("/" + advancerService.Name)
	cobra.CheckErr(advancerService.Serve())
}
