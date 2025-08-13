// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package main

import (
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/cartesi/rollups-node/internal/repository/postgres/schema"
)

func main() {
	var s *schema.Schema
	var err error

	dbConnString, ok := os.LookupEnv("CARTESI_DATABASE_CONNECTION")
	if !ok || dbConnString == "" {
		slog.Error("Error: CARTESI_DATABASE_CONNECTION not defined")
		os.Exit(1)
	}

	uri, err := url.Parse(dbConnString)
	if err != nil {
		slog.Error("Failed to parse database connection.", "error", err)
		os.Exit(1)
	}

	for i := 0; i < 5; i++ {
		s, err = schema.New(uri.String())
		if err == nil {
			break
		}
		if i == 4 {
			slog.Error("Failed to connect to database.", "error", err)
			os.Exit(1)
		}
		slog.Warn("Connection to database failed. Trying again.", "PostgresEndpoint", uri.Redacted())
		time.Sleep(5 * time.Second) // wait before retrying
	}
	defer s.Close()

	err = s.Upgrade()
	if err != nil {
		slog.Error("Error while upgrading database schema", "error", err)
		os.Exit(1)
	}

	version, err := s.ValidateVersion()
	if err != nil {
		slog.Error("Error while validating database schema version", "error", err)
		os.Exit(1)
	}

	slog.Info("Database Schema successfully Updated.", "version", version)
}
