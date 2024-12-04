// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package node

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cartesi/rollups-node/pkg/service"

	"github.com/cartesi/rollups-node/internal/advancer"
	"github.com/cartesi/rollups-node/internal/claimer"
	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/evmreader"
	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/validator"

	"github.com/ethereum/go-ethereum/ethclient"
)

type CreateInfo struct {
	service.CreateInfo

	BlockchainHttpEndpoint config.Redacted[string]
	BlockchainID           uint64
	HTTPEndpoint           config.Redacted[string]
	PostgresEndpoint       config.Redacted[string]
	EnableClaimSubmission  bool
	MaxStartupTime         time.Duration
}

type Service struct {
	service.Service

	Children   []service.IService
	Client     *ethclient.Client
	Repository *repository.Database
}

func (c *CreateInfo) LoadEnv() {
	c.BlockchainHttpEndpoint = config.Redacted[string]{config.GetBlockchainHttpEndpoint()}
	c.BlockchainID = config.GetBlockchainId()
	c.EnableClaimSubmission = config.GetFeatureClaimSubmissionEnabled()
	c.PostgresEndpoint = config.Redacted[string]{config.GetPostgresEndpoint()}
	c.MaxStartupTime = config.GetMaxStartupTime()
	c.LogLevel = service.LogLevel(config.GetLogLevel())
	c.LogPretty = config.GetLogPrettyEnabled()

	httpAddress := config.GetHttpAddress()
	httpPort := config.GetHttpPort()
	c.HTTPEndpoint.Value = fmt.Sprintf("%v:%v", httpAddress, httpPort)
}

func Create(c *CreateInfo, s *Service) error {
	var err error

	err = service.Create(&c.CreateInfo, &s.Service)
	if err != nil {
		return err
	}

	err = service.WithTimeout(c.MaxStartupTime, func() error {
		// database connection
		s.Repository, err = repository.Connect(s.Context, c.PostgresEndpoint.Value, s.Logger)
		if err != nil {
			return err
		}

		// blockchain connection + chainID check
		s.Client, err = ethclient.Dial(c.BlockchainHttpEndpoint.Value)
		if err != nil {
			return err
		}
		chainID, err := s.Client.ChainID(s.Context)
		if err != nil {
			return err
		}
		if c.BlockchainID != chainID.Uint64() {
			return fmt.Errorf(
				"chainId mismatch; got: %v, expected: %v",
				chainID,
				c.BlockchainID,
			)
		}
		return nil
	})

	if err != nil {
		s.Logger.Error(fmt.Sprint(err))
		return err
	}
	return createServices(c, s)
}

func createServices(c *CreateInfo, s *Service) error {
	ch := make(chan service.IService)
	deadline := time.After(c.MaxStartupTime)
	numChildren := 0

	numChildren++
	go func() {
		ch <- newEVMReader(c, s.Logger, s.Repository)
	}()

	numChildren++
	go func() {
		ch <- newAdvancer(c, s.Logger, s.Repository, s.ServeMux)
	}()

	numChildren++
	go func() {
		ch <- newValidator(c, s.Logger, s.Repository)
	}()

	numChildren++
	go func() {
		ch <- newClaimer(c, s.Logger, s.Repository)
	}()

	for range numChildren {
		select {
		case child := <-ch:
			s.Children = append(s.Children, child)
		case <-deadline:
			s.Logger.Error("Failed to create services. Time limit exceded",
				"limit", c.MaxStartupTime)
			return fmt.Errorf("Failed to create services. Time limit exceded")
		}
	}
	return nil
}

func (me *Service) Alive() bool {
	allAlive := true
	for _, s := range me.Children {
		allAlive = allAlive && s.Alive()
	}
	return allAlive
}

func (me *Service) Ready() bool {
	allReady := true
	for _, s := range me.Children {
		allReady = allReady && s.Ready()
	}
	return allReady
}

func (s *Service) Reload() []error { return nil }
func (s *Service) Tick() []error   { return nil }
func (me *Service) Stop(force bool) []error {
	errs := []error{}
	for _, s := range me.Children {
		errs = append(errs, s.Stop(force)...)
	}
	return errs
}

func (me *Service) Serve() error {
	for _, s := range me.Children {
		go s.Serve()
	}
	return me.Service.Serve()
}

// services creation

func newEVMReader(
	nc *CreateInfo,
	logger *slog.Logger,
	database *repository.Database,
) service.IService {
	s := evmreader.Service{}
	c := evmreader.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:      "evm-reader",
			Impl:      &s,
			ProcOwner: true, // TODO: Remove this after updating supervisor
		},
		EvmReaderPersistentConfig: model.EvmReaderPersistentConfig{
			DefaultBlock: model.DefaultBlockStatusSafe,
		},
		Database: database,
	}
	c.LoadEnv()
	c.LogLevel = nc.LogLevel
	c.LogPretty = nc.LogPretty

	err := evmreader.Create(&c, &s)
	if err != nil {
		logger.Error("Fatal",
			"error", err)
		panic(err)
	}
	return &s
}

func newAdvancer(
	nc *CreateInfo,
	logger *slog.Logger,
	database *repository.Database,
	serveMux *http.ServeMux,
) service.IService {
	s := advancer.Service{}
	c := advancer.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:      "advancer",
			Impl:      &s,
			ProcOwner: true,
			ServeMux:  serveMux,
		},
		Repository: database,
	}
	c.LoadEnv()
	c.LogLevel = nc.LogLevel
	c.LogPretty = nc.LogPretty

	err := advancer.Create(&c, &s)
	if err != nil {
		logger.Error("Fatal",
			"error", err)
		panic(err)
	}
	return &s
}

func newValidator(
	nc *CreateInfo,
	logger *slog.Logger,
	database *repository.Database,
) service.IService {
	s := validator.Service{}
	c := validator.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:      "validator",
			Impl:      &s,
			ProcOwner: true,
		},
		Repository: database,
	}
	c.LoadEnv()
	c.LogLevel = nc.LogLevel
	c.LogPretty = nc.LogPretty

	err := validator.Create(&c, &s)
	if err != nil {
		logger.Error("Fatal",
			"error", err)
		panic(err)
	}
	return &s
}

func newClaimer(
	nc *CreateInfo,
	logger *slog.Logger,
	database *repository.Database,
) service.IService {
	s := claimer.Service{}
	c := claimer.CreateInfo{
		CreateInfo: service.CreateInfo{
			Name:      "claimer",
			Impl:      &s,
			ProcOwner: true,
		},
		Repository: database,
	}
	c.LoadEnv()
	c.LogLevel = nc.LogLevel
	c.LogPretty = nc.LogPretty
	c.EnableSubmission = nc.EnableClaimSubmission

	err := claimer.Create(&c, &s)
	if err != nil {
		logger.Error("Fatal",
			"error", err)
		panic(err)
	}
	return &s
}
