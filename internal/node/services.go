// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package node

import (
	"fmt"
	"net/http"

	advancerservice "github.com/cartesi/rollups-node/internal/advancer"
	claimerservice "github.com/cartesi/rollups-node/internal/claimer"
	"github.com/cartesi/rollups-node/internal/config"
	readerservice "github.com/cartesi/rollups-node/internal/evmreader"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/services"
	validatorservice "github.com/cartesi/rollups-node/internal/validator"
	"github.com/cartesi/rollups-node/pkg/service"
)

// We use an enum to define the ports of each service and avoid conflicts.
type portOffset = int

const (
	portOffsetProxy = iota
)

// Get the port of the given service.
func getPort(c config.NodeConfig, offset portOffset) int {
	return c.HttpPort + int(offset)
}

func newSupervisorService(
	c config.NodeConfig,
	database *repository.Database,
) *services.SupervisorService {
	var s []services.Service

	serveMux := http.NewServeMux()
	serveMux.Handle("/healthz", http.HandlerFunc(healthcheckHandler))

	s = append(s, newClaimerService(c, database))
	s = append(s, newEvmReaderService(c, database))
	s = append(s, newAdvancerService(c, database, serveMux))
	s = append(s, newValidatorService(c, database))
	s = append(s, newHttpService(c, serveMux))

	supervisor := services.SupervisorService{
		Name:     "rollups-node",
		Services: s,
	}
	return &supervisor
}

func newHttpService(c config.NodeConfig, serveMux *http.ServeMux) services.Service {
	addr := fmt.Sprintf("%v:%v", c.HttpAddress, getPort(c, portOffsetProxy))
	return &services.HttpService{
		Name:    "http",
		Address: addr,
		Handler: serveMux,
	}
}

func newEvmReaderService(c config.NodeConfig, database *repository.Database) services.Service {
	readerService := readerservice.Service{}
	createInfo := readerservice.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:      "reader",
			Impl:      &readerService,
			ProcOwner: true, // TODO: Remove this after updating supervisor
			LogLevel:  service.LogLevel(c.LogLevel),
		},
		Database: database,
	}

	err := readerservice.Create(&createInfo, &readerService)
	if err != nil {
		readerService.Logger.Error("Fatal",
			"service", readerService.Name,
			"error", err)
	}
	return &readerService
}

func newAdvancerService(c config.NodeConfig, database *repository.Database, serveMux *http.ServeMux) services.Service {
	advancerService := advancerservice.Service{}
	createInfo := advancerservice.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:         "advancer",
			PollInterval: c.AdvancerPollingInterval,
			Impl:         &advancerService,
			ProcOwner:    true, // TODO: Remove this after updating supervisor
			LogLevel:     service.LogLevel(c.LogLevel),
			ServeMux: serveMux,
		},
		Repository: database,
	}

	err := advancerservice.Create(&createInfo, &advancerService)
	if err != nil {
		advancerService.Logger.Error("Fatal",
			"service", advancerService.Name,
			"error", err)
	}
	return &advancerService
}

func newValidatorService(c config.NodeConfig, database *repository.Database) services.Service {
	validatorService := validatorservice.Service{}
	createInfo := validatorservice.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:         "validator",
			PollInterval: c.ValidatorPollingInterval,
			Impl:         &validatorService,
			ProcOwner:    true, // TODO: Remove this after updating supervisor
			LogLevel:     service.LogLevel(c.LogLevel),
		},
		Repository: database,
	}

	err := validatorservice.Create(createInfo, &validatorService)
	if err != nil {
		validatorService.Logger.Error("Fatal",
			"service", validatorService.Name,
			"error", err)
	}
	return &validatorService
}

func newClaimerService(c config.NodeConfig, database *repository.Database) services.Service {
	claimerService := claimerservice.Service{}
	createInfo := claimerservice.CreateInfo{
		Auth:                   c.Auth,
		DBConn:                 database,
		PostgresEndpoint:       c.PostgresEndpoint,
		BlockchainHttpEndpoint: c.BlockchainHttpEndpoint,
		EnableSubmission:       c.FeatureClaimSubmissionEnabled,
		CreateInfo: service.CreateInfo{
			Name:         "claimer",
			PollInterval: c.ClaimerPollingInterval,
			Impl:         &claimerService,
			ProcOwner:    true, // TODO: Remove this after updating supervisor
			LogLevel:     service.LogLevel(c.LogLevel),
		},
	}

	err := claimerservice.Create(createInfo, &claimerService)
	if err != nil {
		claimerService.Logger.Error("Fatal",
			"service", claimerService.Name,
			"error", err)
	}
	return &claimerService
}
