// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package claimer

import (
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"testing"

	"github.com/cartesi/rollups-node/internal/model"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/pkg/contracts/iconsensus"
	"github.com/cartesi/rollups-node/pkg/service"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/lmittmann/tint"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type serviceMock struct {
	mock.Mock
	Service
}

func (m *serviceMock) selectSubmissionClaimPairsPerApp() (
	map[common.Address]*ClaimRow,
	map[common.Address]*ClaimRow,
	error,
) {
	args := m.Called()
	return args.Get(0).(map[common.Address]*ClaimRow),
		args.Get(1).(map[common.Address]*ClaimRow),
		args.Error(2)
}
func (m *serviceMock) selectAcceptanceClaimPairsPerApp() (
	map[common.Address]*ClaimRow,
	map[common.Address]*ClaimRow,
	error,
) {
	args := m.Called()
	return args.Get(0).(map[common.Address]*ClaimRow),
		args.Get(1).(map[common.Address]*ClaimRow),
		args.Error(2)
}
func (m *serviceMock) updateEpochWithSubmittedClaim(
	claim *ClaimRow,
	txHash common.Hash,
) error {
	args := m.Called(claim, txHash)
	return args.Error(0)
}

func (m *serviceMock) updateApplicationState(appID int64, state ApplicationState, reason *string) error {
	args := m.Called(appID, state, reason)
	return args.Error(0)
}

func (m *serviceMock) updateEpochWithAcceptedClaim(
	claim *ClaimRow,
	txHash common.Hash,
) error {
	args := m.Called(claim, txHash)
	return args.Error(0)
}

func (m *serviceMock) findClaimSubmissionEventAndSucc(
	claim *ClaimRow,
	endBlock *big.Int,
) (
	*iconsensus.IConsensus,
	*iconsensus.IConsensusClaimSubmission,
	*iconsensus.IConsensusClaimSubmission,
	error,
) {
	args := m.Called(claim, endBlock)
	return args.Get(0).(*iconsensus.IConsensus),
		args.Get(1).(*iconsensus.IConsensusClaimSubmission),
		args.Get(2).(*iconsensus.IConsensusClaimSubmission),
		args.Error(3)
}
func (m *serviceMock) findClaimAcceptanceEventAndSucc(
	claim *ClaimRow,
	endBlock *big.Int,
) (
	*iconsensus.IConsensus,
	*iconsensus.IConsensusClaimAcceptance,
	*iconsensus.IConsensusClaimAcceptance,
	error,
) {
	args := m.Called(claim, endBlock)
	return args.Get(0).(*iconsensus.IConsensus),
		args.Get(1).(*iconsensus.IConsensusClaimAcceptance),
		args.Get(2).(*iconsensus.IConsensusClaimAcceptance),
		args.Error(3)
}

func (m *serviceMock) submitClaimToBlockchain(
	instance *iconsensus.IConsensus,
	claim *ClaimRow,
) (common.Hash, error) {
	args := m.Called(nil, claim)
	return args.Get(0).(common.Hash), args.Error(1)
}
func (m *serviceMock) pollTransaction(txHash common.Hash, endBlock *big.Int) (bool, *types.Receipt, error) {
	args := m.Called(txHash, endBlock)
	return args.Bool(0),
		args.Get(1).(*types.Receipt),
		args.Error(2)
}

func newServiceMock() *serviceMock {
	opts := &tint.Options{
		Level:     slog.LevelDebug,
		AddSource: true,
		// RFC3339 with milliseconds and without timezone
		TimeFormat: "2006-01-02T15:04:05.000",
	}
	handler := tint.NewHandler(os.Stdout, opts)

	return &serviceMock{
		Service: Service{
			Service: service.Service{
				Logger: slog.New(handler),
			},
			submissionEnabled: true,
			claimsInFlight:    map[common.Address]common.Hash{},
		},
	}
}

func makeAcceptedClaim(i uint64) *ClaimRow {
	hash := common.HexToHash("0x01")
	tx := common.HexToHash("0x02")
	return &ClaimRow{
		IApplicationAddress: common.HexToAddress("0x01"),
		IConsensusAddress:   common.HexToAddress("0x02"),
		Epoch: Epoch{
			Index:                i,
			FirstBlock:           i * 10,
			LastBlock:            i*10 + 9,
			ClaimHash:            &hash,
			ClaimTransactionHash: &tx,
			Status:               model.EpochStatus_ClaimAccepted,
		},
	}
}

func makeSubmittedClaim(i uint64) *ClaimRow {
	hash := common.HexToHash("0x01")
	tx := common.HexToHash("0x02")
	return &ClaimRow{
		IApplicationAddress: common.HexToAddress("0x01"),
		IConsensusAddress:   common.HexToAddress("0x02"),
		Epoch: Epoch{
			Index:                i,
			FirstBlock:           i * 10,
			LastBlock:            i*10 + 9,
			ClaimHash:            &hash,
			ClaimTransactionHash: &tx,
			Status:               model.EpochStatus_ClaimSubmitted,
		},
	}
}

func makeComputedClaim(i uint64) *ClaimRow {
	hash := common.HexToHash("0x01")
	return &ClaimRow{
		IApplicationAddress: common.HexToAddress("0x01"),
		IConsensusAddress:   common.HexToAddress("0x02"),
		Epoch: Epoch{
			Index:                i,
			FirstBlock:           i * 10,
			LastBlock:            i*10 + 9,
			ClaimHash:            &hash,
			ClaimTransactionHash: nil,
			Status:               model.EpochStatus_ClaimComputed,
		},
	}
}

func makeSubmissionEvent(c *ClaimRow) *iconsensus.IConsensusClaimSubmission {
	return &iconsensus.IConsensusClaimSubmission{
		LastProcessedBlockNumber: new(big.Int).SetUint64(c.LastBlock),
		AppContract:              c.IApplicationAddress,
		Claim:                    *c.ClaimHash,
		Raw: types.Log{
			TxHash: common.HexToHash("0x01"),
		},
	}
}

func makeAcceptanceEvent(c *ClaimRow) *iconsensus.IConsensusClaimAcceptance {
	return &iconsensus.IConsensusClaimAcceptance{
		LastProcessedBlockNumber: new(big.Int).SetUint64(c.LastBlock),
		AppContract:              c.IApplicationAddress,
		Claim:                    *c.ClaimHash,
		Raw: types.Log{
			TxHash: common.HexToHash("0x01"),
		},
	}
}

// //////////////////////////////////////////////////////////////////////////////
// Success
// //////////////////////////////////////////////////////////////////////////////
func TestDoNothing(t *testing.T) {
	m := newServiceMock()
	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)

	errs := m.submitClaimsAndUpdateDatabase(m, big.NewInt(0))
	assert.Equal(t, len(errs), 0)
}

func TestSubmitFirstClaim(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	currClaim := makeComputedClaim(3)
	var prevEvent *iconsensus.IConsensusClaimSubmission = nil
	var currEvent *iconsensus.IConsensusClaimSubmission = nil

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", currClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("submitClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 1)
}

func TestSubmitClaimWithAntecessor(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeAcceptedClaim(1)
	currClaim := makeComputedClaim(3)
	var currEvent *iconsensus.IConsensusClaimSubmission = nil
	prevEvent := makeSubmissionEvent(prevClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("submitClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 1)
}

func TestSkipSubmitFirstClaim(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)
	m.submissionEnabled = false

	currClaim := makeComputedClaim(3)
	var prevEvent *iconsensus.IConsensusClaimSubmission = nil
	var currEvent *iconsensus.IConsensusClaimSubmission = nil

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", currClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("submitClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestSkipSubmitClaimWithAntecessor(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)
	m.submissionEnabled = false

	prevClaim := makeAcceptedClaim(1)
	currClaim := makeComputedClaim(3)
	prevEvent := makeSubmissionEvent(prevClaim)
	var currEvent *iconsensus.IConsensusClaimSubmission = nil

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("submitClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestInFlightCompleted(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)
	reqHash := common.HexToHash("0x10")
	txHash := common.HexToHash("0x1000")

	currClaim := makeComputedClaim(3)
	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}
	m.claimsInFlight[currClaim.IApplicationAddress] = reqHash

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("pollTransaction", reqHash, endBlock).
		Return(true, &types.Receipt{
			ContractAddress: currClaim.IApplicationAddress,
			TxHash:          txHash,
		}, nil).Once()
	m.On("updateEpochWithSubmittedClaim", currClaim, txHash).
		Return(nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestUpdateFirstClaim(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	currClaim := makeComputedClaim(3)
	var prevEvent *iconsensus.IConsensusClaimSubmission = nil
	currEvent := makeSubmissionEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}
	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", currClaim, endBlock).
		Return(&iconsensus.IConsensus{}, currEvent, prevEvent, nil).Once()
	m.On("updateEpochWithSubmittedClaim", currClaim, currEvent.Raw.TxHash).
		Return(nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestUpdateClaimWithAntecessor(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeAcceptedClaim(1)
	currClaim := makeComputedClaim(3)
	prevEvent := makeSubmissionEvent(prevClaim)
	currEvent := makeSubmissionEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}
	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("updateEpochWithSubmittedClaim", currClaim, currEvent.Raw.TxHash).
		Return(nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
	assert.Equal(t, len(m.claimsInFlight), 0)
}

func TestAcceptFirstClaim(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	currClaim := makeSubmittedClaim(3)
	var prevEvent *iconsensus.IConsensusClaimAcceptance = nil
	currEvent := makeAcceptanceEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimAcceptanceEventAndSucc", currClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
}

func TestAcceptClaimWithAntecessor(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeAcceptedClaim(1)
	currClaim := makeSubmittedClaim(3)
	prevEvent := makeAcceptanceEvent(prevClaim)
	currEvent := makeAcceptanceEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimAcceptanceEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("updateEpochWithAcceptedClaim", currClaim, currEvent.Raw.TxHash).
		Return(nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
}

// //////////////////////////////////////////////////////////////////////////////
// Failure
// //////////////////////////////////////////////////////////////////////////////

// try again later on failure to fetch claims
func TestDatabaseSelectFailure(t *testing.T) {
	m := newServiceMock()

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, expectedErr).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], expectedErr)
}

func TestClaimInFlightMissingFromCurrClaims(t *testing.T) {
	m := newServiceMock()

	endBlock := big.NewInt(0)
	reqHash := common.HexToHash("0x01")
	receipt := new(types.Receipt)

	currClaim := makeComputedClaim(3)

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{}
	m.claimsInFlight[currClaim.IApplicationAddress] = reqHash

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("pollTransaction", reqHash, endBlock).
		Return(true, receipt, nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, len(errs), 0)
}

// submit again after pollTransaction failure
func TestSubmitFailedClaim(t *testing.T) {
	m := newServiceMock()

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)
	reqHash := common.HexToHash("0x01")
	var nilReceipt *types.Receipt

	prevClaim := makeAcceptedClaim(1)
	currClaim := makeComputedClaim(3)
	prevEvent := makeSubmissionEvent(prevClaim)
	var currEvent *iconsensus.IConsensusClaimSubmission = nil

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}
	m.claimsInFlight[currClaim.IApplicationAddress] = reqHash

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("pollTransaction", reqHash, endBlock).
		Return(false, nilReceipt, expectedErr).Once()
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("submitClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 0, len(errs))

	// submission failed and got retried
	m.AssertNumberOfCalls(t, "findClaimSubmissionEventAndSucc", 1)
	m.AssertNumberOfCalls(t, "pollTransaction", 1)
	m.AssertNumberOfCalls(t, "selectSubmissionClaimPairsPerApp", 1)
	m.AssertNumberOfCalls(t, "submitClaimToBlockchain", 1)
	m.AssertNumberOfCalls(t, "updateEpochWithSubmittedClaim", 0)
}

// !claimSubmissionMatche(prevClaim, prevEvent)
func TestSubmitClaimWithAntecessorMismatch(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)
	prevClaim := makeAcceptedClaim(1)
	currClaim := makeComputedClaim(3)
	claimTransactionHash := common.HexToHash("0x10")

	prevEvent := &iconsensus.IConsensusClaimSubmission{
		LastProcessedBlockNumber: new(big.Int).SetUint64(prevClaim.LastBlock + 1),
		AppContract:              prevClaim.IApplicationAddress,
		Claim:                    *prevClaim.ClaimHash,
	}
	var currEvent *iconsensus.IConsensusClaimSubmission = nil
	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil)
	m.On("updateApplicationState", int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)
	m.On("submitClaimToBlockchain", nil, &currClaim).
		Return(claimTransactionHash, nil)

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], ErrEventMismatch)
}

// !claimMatchesEvent(currClaim, currEvent)
func TestSubmitClaimWithEventMismatch(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeAcceptedClaim(1)
	wrongClaim := makeComputedClaim(2)
	currClaim := makeComputedClaim(3)
	wrongEvent := makeSubmissionEvent(wrongClaim)
	prevEvent := makeSubmissionEvent(prevClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, wrongEvent, nil)
	m.On("updateApplicationState", int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], ErrEventMismatch)
}

// !checkClaimsConstraint(prevClaim, currClaim)
func TestSubmitClaimWithAntecessorOutOfOrder(t *testing.T) {
	m := newServiceMock()

	wrongClaim := makeComputedClaim(2)
	wrongEvent := makeSubmissionEvent(wrongClaim)
	currClaim := makeComputedClaim(1)
	var currEvent *iconsensus.IConsensusClaimSubmission = nil

	prevClaims := map[common.Address]*ClaimRow{
		wrongClaim.IApplicationAddress: wrongClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)
	m.On("findClaimSubmissionEventAndSucc", wrongClaim).
		Return(&iconsensus.IConsensus{}, wrongEvent, currEvent, nil)
	m.On("submitClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil)
	m.On("updateApplicationState", int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)

	errs := m.submitClaimsAndUpdateDatabase(m, big.NewInt(0))
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], ErrClaimMismatch)
}

func TestErrSubmissionMissingEvent(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeComputedClaim(1)
	var prevEvent *iconsensus.IConsensusClaimSubmission = nil
	currClaim := makeComputedClaim(2)
	currEvent := makeSubmissionEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectSubmissionClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimSubmissionEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("updateApplicationState", int64(0), model.ApplicationState_Inoperable, mock.Anything).
		Return(nil)

	errs := m.submitClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], ErrMissingEvent)
}

////////////////////////////////////////////////////////////////////////////////

func TestDatabaseAcceptanceSelectFailure(t *testing.T) {
	m := newServiceMock()

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], expectedErr)
}

func TestFindClaimAcceptanceEventAndSuccFailure0(t *testing.T) {
	m := newServiceMock()

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	currClaim := makeComputedClaim(2)
	currEvent := makeAcceptanceEvent(currClaim)
	var prevEvent *iconsensus.IConsensusClaimAcceptance = nil

	prevClaims := map[common.Address]*ClaimRow{}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimAcceptanceEventAndSucc", currClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, expectedErr).Once()
	m.On("AcceptClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], expectedErr)
}

func TestFindClaimAcceptanceEventAndSuccFailure1(t *testing.T) {
	m := newServiceMock()

	expectedErr := fmt.Errorf("not found")
	endBlock := big.NewInt(0)

	prevClaim := makeComputedClaim(1)
	prevEvent := makeAcceptanceEvent(prevClaim)
	currClaim := makeComputedClaim(2)
	currEvent := makeAcceptanceEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimAcceptanceEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, errs[0], expectedErr)
}

// !claimAcceptanceMatch(prevClaim, prevEvent)
func TestAcceptClaimWithAntecessorMismatch(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)
	prevClaim := makeAcceptedClaim(1)
	currClaim := makeComputedClaim(3)

	prevEvent := &iconsensus.IConsensusClaimAcceptance{
		LastProcessedBlockNumber: new(big.Int).SetUint64(prevClaim.LastBlock + 1),
		AppContract:              prevClaim.IApplicationAddress,
		Claim:                    *prevClaim.ClaimHash,
	}
	var currEvent *iconsensus.IConsensusClaimAcceptance = nil
	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)
	m.On("findClaimAcceptanceEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil)
	m.On("AcceptClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil)

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, ErrEventMismatch, errs[0])
}

// !claimAcceptanceMatch(currClaim, currEvent)
func TestAcceptClaimWithEventMismatch(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeAcceptedClaim(1)
	wrongClaim := makeComputedClaim(2)
	currClaim := makeComputedClaim(3)
	wrongEvent := makeAcceptanceEvent(wrongClaim)
	prevEvent := makeAcceptanceEvent(prevClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)
	m.On("findClaimAcceptanceEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, wrongEvent, nil)

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, ErrEventMismatch, errs[0])
}

// !checkClaimsConstraint(prevClaim, currClaim)
func TestAcceptClaimWithAntecessorOutOfOrder(t *testing.T) {
	m := newServiceMock()

	wrongClaim := makeComputedClaim(2)
	wrongEvent := makeAcceptanceEvent(wrongClaim)
	currClaim := makeComputedClaim(1)
	var currEvent *iconsensus.IConsensusClaimAcceptance = nil

	prevClaims := map[common.Address]*ClaimRow{
		wrongClaim.IApplicationAddress: wrongClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil)
	m.On("findClaimAcceptanceEventAndSucc", wrongClaim).
		Return(&iconsensus.IConsensus{}, wrongEvent, currEvent, nil)
	m.On("AcceptClaimToBlockchain", nil, currClaim).
		Return(common.HexToHash("0x10"), nil)

	errs := m.acceptClaimsAndUpdateDatabase(m, big.NewInt(0))
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, ErrClaimMismatch, errs[0])
}

func TestErrAcceptanceMissingEvent(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)

	prevClaim := makeComputedClaim(1)
	var prevEvent *iconsensus.IConsensusClaimAcceptance = nil
	currClaim := makeComputedClaim(2)
	currEvent := makeAcceptanceEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimAcceptanceEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, ErrMissingEvent, errs[0])
}

func TestUpdateEpochWithAcceptedClaimFailed(t *testing.T) {
	m := newServiceMock()
	endBlock := big.NewInt(0)
	expectedErr := fmt.Errorf("not found")

	prevClaim := makeSubmittedClaim(1)
	prevEvent := makeAcceptanceEvent(prevClaim)
	currClaim := makeSubmittedClaim(2)
	currEvent := makeAcceptanceEvent(currClaim)

	prevClaims := map[common.Address]*ClaimRow{
		prevClaim.IApplicationAddress: prevClaim,
	}
	currClaims := map[common.Address]*ClaimRow{
		currClaim.IApplicationAddress: currClaim,
	}

	m.On("selectAcceptanceClaimPairsPerApp").
		Return(prevClaims, currClaims, nil).Once()
	m.On("findClaimAcceptanceEventAndSucc", prevClaim, endBlock).
		Return(&iconsensus.IConsensus{}, prevEvent, currEvent, nil).Once()
	m.On("updateEpochWithAcceptedClaim", currClaim, currEvent.Raw.TxHash).
		Return(expectedErr).Once()

	errs := m.acceptClaimsAndUpdateDatabase(m, endBlock)
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, expectedErr, errs[0])
}
