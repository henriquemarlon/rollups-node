// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package validator

import (
	"context"
	crand "crypto/rand"
	"testing"
	"time"

	"github.com/cartesi/rollups-node/internal/merkle"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ValidatorSuite struct {
	suite.Suite
}

func TestValidatorSuite(t *testing.T) {
	suite.Run(t, new(ValidatorSuite))
}

var (
	validator   *Service
	repo        *Mockrepo
	dummyEpochs []Epoch
)

func (s *ValidatorSuite) SetupSubTest() {
	repo = newMockrepo()
	validator = &Service{}
	s.Require().Nil(Create(&CreateInfo{
		Repository:     repo,
		MaxStartupTime: 5 * time.Second,
		CreateInfo: service.CreateInfo{
			Impl: validator,
		},
	}, validator))
	dummyEpochs = []Epoch{
		{Index: 0, FirstBlock: 0, LastBlock: 9},
		{Index: 1, FirstBlock: 10, LastBlock: 19},
		{Index: 2, FirstBlock: 20, LastBlock: 29},
		{Index: 3, FirstBlock: 30, LastBlock: 39},
	}
}

func (s *ValidatorSuite) TearDownSubTest() {
	repo = nil
	validator = nil
}

func (s *ValidatorSuite) TestItFailsWhenClaimDoesNotMatchMachineOutputsHash() {
	s.Run("OneAppSingleEpoch", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		app := Application{ContractAddress: randomAddress()}
		epochs := []Epoch{dummyEpochs[0]}
		epochs[0].AppAddress = app.ContractAddress
		mismatchedHash := randomHash()
		repo.On(
			"GetProcessedEpochs", mock.Anything, epochs[0].AppAddress,
		).Return(epochs, nil)
		repo.On(
			"GetLastInputOutputsHash",
			mock.Anything, epochs[0].Index, epochs[0].AppAddress,
		).Return(&mismatchedHash, nil)
		repo.On(
			"GetOutputsProducedInBlockRange",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil, nil)
		repo.On("GetPreviousEpoch", mock.Anything, mock.Anything).Return(nil, nil)

		err := validator.validateApplication(ctx, app)
		s.NotNil(err)
		s.ErrorContains(err, "claim does not match")

		repo.AssertExpectations(s.T())
	})

	// fails on the second epoch, do not process the third
	s.Run("OneAppThreeEpochs", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		app := Application{ContractAddress: randomAddress()}
		epochs := []Epoch{dummyEpochs[0], dummyEpochs[1], dummyEpochs[2]}
		for idx := range epochs {
			epochs[idx].AppAddress = app.ContractAddress
		}
		epoch0Claim, _, err := merkle.CreateProofs(nil, MAX_OUTPUT_TREE_HEIGHT)
		s.Require().Nil(err)
		epochs[0].ClaimHash = &epoch0Claim
		mismatchedHash := randomHash()

		repo.On(
			"GetProcessedEpochs", mock.Anything, app.ContractAddress,
		).Return(epochs, nil).Once()
		repo.On(
			"GetOutputsProducedInBlockRange",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil, nil)
		repo.On("GetPreviousEpoch", mock.Anything, epochs[0]).Return(nil, nil)
		repo.On(
			"GetLastInputOutputsHash",
			mock.Anything, epochs[0].Index, epochs[0].AppAddress,
		).Return(epochs[0].ClaimHash, nil)
		repo.On(
			"GetLastInputOutputsHash",
			mock.Anything, epochs[1].Index, epochs[1].AppAddress,
		).Return(&mismatchedHash, nil)
		repo.On("GetPreviousEpoch", mock.Anything, epochs[1]).Return(epochs[0], nil)
		repo.On(
			"SetEpochClaimAndInsertProofsTransaction",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		err = validator.validateApplication(ctx, app)
		s.NotNil(err)
		s.ErrorContains(err, "claim does not match")
		repo.AssertExpectations(s.T())
	})

	// validates first app, fails on the first epoch of the second
	s.Run("TwoAppsTwoEpochsEach", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		applications := []Application{
			{ContractAddress: randomAddress()},
			{ContractAddress: randomAddress()},
		}
		epochsApp1 := []Epoch{dummyEpochs[0], dummyEpochs[1]}
		epochsApp2 := []Epoch{dummyEpochs[2], dummyEpochs[3]}
		for idx := range epochsApp1 {
			epochsApp1[idx].AppAddress = applications[0].ContractAddress
		}
		for idx := range epochsApp2 {
			epochsApp2[idx].AppAddress = applications[1].ContractAddress
		}
		epoch0Claim, _, err := merkle.CreateProofs(nil, MAX_OUTPUT_TREE_HEIGHT)
		s.Require().Nil(err)
		epochsApp1[0].ClaimHash = &epoch0Claim
		mismatchedHash := randomHash()

		repo.On("GetAllRunningApplications", mock.Anything).Return(applications, nil)
		repo.On(
			"GetProcessedEpochs", mock.Anything, applications[0].ContractAddress,
		).Return(epochsApp1, nil)
		repo.On(
			"GetOutputsProducedInBlockRange",
			mock.Anything, applications[0].ContractAddress, mock.Anything, mock.Anything,
		).Return(nil, nil)
		repo.On("GetPreviousEpoch", mock.Anything, epochsApp1[0]).Return(nil, nil)
		repo.On("GetPreviousEpoch", mock.Anything, epochsApp1[1]).Return(epochsApp1[0], nil)
		repo.On(
			"GetLastInputOutputsHash",
			mock.Anything, epochsApp1[0].Index, epochsApp1[0].AppAddress,
		).Return(epochsApp1[0].ClaimHash, nil)
		repo.On(
			"GetLastInputOutputsHash",
			mock.Anything, epochsApp1[1].Index, epochsApp1[1].AppAddress,
		).Return(epochsApp1[0].ClaimHash, nil)
		repo.On(
			"SetEpochClaimAndInsertProofsTransaction",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Twice()
		repo.On(
			"GetProcessedEpochs", mock.Anything, applications[1].ContractAddress,
		).Return(epochsApp2, nil)
		repo.On(
			"GetOutputsProducedInBlockRange",
			mock.Anything, applications[1].ContractAddress, mock.Anything, mock.Anything,
		).Return(nil, nil)
		repo.On("GetPreviousEpoch", mock.Anything, epochsApp2[0]).Return(nil, nil)
		repo.On(
			"GetLastInputOutputsHash",
			mock.Anything, epochsApp2[0].Index, epochsApp2[0].AppAddress,
		).Return(&mismatchedHash, nil)

		err = validator.Run(ctx)
		s.NotNil(err)
		s.ErrorContains(err, "claim does not match")
		repo.AssertExpectations(s.T())
	})
}

func randomAddress() Address {
	address := make([]byte, 20)
	_, err := crand.Read(address)
	if err != nil {
		panic(err)
	}
	return Address(address)
}

func randomHash() Hash {
	hash := make([]byte, 32)
	_, err := crand.Read(hash)
	if err != nil {
		panic(err)
	}
	return Hash(hash)
}

type Mockrepo struct {
	mock.Mock
}

func newMockrepo() *Mockrepo {
	return new(Mockrepo)
}

func (m *Mockrepo) GetAllRunningApplications(ctx context.Context) ([]Application, error) {
	args := m.Called(ctx)

	apps, ok := args.Get(0).([]Application)
	if ok {
		return apps, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *Mockrepo) GetOutputsProducedInBlockRange(
	ctx context.Context,
	application Address,
	firstBlock, lastBlock uint64,
) ([]Output, error) {
	args := m.Called(ctx, application, firstBlock, lastBlock)

	if outputs, ok := args.Get(0).([]Output); ok {
		return outputs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *Mockrepo) GetProcessedEpochs(
	ctx context.Context,
	application Address,
) ([]Epoch, error) {
	args := m.Called(ctx, application)

	if epochs, ok := args.Get(0).([]Epoch); ok {
		return epochs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *Mockrepo) GetLastInputOutputsHash(
	ctx context.Context,
	epochIndex uint64,
	appAddress Address,
) (*Hash, error) {
	args := m.Called(ctx, epochIndex, appAddress)

	if hash, ok := args.Get(0).(*Hash); ok {
		return hash, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *Mockrepo) GetPreviousEpoch(
	ctx context.Context,
	currentEpoch Epoch,
) (*Epoch, error) {
	args := m.Called(ctx, currentEpoch)

	if epoch, ok := args.Get(0).(*Epoch); ok {
		return epoch, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *Mockrepo) SetEpochClaimAndInsertProofsTransaction(
	ctx context.Context,
	epoch Epoch,
	outputs []Output,
) error {
	args := m.Called(ctx, epoch, outputs)
	return args.Error(0)
}
