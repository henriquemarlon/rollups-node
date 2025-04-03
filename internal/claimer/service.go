// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package claimer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/config/auth"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/service"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type CreateInfo struct {
	service.CreateInfo

	Config config.Config

	EthConn    *ethclient.Client
	Repository repository.Repository
}

type Service struct {
	service.Service

	repository        iclaimerRepository
	blockchain        iclaimerBlockchain
	claimsInFlight    map[int64]common.Hash // application.ID -> txHash
	submissionEnabled bool
}

const ClaimerConfigKey = "claimer"

type PersistentConfig struct {
	DefaultBlock           DefaultBlock
	ClaimSubmissionEnabled bool
	ChainID                uint64
}

func Create(ctx context.Context, c *CreateInfo) (*Service, error) {
	var err error
	if err = ctx.Err(); err != nil {
		return nil, err // This returns context.Canceled or context.DeadlineExceeded.
	}
	if c.Repository == nil {
		return nil, fmt.Errorf("repository on claimer service Create is nil")
	}
	if c.EthConn == nil {
		return nil, fmt.Errorf("ethclient on claimer service Create is nil")
	}

	s := &Service{}
	c.CreateInfo.Impl = s

	err = service.Create(ctx, &c.CreateInfo, &s.Service)
	if err != nil {
		return nil, err
	}

	nodeConfig, err := setupPersistentConfig(ctx, s.Logger, c.Repository, &c.Config)
	if err != nil {
		return nil, err
	}

	chainId, err := c.EthConn.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	if chainId.Uint64() != c.Config.BlockchainId {
		return nil, fmt.Errorf("chainId mismatch: network %d != provided %d", chainId.Uint64(), c.Config.BlockchainId)
	}

	if chainId.Uint64() != nodeConfig.ChainID {
		return nil, fmt.Errorf("NodeConfig chainId mismatch: network %d != config %d",
			chainId.Uint64(), nodeConfig.ChainID)
	}
	s.submissionEnabled = nodeConfig.ClaimSubmissionEnabled
	s.claimsInFlight = map[int64]common.Hash{}

	var txOpts *bind.TransactOpts = nil
	if s.submissionEnabled {
		txOpts, err = auth.GetTransactOpts(chainId)
		if err != nil {
			return nil, err
		}
	}

	s.repository = c.Repository

	s.blockchain = &claimerBlockchain{
		logger:       s.Logger,
		client:       c.EthConn,
		txOpts:       txOpts,
		defaultBlock: c.Config.BlockchainDefaultBlock,
	}

	return s, nil
}

func (s *Service) Alive() bool {
	return true
}

func (s *Service) Ready() bool {
	return true
}

func (s *Service) Reload() []error {
	return nil
}

func (s *Service) Stop(bool) []error {
	return nil
}

func (s *Service) Tick() []error {
	errs := []error{}
	endBlock, err := s.blockchain.getBlockNumber(s.Context)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	errs = append(errs, s.submitClaimsAndUpdateDatabase(endBlock)...)
	errs = append(errs, s.acceptClaimsAndUpdateDatabase(endBlock)...)

	return errs
}

func setupPersistentConfig(
	ctx context.Context,
	logger *slog.Logger,
	repo iclaimerRepository,
	c *config.Config,
) (*PersistentConfig, error) {
	config, err := repository.LoadNodeConfig[PersistentConfig](ctx, repo, ClaimerConfigKey)
	if config == nil && err == nil {
		nc := NodeConfig[PersistentConfig]{
			Key: ClaimerConfigKey,
			Value: PersistentConfig{
				DefaultBlock:           c.BlockchainDefaultBlock,
				ClaimSubmissionEnabled: c.FeatureClaimSubmissionEnabled,
				ChainID:                c.BlockchainId,
			},
		}
		logger.Info("Initializing claimer persistent config", "config", nc.Value)
		err = repository.SaveNodeConfig(ctx, repo, &nc)
		if err != nil {
			return nil, err
		}
		return &nc.Value, nil
	} else if err == nil {
		logger.Info("Claimer was already configured. Using previous persistent config", "config", config.Value)
		return &config.Value, nil
	}

	logger.Error("Could not retrieve persistent config from Database. %w", "error", err)
	return nil, err
}
