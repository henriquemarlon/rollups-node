// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	appcontract "github.com/cartesi/rollups-node/pkg/contracts/iapplication"
	"github.com/cartesi/rollups-node/pkg/contracts/iinputbox"
	"github.com/cartesi/rollups-node/pkg/service"
)

type CreateInfo struct {
	service.CreateInfo

	Config config.Config

	Repository repository.Repository

	EthClient   *ethclient.Client
	EthWsClient *ethclient.Client
}

type Service struct {
	service.Service

	client             EthClient
	wsClient           EthWsClient
	repository         EvmReaderRepository
	contractFactory    ContractFactory
	chainId            uint64
	defaultBlock       DefaultBlock
	hasEnabledApps     bool
	inputReaderEnabled bool
}

const EvmReaderConfigKey = "evm-reader"

type PersistentConfig struct {
	DefaultBlock       DefaultBlock
	InputReaderEnabled bool
	ChainID            uint64
}

func Create(ctx context.Context, c *CreateInfo) (*Service, error) {
	var err error
	if err = ctx.Err(); err != nil {
		return nil, err // This returns context.Canceled or context.DeadlineExceeded.
	}

	s := &Service{}
	c.CreateInfo.Impl = s

	err = service.Create(ctx, &c.CreateInfo, &s.Service)
	if err != nil {
		return nil, err
	}

	if c.EthClient == nil {
		return nil, fmt.Errorf("EthClient on evmreader service Create is nil")
	}
	chainId, err := c.EthClient.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	if chainId.Uint64() != c.Config.BlockchainId {
		return nil, fmt.Errorf("EthClient chainId mismatch: network %d != provided %d",
			chainId.Uint64(), c.Config.BlockchainId)
	}

	if c.EthWsClient == nil {
		return nil, fmt.Errorf("EthWsClient on evmreader service Create is nil")
	}
	chainId, err = c.EthWsClient.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	if chainId.Uint64() != c.Config.BlockchainId {
		return nil, fmt.Errorf("EthWsClient chainId mismatch: network %d != provided %d",
			chainId.Uint64(), c.Config.BlockchainId)
	}

	s.repository = c.Repository
	if s.repository == nil {
		return nil, fmt.Errorf("repository on evmreader service Create is nil")
	}

	nodeConfig, err := s.setupPersistentConfig(ctx, &c.Config)
	if err != nil {
		return nil, err
	}
	if chainId.Uint64() != nodeConfig.ChainID {
		return nil, fmt.Errorf("NodeConfig chainId mismatch: network %d != config %d",
			chainId.Uint64(), nodeConfig.ChainID)
	}

	maxRetries := c.Config.EvmReaderRetryPolicyMaxRetries
	maxDelay := c.Config.EvmReaderRetryPolicyMaxDelay
	s.client = NewEhtClientWithRetryPolicy(c.EthClient, maxRetries, maxDelay, s.Logger)
	s.wsClient = NewEthWsClientWithRetryPolicy(c.EthWsClient, maxRetries, maxDelay, s.Logger)
	s.contractFactory = NewEvmReaderContractFactory(c.EthClient, maxRetries, maxDelay)

	s.chainId = nodeConfig.ChainID
	s.defaultBlock = nodeConfig.DefaultBlock
	s.inputReaderEnabled = nodeConfig.InputReaderEnabled
	s.hasEnabledApps = true

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
	return []error{}
}

func (s *Service) Serve() error {
	ready := make(chan struct{}, 1)
	go s.Run(s.Context, ready)
	return s.Service.Serve()
}

func (s *Service) String() string {
	return s.Name
}

func (s *Service) setupPersistentConfig(
	ctx context.Context,
	c *config.Config,
) (*PersistentConfig, error) {
	config, err := repository.LoadNodeConfig[PersistentConfig](ctx, s.repository, EvmReaderConfigKey)
	if config == nil && err == nil {
		nc := model.NodeConfig[PersistentConfig]{
			Key: EvmReaderConfigKey,
			Value: PersistentConfig{
				DefaultBlock:       c.BlockchainDefaultBlock,
				InputReaderEnabled: c.FeatureInputReaderEnabled,
				ChainID:            c.BlockchainId,
			},
		}
		s.Logger.Info("Initializing evm-reader persistent config", "config", nc.Value)
		err = repository.SaveNodeConfig(ctx, s.repository, &nc)
		if err != nil {
			return nil, err
		}
		return &nc.Value, nil
	} else if err == nil {
		s.Logger.Info("Evm-reader was already configured. Using previous persistent config", "config", config.Value)
		return &config.Value, nil
	}

	s.Logger.Error("Could not retrieve persistent config from Database. %w", "error", err)
	return nil, err
}

// Interface for Input reading
type InputSource interface {
	// Wrapper for FilterInputAdded(), which is automatically generated
	// by go-ethereum and cannot be used for testing
	RetrieveInputs(opts *bind.FilterOpts, appAddresses []common.Address, index []*big.Int,
	) ([]iinputbox.IInputBoxInputAdded, error)
}

// Interface for the node repository
type EvmReaderRepository interface {
	ListApplications(ctx context.Context, f repository.ApplicationFilter, p repository.Pagination) ([]*Application, uint64, error)
	UpdateApplicationState(ctx context.Context, appID int64, state ApplicationState, reason *string) error
	UpdateEventLastCheckBlock(ctx context.Context, appIDs []int64, event MonitoredEvent, blockNumber uint64) error

	SaveNodeConfigRaw(ctx context.Context, key string, rawJSON []byte) error
	LoadNodeConfigRaw(ctx context.Context, key string) (rawJSON []byte, createdAt, updatedAt time.Time, err error)

	// Input monitor
	CreateEpochsAndInputs(
		ctx context.Context, nameOrAddress string,
		epochInputMap map[*Epoch][]*Input, blockNumber uint64,
	) error
	GetEpoch(ctx context.Context, nameOrAddress string, index uint64) (*Epoch, error)
	ListEpochs(ctx context.Context, nameOrAddress string, f repository.EpochFilter, p repository.Pagination) ([]*Epoch, uint64, error)

	// Output execution monitor
	GetOutput(ctx context.Context, nameOrAddress string, indexKey uint64) (*Output, error)
	UpdateOutputsExecution(ctx context.Context, nameOrAddress string, executedOutputs []*Output, blockNumber uint64) error
}

// EthClient mimics part of ethclient.Client functions to narrow down the
// interface needed by the EvmReader. It must be bound to an HTTP endpoint
type EthClient interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
}

// EthWsClient mimics part of ethclient.Client functions to narrow down the
// interface needed by the EvmReader. It must be bound to a WS endpoint
type EthWsClient interface {
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
}

type ApplicationContract interface {
	RetrieveOutputExecutionEvents(
		opts *bind.FilterOpts,
	) ([]*appcontract.IApplicationOutputExecuted, error)
}

type ContractFactory interface {
	NewApplication(address common.Address) (ApplicationContract, error)
	NewInputSource(address common.Address) (InputSource, error)
}

type SubscriptionError struct {
	Cause error
}

func (e *SubscriptionError) Error() string {
	return fmt.Sprintf("Subscription error : %v", e.Cause)
}

// Internal struct to hold application and it's contracts together
type appContracts struct {
	application         *Application
	applicationContract ApplicationContract
	inputSource         InputSource
}

func (r *Service) Run(ctx context.Context, ready chan struct{}) error {
	// TODO: check if chainId matches
	for {
		err := r.watchForNewBlocks(ctx, ready)
		// If the error is a SubscriptionError, re run watchForNewBlocks
		// that it will restart the websocket subscription
		if _, ok := err.(*SubscriptionError); !ok {
			return err
		}
		r.Logger.Error(err.Error())
		r.Logger.Info("Restarting subscription")
	}
}

func getAllRunningApplications(ctx context.Context, er EvmReaderRepository) ([]*Application, uint64, error) {
	f := repository.ApplicationFilter{
		State:            Pointer(ApplicationState_Enabled),
	}
	return er.ListApplications(ctx, f, repository.Pagination{})
}

// watchForNewBlocks watches for new blocks and reads new inputs based on the
// default block configuration, which have not been processed yet.
func (r *Service) watchForNewBlocks(ctx context.Context, ready chan<- struct{}) error {
	headers := make(chan *types.Header)
	sub, err := r.wsClient.SubscribeNewHead(ctx, headers)
	if err != nil {
		return fmt.Errorf("could not start subscription: %v", err)
	}
	r.Logger.Info("Subscribed to new block events")
	ready <- struct{}{}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-sub.Err():
			return &SubscriptionError{Cause: err}
		case header := <-headers:

			// Every time a new block arrives
			r.Logger.Debug("New block header received", "blockNumber", header.Number, "blockHash", header.Hash())

			r.Logger.Debug("Retrieving enabled applications")
			runningApps, _, err := getAllRunningApplications(ctx, r.repository)
			if err != nil {
				r.Logger.Error("Error retrieving running applications",
					"error",
					err,
				)
				continue
			}

			if len(runningApps) == 0 {
				if r.hasEnabledApps {
					r.Logger.Info("No registered applications enabled")
				}
				r.hasEnabledApps = false
				continue
			}
			if !r.hasEnabledApps {
				r.Logger.Info("Found enabled applications")
			}
			r.hasEnabledApps = true

			// Build Contracts
			var apps []appContracts
			for _, app := range runningApps {
				applicationContract, inputSource, err := r.getAppContracts(app)
				if err != nil {
					r.Logger.Error("Error retrieving application contracts", "app", app, "error", err)
					continue
				}
				aContracts := appContracts{
					application:         app,
					applicationContract: applicationContract,
					inputSource:         inputSource,
				}

				apps = append(apps, aContracts)
			}

			if len(apps) == 0 {
				r.Logger.Info("No correctly configured applications running")
				continue
			}

			blockNumber := header.Number.Uint64()
			if r.defaultBlock != DefaultBlock_Latest {
				mostRecentHeader, err := r.fetchMostRecentHeader(
					ctx,
					r.defaultBlock,
				)
				if err != nil {
					r.Logger.Error("Error fetching most recent block",
						"default block", r.defaultBlock,
						"error", err)
					continue
				}
				blockNumber = mostRecentHeader.Number.Uint64()

				r.Logger.Debug(fmt.Sprintf("Using block %d and not %d because of commitment policy: %s",
					mostRecentHeader.Number.Uint64(), header.Number.Uint64(), r.defaultBlock))
			}

			r.checkForNewInputs(ctx, apps, blockNumber)

			r.checkForOutputExecution(ctx, apps, blockNumber)

		}
	}
}

// fetchMostRecentHeader fetches the most recent header up till the
// given default block
func (r *Service) fetchMostRecentHeader(
	ctx context.Context,
	defaultBlock DefaultBlock,
) (*types.Header, error) {

	var defaultBlockNumber int64
	switch defaultBlock {
	case DefaultBlock_Pending:
		defaultBlockNumber = rpc.PendingBlockNumber.Int64()
	case DefaultBlock_Latest:
		defaultBlockNumber = rpc.LatestBlockNumber.Int64()
	case DefaultBlock_Finalized:
		defaultBlockNumber = rpc.FinalizedBlockNumber.Int64()
	case DefaultBlock_Safe:
		defaultBlockNumber = rpc.SafeBlockNumber.Int64()
	default:
		return nil, fmt.Errorf("default block '%v' not supported", defaultBlock)
	}

	header, err :=
		r.client.HeaderByNumber(
			ctx,
			new(big.Int).SetInt64(defaultBlockNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve header. %v", err)
	}

	if header == nil {
		return nil, fmt.Errorf("returned header is nil")
	}
	return header, nil
}

// getAppContracts retrieves the ApplicationContract and ConsensusContract for a given Application.
// Also validates if IConsensus configuration matches the blockchain registered one
func (r *Service) getAppContracts(app *Application,
) (ApplicationContract, InputSource, error) {
	if app == nil {
		return nil, nil, fmt.Errorf("Application reference is nil. Should never happen")
	}

	applicationContract, err := r.contractFactory.NewApplication(app.IApplicationAddress)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("error building application contract"),
			err,
		)

	}

	inputSource, err := r.contractFactory.NewInputSource(app.IInputBoxAddress)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("error building inputbox contract"),
			err,
		)

	}

	return applicationContract, inputSource, nil
}
