// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package root

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/node"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/spf13/cobra"
)

const CMD_NAME = "node"

var (
	// Should be overridden during the final release build with ldflags
	// to contain the actual version number
	buildVersion = "devel"
	Cmd          = &cobra.Command{
		Use:   CMD_NAME,
		Short: "Runs the Cartesi Rollups Node",
		Long:  "Runs the Cartesi Rollups Node as a single process",
		RunE:  run,
	}
	enableClaimSubmission bool
)

func init() {
	Cmd.Flags().BoolVar(&enableClaimSubmission,
		"claim-submission", true,
		"enable or disable claim submission (reader mode)")
}

func run(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.FromEnv()
	if cmd.Flags().Lookup("claim-submission").Changed {
		cfg.FeatureClaimSubmissionEnabled = enableClaimSubmission
		if enableClaimSubmission && cfg.Auth == nil {
			cfg.Auth = config.AuthFromEnv()
		}
	}

	database, err := repository.Connect(ctx, cfg.PostgresEndpoint.Value)
	if err != nil {
		slog.Error("Node couldn't connect to the database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// create the node supervisor
	supervisor, err := node.Setup(ctx, cfg, database)
	if err != nil {
		slog.Error("Node exited with an error", "error", err)
		os.Exit(1)
	}

	// logs startup time
	ready := make(chan struct{}, 1)
	go func() {
		select {
		case <-ready:
			duration := time.Since(startTime)
			slog.Info("Node is ready", "after", duration)
		case <-ctx.Done():
		}
	}()

	// start supervisor
	if err := supervisor.Start(ctx, ready); err != nil {
		slog.Error("Node exited with an error", "error", err)
		os.Exit(1)
	}

	return err
}
