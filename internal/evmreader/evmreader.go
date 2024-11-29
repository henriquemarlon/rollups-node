// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package evmreader

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/cartesi/rollups-node/internal/config"
	"github.com/cartesi/rollups-node/internal/model"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	appcontract "github.com/cartesi/rollups-node/pkg/contracts/iapplication"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"
	"github.com/cartesi/rollups-node/pkg/contracts/iinputbox"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5"
)

type CreateInfo struct {
	service.CreateInfo

	model.EvmReaderPersistentConfig

	DefaultBlockString     string
	PostgresEndpoint       config.Redacted[string]
	BlockchainHttpEndpoint config.Redacted[string]
	BlockchainWsEndpoint   config.Redacted[string]
	Database               *repository.Database
	MaxRetries             uint64
	MaxDelay               time.Duration
}

type Service struct {
	service.Service
	reader EvmReader
}

func (c *CreateInfo) LoadEnv() {
	c.BlockchainHttpEndpoint.Value = config.GetBlockchainHttpEndpoint()
	c.BlockchainWsEndpoint.Value = config.GetBlockchainWsEndpoint()
	c.MaxDelay = config.GetEvmReaderRetryPolicyMaxDelay()
	c.MaxRetries = config.GetEvmReaderRetryPolicyMaxRetries()
	c.PostgresEndpoint.Value = config.GetPostgresEndpoint()

	// persistent
	c.DefaultBlock = config.GetEvmReaderDefaultBlock()
	c.InputBoxDeploymentBlock = uint64(config.GetContractsInputBoxDeploymentBlockNumber())
	c.InputBoxAddress = common.HexToAddress(config.GetContractsInputBoxAddress())
	c.ChainId = config.GetBlockchainId()
}

func Create(c *CreateInfo, s *Service) error {
	var err error

	err = service.Create(&c.CreateInfo, &s.Service)
	if err != nil {
		return err
	}

	c.DefaultBlock, err = config.ToDefaultBlockFromString(c.DefaultBlockString)
	if err != nil {
		return err
	}

	client, err := ethclient.DialContext(s.Context, c.BlockchainHttpEndpoint.Value)
	if err != nil {
		return err
	}

	wsClient, err := ethclient.DialContext(s.Context, c.BlockchainWsEndpoint.Value)
	if err != nil {
		return err
	}

	if c.Database == nil {
		c.Database, err = repository.Connect(s.Context, c.PostgresEndpoint.Value)
		if err != nil {
			return err
		}
	}

	err = s.SetupPersistentConfig(s.Context, c.Database, &c.EvmReaderPersistentConfig)
	if err != nil {
		return err
	}

	inputSource, err := NewInputSourceAdapter(c.InputBoxAddress, client)
	if err != nil {
		return err
	}

	contractFactory := NewEvmReaderContractFactory(client, c.MaxRetries, c.MaxDelay)

	s.reader = NewEvmReader(
		NewEhtClientWithRetryPolicy(client, c.MaxRetries, c.MaxDelay),
		NewEthWsClientWithRetryPolicy(wsClient, c.MaxRetries, c.MaxDelay),
		NewInputSourceWithRetryPolicy(inputSource, c.MaxRetries, c.MaxDelay),
		c.Database,
		c.InputBoxDeploymentBlock,
		c.DefaultBlock,
		contractFactory,
	)
	return nil
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

func (s *Service) Start(context context.Context, ready chan<- struct{}) error {
	go s.reader.Run(s.Context, ready)
	return s.Serve()
}
func (s *Service) String() string {
	return s.Name
}

func (me *Service) SetupPersistentConfig(
	ctx      context.Context,
	database *repository.Database,
	c        *model.EvmReaderPersistentConfig,
) error {
	err := database.SelectEvmReaderConfig(ctx, c)
	if err == pgx.ErrNoRows {
		_, err = database.InsertEvmReaderConfig(ctx, c)
		if err != nil {
			return err
		}
	} else if err == nil {
		me.Logger.Info("Node was already configured. Using previous persistent config", "config", c)
	} else {
		me.Logger.Error("Could not retrieve persistent config from Database. %w", "error", err)
	}
	return err
}

// Interface for Input reading
type InputSource interface {
	// Wrapper for FilterInputAdded(), which is automatically generated
	// by go-ethereum and cannot be used for testing
	RetrieveInputs(opts *bind.FilterOpts, appAddresses []Address, index []*big.Int,
	) ([]iinputbox.IInputBoxInputAdded, error)
}

// Interface for the node repository
type EvmReaderRepository interface {
	StoreEpochAndInputsTransaction(
		ctx context.Context, epochInputMap map[*Epoch][]Input, blockNumber uint64,
		appAddress Address,
	) (epochIndexIdMap map[uint64]uint64, epochIndexInputIdsMap map[uint64][]uint64, err error)

	GetAllRunningApplications(ctx context.Context) ([]Application, error)
	SelectEvmReaderConfig(context.Context, *model.EvmReaderPersistentConfig) error
	GetEpoch(ctx context.Context, indexKey uint64, appAddressKey Address) (*Epoch, error)
	GetPreviousEpochsWithOpenClaims(
		ctx context.Context,
		app Address,
		lastBlock uint64,
	) ([]*Epoch, error)
	UpdateEpochs(ctx context.Context,
		app Address,
		claims []*Epoch,
		mostRecentBlockNumber uint64,
	) error
	GetOutput(
		ctx context.Context, appAddressKey Address, indexKey uint64,
	) (*Output, error)
	UpdateOutputExecutionTransaction(
		ctx context.Context, app Address, executedOutputs []*Output, blockNumber uint64,
	) error
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

type ConsensusContract interface {
	GetEpochLength(opts *bind.CallOpts) (*big.Int, error)
	RetrieveClaimAcceptanceEvents(
		opts *bind.FilterOpts,
		appAddresses []Address,
	) ([]*iconsensus.IConsensusClaimAcceptance, error)
}

type ApplicationContract interface {
	GetConsensus(opts *bind.CallOpts) (Address, error)
	RetrieveOutputExecutionEvents(
		opts *bind.FilterOpts,
	) ([]*appcontract.IApplicationOutputExecuted, error)
}

type ContractFactory interface {
	NewApplication(address Address) (ApplicationContract, error)
	NewIConsensus(address Address) (ConsensusContract, error)
}

type SubscriptionError struct {
	Cause error
}

func (e *SubscriptionError) Error() string {
	return fmt.Sprintf("Subscription error : %v", e.Cause)
}

// Internal struct to hold application and it's contracts together
type application struct {
	Application
	applicationContract ApplicationContract
	consensusContract   ConsensusContract
}

// EvmReader reads Input Added, Claim Submitted and
// Output Executed events from the blockchain
type EvmReader struct {
	client                  EthClient
	wsClient                EthWsClient
	inputSource             InputSource
	repository              EvmReaderRepository
	contractFactory         ContractFactory
	inputBoxDeploymentBlock uint64
	defaultBlock            DefaultBlock
	epochLengthCache        map[Address]uint64
	hasEnabledApps          bool
}

func (r *EvmReader) String() string {
	return "evmreader"
}

// Creates a new EvmReader
func NewEvmReader(
	client EthClient,
	wsClient EthWsClient,
	inputSource InputSource,
	repository EvmReaderRepository,
	inputBoxDeploymentBlock uint64,
	defaultBlock DefaultBlock,
	contractFactory ContractFactory,
) EvmReader {
	return EvmReader{
		client:                  client,
		wsClient:                wsClient,
		inputSource:             inputSource,
		repository:              repository,
		inputBoxDeploymentBlock: inputBoxDeploymentBlock,
		defaultBlock:            defaultBlock,
		contractFactory:         contractFactory,
		hasEnabledApps:          true,
	}
}

func (r *EvmReader) Run(ctx context.Context, ready chan<- struct{}) error {

	// Initialize epochLength cache
	r.epochLengthCache = make(map[Address]uint64)

	for {
		err := r.watchForNewBlocks(ctx, ready)
		// If the error is a SubscriptionError, re run watchForNewBlocks
		// that it will restart the websocket subscription
		if _, ok := err.(*SubscriptionError); !ok {
			return err
		}
		slog.Error(err.Error())
		slog.Info("evmreader: Restarting subscription")
	}
}

// watchForNewBlocks watches for new blocks and reads new inputs based on the
// default block configuration, which have not been processed yet.
func (r *EvmReader) watchForNewBlocks(ctx context.Context, ready chan<- struct{}) error {
	headers := make(chan *types.Header)
	sub, err := r.wsClient.SubscribeNewHead(ctx, headers)
	if err != nil {
		return fmt.Errorf("could not start subscription: %v", err)
	}
	slog.Info("evmreader: Subscribed to new block events")
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
			slog.Debug("evmreader: New block header received", "blockNumber", header.Number, "blockHash", header.Hash())

			slog.Debug("evmreader: Retrieving enabled applications")
			// Get All Applications
			runningApps, err := r.repository.GetAllRunningApplications(ctx)
			if err != nil {
				slog.Error("evmreader: Error retrieving running applications",
					"error",
					err,
				)
				continue
			}

			if len(runningApps) == 0 {
				if r.hasEnabledApps {
					slog.Info("evmreader: No registered applications enabled")
				}
				r.hasEnabledApps = false
				continue
			}
			if !r.hasEnabledApps {
				slog.Info("evmreader: Found enabled applications")
			}
			r.hasEnabledApps = true

			// Build Contracts
			var apps []application
			for _, app := range runningApps {
				applicationContract, consensusContract, err := r.getAppContracts(app)
				if err != nil {
					slog.Error("evmreader: Error retrieving application contracts", "app", app, "error", err)
					continue
				}
				apps = append(apps, application{Application: app,
					applicationContract: applicationContract,
					consensusContract:   consensusContract})
			}

			if len(apps) == 0 {
				slog.Info("evmreader: No correctly configured applications running")
				continue
			}

			blockNumber := header.Number.Uint64()
			if r.defaultBlock != DefaultBlockStatusLatest {
				mostRecentHeader, err := r.fetchMostRecentHeader(
					ctx,
					r.defaultBlock,
				)
				if err != nil {
					slog.Error("evmreader: Error fetching most recent block",
						"default block", r.defaultBlock,
						"error", err)
					continue
				}
				blockNumber = mostRecentHeader.Number.Uint64()

				slog.Debug(fmt.Sprintf("evmreader: Using block %d and not %d because of commitment policy: %s",
					mostRecentHeader.Number.Uint64(), header.Number.Uint64(), r.defaultBlock))
			}

			r.checkForNewInputs(ctx, apps, blockNumber)

			r.checkForClaimStatus(ctx, apps, blockNumber)

			r.checkForOutputExecution(ctx, apps, blockNumber)

		}
	}
}

// fetchMostRecentHeader fetches the most recent header up till the
// given default block
func (r *EvmReader) fetchMostRecentHeader(
	ctx context.Context,
	defaultBlock DefaultBlock,
) (*types.Header, error) {

	var defaultBlockNumber int64
	switch defaultBlock {
	case DefaultBlockStatusPending:
		defaultBlockNumber = rpc.PendingBlockNumber.Int64()
	case DefaultBlockStatusLatest:
		defaultBlockNumber = rpc.LatestBlockNumber.Int64()
	case DefaultBlockStatusFinalized:
		defaultBlockNumber = rpc.FinalizedBlockNumber.Int64()
	case DefaultBlockStatusSafe:
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
func (r *EvmReader) getAppContracts(app Application,
) (ApplicationContract, ConsensusContract, error) {
	applicationContract, err := r.contractFactory.NewApplication(app.ContractAddress)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("error building application contract"),
			err,
		)

	}
	consensusAddress, err := applicationContract.GetConsensus(nil)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("error retrieving application consensus"),
			err,
		)
	}

	if app.IConsensusAddress != consensusAddress {
		return nil, nil,
			fmt.Errorf("IConsensus addresses do not match. Deployed: %s. Configured: %s",
				consensusAddress,
				app.IConsensusAddress)
	}

	consensus, err := r.contractFactory.NewIConsensus(consensusAddress)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("error building consensus contract"),
			err,
		)

	}
	return applicationContract, consensus, nil
}
