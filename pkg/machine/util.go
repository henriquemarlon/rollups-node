// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package machine

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cartesi/rollups-node/pkg/emulator"
)

// StartServer starts the JSON RPC remote cartesi machine server hosted at address.
func StartServer(logger *slog.Logger, deadline time.Duration) (Backend, string, uint32, error) {
	if logger == nil {
		return nil, "", 0, errors.New("logger must not be nil")
	}

	logger.Info("Starting server")
	server, address, pid, err := emulator.SpawnServer("127.0.0.1:0", deadline)
	if err != nil {
		return nil, "", 0, fmt.Errorf("spawn server failed: %w", err)
	}

	return &LibCartesiBackend{inner: server}, address, pid, nil
}

// StopServer shuts down the JSON RPC remote cartesi machine server hosted at address.
func StopServer(address string, logger *slog.Logger, deadline time.Duration) error {
	if logger == nil {
		return errors.New("logger must not be nil")
	}

	logger.Info("Stopping server at", "address", address)
	remote, err := emulator.ConnectServer(address, deadline)
	if err != nil {
		return err
	}
	return remote.ShutdownServer()
}

// ValidatePath provides path validation functionality for security
func ValidatePath(path, pathType string) error {
	if path == "" {
		return fmt.Errorf("%s path cannot be empty", pathType)
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("%s path contains invalid characters", pathType)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve %s path: %w", pathType, err)
	}

	// For template and snapshot paths, check if they exist
	if pathType == "template" || pathType == "snapshot" {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return fmt.Errorf("%s path does not exist: %s", pathType, absPath)
		}
	}

	// For store paths, check if parent directory exists
	if pathType == "store" {
		parentDir := filepath.Dir(absPath)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return fmt.Errorf("parent directory for %s path does not exist: %s", pathType, parentDir)
		}
	}

	return nil
}

// ValidateTemplatePath validates a template path
func ValidateTemplatePath(path string) error {
	return ValidatePath(path, "template")
}

// ValidateSnapshotPath validates a snapshot path
func ValidateSnapshotPath(path string) error {
	return ValidatePath(path, "snapshot")
}

// ValidateStorePath validates a store path
func ValidateStorePath(path string) error {
	return ValidatePath(path, "store")
}
