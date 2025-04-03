// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package claimer

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/cartesi/rollups-node/internal/model"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/lmittmann/tint"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type claimerRepositoryMock struct {
	mock.Mock
}

func (m *claimerRepositoryMock) ListApplications(
	ctx context.Context,
	f repository.ApplicationFilter,
	pagination repository.Pagination,
) ([]*Application, uint64, error) {
	args := m.Called(ctx, f, pagination)
	return args.Get(0).([]*Application), args.Get(1).(uint64), args.Error(2)
}

func (m *claimerRepositoryMock) SelectSubmittedClaimPairsPerApp(ctx context.Context) (
	map[int64]*model.Epoch,
	map[int64]*model.Epoch,
	map[int64]*model.Application,
	error,
) {
	args := m.Called(ctx)
	return args.Get(0).(map[int64]*model.Epoch),
		args.Get(1).(map[int64]*model.Epoch),
		args.Get(2).(map[int64]*model.Application),
		args.Error(3)
}

func (m *claimerRepositoryMock) SelectAcceptedClaimPairsPerApp(ctx context.Context) (
	map[int64]*model.Epoch,
	map[int64]*model.Epoch,
	map[int64]*model.Application,
	error,
) {
	args := m.Called(ctx)
	return args.Get(0).(map[int64]*model.Epoch),
		args.Get(1).(map[int64]*model.Epoch),
		args.Get(2).(map[int64]*model.Application),
		args.Error(3)
}
func (m *claimerRepositoryMock) UpdateEpochWithSubmittedClaim(
	ctx context.Context,
	appid int64,
	index uint64,
	txHash common.Hash,
) error {
	args := m.Called(ctx, appid, index, txHash)
	return args.Error(0)
}

func (m *claimerRepositoryMock) UpdateApplicationState(
	ctx context.Context,
	appID int64,
	state ApplicationState,
	reason *string,
) error {
	args := m.Called(ctx, appID, state, reason)
	return args.Error(0)
}

func (m *claimerRepositoryMock) UpdateEpochWithAcceptedClaim(
	ctx context.Context,
	appid int64,
	index uint64,
) error {
	args := m.Called(ctx, appid, index)
	return args.Error(0)
}

func (m *claimerRepositoryMock) SaveNodeConfigRaw(
	ctx context.Context,
	key string,
	rawJSON []byte,
) error {
	args := m.Called(ctx, key, rawJSON)
	return args.Error(0)
}

func (m *claimerRepositoryMock) LoadNodeConfigRaw(ctx context.Context, key string) (
	rawJSON []byte,
	createdAt, updatedAt time.Time,
	err error,
) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Get(1).(time.Time), args.Get(2).(time.Time), args.Error(3)
}

type claimerBlockchainMock struct {
	mock.Mock
}

func (m *claimerBlockchainMock) findClaimSubmittedEventAndSucc(
	ctx context.Context,
	app *model.Application,
	epoch *model.Epoch,
	endBlock *big.Int,
) (
	*iconsensus.IConsensus,
	*iconsensus.IConsensusClaimSubmitted,
	*iconsensus.IConsensusClaimSubmitted,
	error,
) {
	args := m.Called(app, epoch, endBlock)
	return args.Get(0).(*iconsensus.IConsensus),
		args.Get(1).(*iconsensus.IConsensusClaimSubmitted),
		args.Get(2).(*iconsensus.IConsensusClaimSubmitted),
		args.Error(3)
}

func (m *claimerBlockchainMock) findClaimAcceptedEventAndSucc(
	ctx context.Context,
	app *model.Application,
	epoch *model.Epoch,
	endBlock *big.Int,
) (
	*iconsensus.IConsensus,
	*iconsensus.IConsensusClaimAccepted,
	*iconsensus.IConsensusClaimAccepted,
	error,
) {
	args := m.Called(ctx, app, epoch, endBlock)
	return args.Get(0).(*iconsensus.IConsensus),
		args.Get(1).(*iconsensus.IConsensusClaimAccepted),
		args.Get(2).(*iconsensus.IConsensusClaimAccepted),
		args.Error(3)
}

func (m *claimerBlockchainMock) submitClaimToBlockchain(
	instance *iconsensus.IConsensus,
	app *model.Application,
	epoch *model.Epoch,
) (common.Hash, error) {
	args := m.Called(instance, app, epoch)
	return args.Get(0).(common.Hash), args.Error(1)
}
func (m *claimerBlockchainMock) pollTransaction(
	ctx context.Context,
	txHash common.Hash,
	endBlock *big.Int,
) (bool, *types.Receipt, error) {
	args := m.Called(ctx, txHash, endBlock)
	return args.Bool(0),
		args.Get(1).(*types.Receipt),
		args.Error(2)
}
func (m *claimerBlockchainMock) getBlockNumber(ctx context.Context) (*big.Int, error) {
	args := m.Called(ctx)
	return args.Get(0).(*big.Int),
		args.Error(1)
}

func (m *claimerBlockchainMock) getConsensusAddress(
	ctx context.Context,
	app *model.Application,
) (common.Address, error) {
	args := m.Called(ctx, app)
	return args.Get(0).(common.Address),
		args.Error(1)
}

func newServiceMock() (*Service, *claimerRepositoryMock, *claimerBlockchainMock) {
	opts := &tint.Options{
		Level:     slog.LevelDebug,
		AddSource: true,
		// RFC3339 with milliseconds and without timezone
		TimeFormat: "2006-01-02T15:04:05.000",
	}
	handler := tint.NewHandler(os.Stdout, opts)
	repository := &claimerRepositoryMock{}
	blockchain := &claimerBlockchainMock{}

	claimer := &Service{
		Service: service.Service{
			Logger: slog.New(handler),
		},
		submissionEnabled: true,
		claimsInFlight:    map[int64]common.Hash{},
		repository:        repository,
		blockchain:        blockchain,
	}
	return claimer, repository, blockchain
}

func makeApplication(id int64) *model.Application {
	return &model.Application{
		ID:                  id,
		IApplicationAddress: common.HexToAddress("0x01"),
		IConsensusAddress:   common.HexToAddress("0x01"),
		IInputBoxAddress:    common.HexToAddress("0x02"),
	}
}

func makeEpoch(id int64, status model.EpochStatus, i uint64) *model.Epoch {
	hash := common.HexToHash("0x01")
	tx := common.HexToHash("0x02")
	epoch := &Epoch{
		ApplicationID:        id,
		Index:                i,
		FirstBlock:           i * 10,
		LastBlock:            i*10 + 9,
		Status:               status,
		ClaimTransactionHash: &tx,
		ClaimHash:            &hash,
	}
	return epoch
}

func makeAcceptedEpoch(app *model.Application, i uint64) *model.Epoch {
	return makeEpoch(app.ID, model.EpochStatus_ClaimAccepted, i)
}

func makeSubmittedEpoch(app *model.Application, i uint64) *model.Epoch {
	return makeEpoch(app.ID, model.EpochStatus_ClaimSubmitted, i)
}

func makeComputedEpoch(app *model.Application, i uint64) *model.Epoch {
	return makeEpoch(app.ID, model.EpochStatus_ClaimComputed, i)
}
func makeEpochMap(epochs ...*model.Epoch) map[int64]*model.Epoch {
	result := map[int64]*Epoch{}
	for _, epoch := range epochs {
		result[epoch.ApplicationID] = epoch
	}
	return result
}
func makeApplicationMap(apps ...*model.Application) map[int64]*model.Application {
	result := map[int64]*Application{}
	for _, app := range apps {
		result[app.ID] = app
	}
	return result
}

func makeSubmittedEvent(app *model.Application, epoch *model.Epoch) *iconsensus.IConsensusClaimSubmitted {
	return &iconsensus.IConsensusClaimSubmitted{
		LastProcessedBlockNumber: new(big.Int).SetUint64(epoch.LastBlock),
		AppContract:              app.IApplicationAddress,
		OutputsMerkleRoot:        *epoch.ClaimHash,
		Raw: types.Log{
			TxHash: common.HexToHash(epoch.ClaimTransactionHash.Hex()),
		},
	}
}

func makeAcceptedEvent(app *model.Application, epoch *model.Epoch) *iconsensus.IConsensusClaimAccepted {
	return &iconsensus.IConsensusClaimAccepted{
		LastProcessedBlockNumber: new(big.Int).SetUint64(epoch.LastBlock),
		AppContract:              app.IApplicationAddress,
		OutputsMerkleRoot:        *epoch.ClaimHash,
		Raw: types.Log{
			TxHash: common.HexToHash(epoch.ClaimTransactionHash.Hex()),
		},
	}
}

// //////////////////////////////////////////////////////////////////////////////
// Success
// //////////////////////////////////////////////////////////////////////////////
func TestDoNothing(t *testing.T) {
	m, r, _ := newServiceMock()
	defer r.AssertExpectations(t)

	prevEpochs := makeEpochMap()
	currEpochs := makeEpochMap()

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(prevEpochs, currEpochs, makeApplicationMap(), nil)

	errs := m.submitClaimsAndUpdateDatabase(big.NewInt(0))
	assert.Equal(t, len(errs), 0)
}

func TestSubmitFirstClaim(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 3)
	var prevEvent *iconsensus.IConsensusClaimSubmitted = nil
	var currEvent *iconsensus.IConsensusClaimSubmitted = nil

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, currEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	b.On("submitClaimToBlockchain", mock.Anything, app, currEpoch).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 1)
}

func TestSubmitClaimWithAntecessor(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)
	var currEvent *iconsensus.IConsensusClaimSubmitted = nil
	prevEvent := makeSubmittedEvent(app, prevEpoch)

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	b.On("submitClaimToBlockchain", mock.Anything, app, currEpoch).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 1)
}

func TestSkipSubmitFirstClaim(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	m.submissionEnabled = false
	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 3)
	var prevEvent *iconsensus.IConsensusClaimSubmitted = nil
	var currEvent *iconsensus.IConsensusClaimSubmitted = nil

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, currEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestSkipSubmitClaimWithAntecessor(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	m.submissionEnabled = false
	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)
	prevEvent := makeSubmittedEvent(app, prevEpoch)
	var currEvent *iconsensus.IConsensusClaimSubmitted = nil

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestInFlightCompleted(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	txHash := common.HexToHash("0x10")
	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 3)
	currEpoch.ClaimTransactionHash = &txHash

	m.claimsInFlight[app.ID] = *currEpoch.ClaimTransactionHash

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("pollTransaction", mock.Anything, txHash, endBlock).
		Return(true, &types.Receipt{
			ContractAddress: app.IApplicationAddress,
			TxHash:          *currEpoch.ClaimTransactionHash,
		}, nil).Once()
	r.On("UpdateEpochWithSubmittedClaim", mock.Anything, app.ID, currEpoch.Index, txHash).
		Return(nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestUpdateFirstClaim(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 3)
	var prevEvent *iconsensus.IConsensusClaimSubmitted = nil
	currEvent := makeSubmittedEvent(app, currEpoch)

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, currEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, currEvent, prevEvent, nil).Once()
	r.On("UpdateEpochWithSubmittedClaim", mock.Anything, app.ID, currEpoch.Index, currEvent.Raw.TxHash).
		Return(nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestUpdateClaimWithAntecessor(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)
	prevEvent := makeSubmittedEvent(app, prevEpoch)
	currEvent := makeSubmittedEvent(app, currEpoch)

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	r.On("UpdateEpochWithSubmittedClaim", mock.Anything, app.ID, currEpoch.Index, currEvent.Raw.TxHash).
		Return(nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestAcceptFirstClaim(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeSubmittedEpoch(app, 3)
	var prevEvent *iconsensus.IConsensusClaimAccepted = nil
	currEvent := makeAcceptedEvent(app, currEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, currEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
}

func TestAcceptClaimWithAntecessor(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeSubmittedEpoch(app, 3)
	prevEvent := makeAcceptedEvent(app, prevEpoch)
	currEvent := makeAcceptedEvent(app, currEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	r.On("UpdateEpochWithAcceptedClaim", mock.Anything, app.ID, currEpoch.Index).
		Return(nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
}

// //////////////////////////////////////////////////////////////////////////////
// Failure
// //////////////////////////////////////////////////////////////////////////////

// try again later on failure to fetch claims
func TestDatabaseSelectFailure(t *testing.T) {
	m, r, _ := newServiceMock()
	defer r.AssertExpectations(t)

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(), makeApplicationMap(), expectedErr).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], expectedErr)
}

func TestClaimInFlightMissingFromCurrClaims(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	reqHash := common.HexToHash("0x01")
	receipt := new(types.Receipt)

	app := makeApplication(0)
	m.claimsInFlight[app.ID] = reqHash

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(), makeApplicationMap(app), nil).Once()
	b.On("pollTransaction", mock.Anything, reqHash, endBlock).
		Return(true, receipt, nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 0)
}

// submit again after pollTransaction failure
func TestSubmitFailedClaim(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)
	reqHash := common.HexToHash("0x01")
	var nilReceipt *types.Receipt

	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)
	prevEvent := makeSubmittedEvent(app, prevEpoch)
	var currEvent *iconsensus.IConsensusClaimSubmitted = nil

	m.claimsInFlight[app.ID] = reqHash

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("pollTransaction", mock.Anything, reqHash, endBlock).
		Return(false, nilReceipt, expectedErr).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	b.On("submitClaimToBlockchain", mock.Anything, app, currEpoch).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 0, len(errs))
}

// !claimSubmittedMatche(prevClaim, prevEvent)
func TestSubmitClaimWithAntecessorMismatch(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)

	// event has an incorrect LastProcessedBlockNumber field.
	prevEvent := &iconsensus.IConsensusClaimSubmitted{
		LastProcessedBlockNumber: new(big.Int).SetUint64(prevEpoch.LastBlock + 1),
		AppContract:              app.IApplicationAddress,
		OutputsMerkleRoot:        *prevEpoch.ClaimHash,
	}
	var currEvent *iconsensus.IConsensusClaimSubmitted = nil

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).
		Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).
		Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).
		Once()
	r.On("UpdateApplicationState", nil, int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil).
		Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

// !claimMatchesEvent(currClaim, currEvent)
func TestSubmitClaimWithEventMismatch(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)
	prevEvent := makeSubmittedEvent(app, prevEpoch)
	wrongEvent := makeSubmittedEvent(app, makeComputedEpoch(app, 2))

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil)
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, wrongEvent, nil)
	r.On("UpdateApplicationState", nil, int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

// !checkClaimsConstraint(prevClaim, currClaim) // epoch pair has its blocks out of order
func TestSubmitClaimWithAntecessorOutOfOrder(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	app := makeApplication(0)
	prevEpoch := makeSubmittedEpoch(app, 2)
	currEpoch := makeComputedEpoch(app, 1)

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil)
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	r.On("UpdateApplicationState", nil, int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)

	errs := m.submitClaimsAndUpdateDatabase(big.NewInt(0))
	assert.Equal(t, 1, len(errs))
}

func TestErrSubmittedMissingEvent(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeComputedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 2)
	var prevEvent *iconsensus.IConsensusClaimSubmitted = nil
	currEvent := makeSubmittedEvent(app, currEpoch)

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimSubmittedEventAndSucc", app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	r.On("UpdateApplicationState", nil, int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

func TestConsensusAddressChangedOnSubmitedClaims(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 3)
	wrongConsensusAddress := app.IConsensusAddress
	wrongConsensusAddress[0]++

	r.On("SelectSubmittedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).
		Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(wrongConsensusAddress, nil).
		Once()
	r.On("UpdateApplicationState", nil, int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil).
		Once()

	errs := m.submitClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 1)
}

////////////////////////////////////////////////////////////////////////////////

func TestDatabaseAcceptedSelectFailure(t *testing.T) {
	m, r, _ := newServiceMock()
	defer r.AssertExpectations(t)

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(), makeEpochMap(), makeApplicationMap(), expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

func TestFindClaimAcceptedEventAndSuccFailure0(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 2)
	var prevEvent *iconsensus.IConsensusClaimAccepted = nil
	currEvent := makeAcceptedEvent(app, currEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, currEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

func TestFindClaimAcceptedEventAndSuccFailure1(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 2)
	prevEvent := makeAcceptedEvent(app, prevEpoch)
	currEvent := makeAcceptedEvent(app, currEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

// !claimAcceptedMatch(prevClaim, prevEvent)
func TestAcceptClaimWithAntecessorMismatch(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 3)

	prevEvent := &iconsensus.IConsensusClaimAccepted{
		LastProcessedBlockNumber: new(big.Int).SetUint64(prevEpoch.LastBlock + 1),
		AppContract:              app.IApplicationAddress,
		OutputsMerkleRoot:        *prevEpoch.ClaimHash,
	}
	var currEvent *iconsensus.IConsensusClaimAccepted = nil

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil)
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil)

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

// !claimAcceptedMatch(currClaim, currEvent)
func TestAcceptClaimWithEventMismatch(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeAcceptedEpoch(app, 1)
	wrongEpoch := makeComputedEpoch(app, 2)
	currEpoch := makeComputedEpoch(app, 3)
	wrongEvent := makeAcceptedEvent(app, wrongEpoch)
	prevEvent := makeAcceptedEvent(app, prevEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, wrongEvent, nil)

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

// !checkClaimsConstraint(prevClaim, currClaim)
func TestAcceptClaimWithAntecessorOutOfOrder(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	app := makeApplication(0)
	wrongEpoch := makeComputedEpoch(app, 2)
	currEpoch := makeComputedEpoch(app, 1)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(wrongEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil)
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(big.NewInt(0))
	assert.Equal(t, 1, len(errs))
}

func TestErrAcceptedMissingEvent(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeComputedEpoch(app, 1)
	currEpoch := makeComputedEpoch(app, 2)
	var prevEvent *iconsensus.IConsensusClaimAccepted = nil
	currEvent := makeAcceptedEvent(app, currEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

func TestUpdateEpochWithAcceptedClaimFailed(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	expectedErr := fmt.Errorf("not found")

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	prevEpoch := makeSubmittedEpoch(app, 1)
	currEpoch := makeSubmittedEpoch(app, 2)
	prevEvent := makeAcceptedEvent(app, prevEpoch)
	currEvent := makeAcceptedEvent(app, currEpoch)

	r.On("SelectAcceptedClaimPairsPerApp", mock.Anything).
		Return(makeEpochMap(prevEpoch), makeEpochMap(currEpoch), makeApplicationMap(app), nil).Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(app.IConsensusAddress, nil).Once()
	b.On("findClaimAcceptedEventAndSucc", mock.Anything, app, prevEpoch, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	r.On("UpdateEpochWithAcceptedClaim", mock.Anything, app.ID, currEpoch.Index).
		Return(expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, 1, len(errs))
}

func TestConsensusAddressChangedOnAcceptedClaims(t *testing.T) {
	m, r, b := newServiceMock()
	defer r.AssertExpectations(t)
	defer b.AssertExpectations(t)

	endBlock := big.NewInt(0)
	app := makeApplication(0)
	currEpoch := makeComputedEpoch(app, 3)
	wrongConsensusAddress := app.IConsensusAddress
	wrongConsensusAddress[0]++

	r.On("SelectAcceptedClaimPairsPerApp", nil).
		Return(makeEpochMap(), makeEpochMap(currEpoch), makeApplicationMap(app), nil).
		Once()
	b.On("getConsensusAddress", mock.Anything, app).
		Return(wrongConsensusAddress, nil).
		Once()
	r.On("UpdateApplicationState", nil, int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil).
		Once()

	errs := m.acceptClaimsAndUpdateDatabase(endBlock)
	assert.Equal(t, len(errs), 1)
}
