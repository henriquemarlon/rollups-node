// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package validator

import (
	"context"
	"fmt"
	"testing"

	"github.com/cartesi/rollups-node/internal/merkle"
	. "github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/pkg/service"
	"github.com/ethereum/go-ethereum/common"
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
	validator    *Service
	repo         *Mockrepo
	dummyEpochs  []Epoch
	dummyOutputs []Output
)

func (s *ValidatorSuite) SetupSubTest() {
	repo = newMockrepo()
	postContext := merkle.CreatePostContext()
	validator = &Service{
		repository:          repo,
		pristinePostContext: postContext,
		pristineRootHash:    postContext[merkle.TREE_DEPTH-1],
	}
	serviceArgs := &service.CreateInfo{Name: "validator", Impl: validator}
	err := service.Create(context.Background(), serviceArgs, &validator.Service)
	s.Require().Nil(err)
	dummyClaimHash := common.HexToHash("0x4128b6c65e6131a6823bab8deee051078080bb82d505015976efe2fb3b4c91c0")
	dummyEpochs = []Epoch{
		{Index: 0, VirtualIndex: 0, FirstBlock: 0, LastBlock: 9, ClaimHash: &dummyClaimHash},
		{Index: 1, VirtualIndex: 1, FirstBlock: 10, LastBlock: 19},
		{Index: 2, VirtualIndex: 2, FirstBlock: 20, LastBlock: 29},
		{Index: 3, VirtualIndex: 3, FirstBlock: 30, LastBlock: 39},
	}
	hash := common.HexToHash("0x68b9914c8b694e0037fc9ae85670b5de94fa9d1adb10b3a037c3b170e14ee50d")
	dummyOutputs = []Output{
		{
			Index:   0,
			RawData: common.Hash{}.Bytes(),
			Hash:    &hash,
			OutputHashesSiblings: []common.Hash{
				common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
				common.HexToHash("0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5"),
				common.HexToHash("0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30"),
				common.HexToHash("0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85"),
				common.HexToHash("0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344"),
				common.HexToHash("0x0eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d"),
				common.HexToHash("0x887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968"),
				common.HexToHash("0xffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f83"),
				common.HexToHash("0x9867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756af"),
				common.HexToHash("0xcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0"),
				common.HexToHash("0xf9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5"),
				common.HexToHash("0xf8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf892"),
				common.HexToHash("0x3490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99c"),
				common.HexToHash("0xc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb"),
				common.HexToHash("0x5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8becc"),
				common.HexToHash("0xda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d2"),
				common.HexToHash("0x2733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981f"),
				common.HexToHash("0xe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a"),
				common.HexToHash("0x5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0"),
				common.HexToHash("0xb46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0"),
				common.HexToHash("0xc65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2"),
				common.HexToHash("0xf4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd9"),
				common.HexToHash("0x5a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e377"),
				common.HexToHash("0x4df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652"),
				common.HexToHash("0xcdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef"),
				common.HexToHash("0x0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618d"),
				common.HexToHash("0xb8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d0"),
				common.HexToHash("0x838c5655cb21c6cb83313b5a631175dff4963772cce9108188b34ac87c81c41e"),
				common.HexToHash("0x662ee4dd2dd7b2bc707961b1e646c4047669dcb6584f0d8d770daf5d7e7deb2e"),
				common.HexToHash("0x388ab20e2573d171a88108e79d820e98f26c0b84aa8b2f4aa4968dbb818ea322"),
				common.HexToHash("0x93237c50ba75ee485f4c22adf2f741400bdf8d6a9cc7df7ecae576221665d735"),
				common.HexToHash("0x8448818bb4ae4562849e949e17ac16e0be16688e156b5cf15e098c627c0056a9"),
				common.HexToHash("0x27ae5ba08d7291c96c8cbddcc148bf48a6d68c7974b94356f53754ef6171d757"),
				common.HexToHash("0xbf558bebd2ceec7f3c5dce04a4782f88c2c6036ae78ee206d0bc5289d20461a2"),
				common.HexToHash("0xe21908c2968c0699040a6fd866a577a99a9d2ec88745c815fd4a472c789244da"),
				common.HexToHash("0xae824d72ddc272aab68a8c3022e36f10454437c1886f3ff9927b64f232df414f"),
				common.HexToHash("0x27e429a4bef3083bc31a671d046ea5c1f5b8c3094d72868d9dfdc12c7334ac5f"),
				common.HexToHash("0x743cc5c365a9a6a15c1f240ac25880c7a9d1de290696cb766074a1d83d927816"),
				common.HexToHash("0x4adcf616c3bfabf63999a01966c998b7bb572774035a63ead49da73b5987f347"),
				common.HexToHash("0x75786645d0c5dd7c04a2f8a75dcae085213652f5bce3ea8b9b9bedd1cab3c5e9"),
				common.HexToHash("0xb88b152c9b8a7b79637d35911848b0c41e7cc7cca2ab4fe9a15f9c38bb4bb939"),
				common.HexToHash("0x0c4e2d8ce834ffd7a6cd85d7113d4521abb857774845c4291e6f6d010d97e318"),
				common.HexToHash("0x5bc799d83e3bb31501b3da786680df30fbc18eb41cbce611e8c0e9c72f69571c"),
				common.HexToHash("0xa10d3ef857d04d9c03ead7c6317d797a090fa1271ad9c7addfbcb412e9643d4f"),
				common.HexToHash("0xb33b1809c42623f474055fa9400a2027a7a885c8dfa4efe20666b4ee27d7529c"),
				common.HexToHash("0x134d7f28d53f175f6bf4b62faa2110d5b76f0f770c15e628181c1fcc18f970a9"),
				common.HexToHash("0xc34d24b2fc8c50ca9c07a7156ef4e5ff4bdf002eda0b11c1d359d0b59a546807"),
				common.HexToHash("0x04dbb9db631457879b27e0dfdbe50158fd9cf9b4cf77605c4ac4c95bd65fc9f6"),
				common.HexToHash("0xf9295a686647cb999090819cda700820c282c613cedcd218540bbc6f37b01c65"),
				common.HexToHash("0x67c4a1ea624f092a3a5cca2d6f0f0db231972fce627f0ecca0dee60f17551c5f"),
				common.HexToHash("0x8fdaeb5ab560b2ceb781cdb339361a0fbee1b9dffad59115138c8d6a70dda9cc"),
				common.HexToHash("0xc1bf0bbdd7fee15764845db875f6432559ff8dbc9055324431bc34e5b93d15da"),
				common.HexToHash("0x307317849eccd90c0c7b98870b9317c15a5959dcfb84c76dcc908c4fe6ba9212"),
				common.HexToHash("0x6339bf06e458f6646df5e83ba7c3d35bc263b3222c8e9040068847749ca8e8f9"),
				common.HexToHash("0x5045e4342aeb521eb3a5587ec268ed3aa6faf32b62b0bc41a9d549521f406fc3"),
				common.HexToHash("0x08601d83cdd34b5f7b8df63e7b9a16519d35473d0b89c317beed3d3d9424b253"),
				common.HexToHash("0x84e35c5d92171376cae5c86300822d729cd3a8479583bef09527027dba5f1126"),
				common.HexToHash("0x3c5cbbeb3834b7a5c1cba9aa5fee0c95ec3f17a33ec3d8047fff799187f5ae20"),
				common.HexToHash("0x40bbe913c226c34c9fbe4389dd728984257a816892b3cae3e43191dd291f0eb5"),
				common.HexToHash("0x14af5385bcbb1e4738bbae8106046e6e2fca42875aa5c000c582587742bcc748"),
				common.HexToHash("0x72f29656803c2f4be177b1b8dd2a5137892b080b022100fde4e96d93ef8c96ff"),
				common.HexToHash("0xd06f27061c734d7825b46865d00aa900e5cc3a3672080e527171e1171aa5038a"),
				common.HexToHash("0x28203985b5f2d87709171678169739f957d2745f4bfa5cc91e2b4bd9bf483b40"),
			},
		},
	}
}

func (s *ValidatorSuite) TearDownSubTest() {
	repo = nil
	validator = nil
}

func (s *ValidatorSuite) TestCreateClaimAndProofSuccess() {
	app := Application{
		Name: "dummy-application-name",
	}

	s.Run("FirstEpochNoOutputs", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil)

		claimHash, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[0])
		s.ErrorIs(nil, err)
		s.NotNil(claimHash)
		s.Equal(validator.pristineRootHash, *claimHash)
		repo.AssertExpectations(s.T())
	})

	s.Run("FirstEpochOneOutput", func() {
		output := Output{
			RawData: common.Hash{}.Bytes(),
		}

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{&output}, uint64(1), nil)

		claimHash, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[0])
		s.ErrorIs(nil, err)
		s.NotNil(claimHash)
		repo.AssertExpectations(s.T())
	})

	s.Run("SecondEpochNoOutputs", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyEpochs[0], nil).Once()

		claimHash, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.ErrorIs(nil, err)
		s.Equal(dummyEpochs[0].ClaimHash, claimHash)
		repo.AssertExpectations(s.T())
	})

	s.Run("SecondEpochTwoOutputs", func() {
		newOutput0 := Output{
			Index:   1,
			RawData: common.Hash{}.Bytes(),
		}
		newOutput1 := Output{
			Index:   2,
			RawData: common.Hash{}.Bytes(),
		}
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{&newOutput0, &newOutput1}, uint64(2), nil).Once()

		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyEpochs[0], nil).Once()

		repo.On("GetLastOutputBeforeBlock",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyOutputs[0], nil).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.ErrorIs(nil, err)
		repo.AssertExpectations(s.T())
	})
}

func (s *ValidatorSuite) TestCreateClaimAndProofFailures() {
	app := Application{
		Name: "dummy-application-name",
	}
	xerror := fmt.Errorf("Error")

	// Fail because ListOutputs failed
	s.Run("ListOutputsFailure", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), xerror).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[0])
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	// Fail because GetEpochByVirtualIndex failed
	s.Run("GetEpochByVirtualIndex", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyEpochs[0], xerror).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	// Fail because somehow the previous epoch does not have a claim hash yet.
	s.Run("InvalidPreviousEpoch", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		invalidEpoch := dummyEpochs[0]
		invalidEpoch.ClaimHash = nil
		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&invalidEpoch, nil).Once()

		repo.On("UpdateApplicationState",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	// Fail because GetLastOutputBeforeBlock failed
	s.Run("GetLastOutputBeforeBlockFailure", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{&dummyOutputs[0]}, uint64(1), nil).Once()

		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyEpochs[0], nil).Once()

		repo.On("GetLastOutputBeforeBlock",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&Output{}, xerror).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	// Fail because last output somehow does not have a hash
	s.Run("InvalidLastOutputiFailure", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{&dummyOutputs[0]}, uint64(1), nil).Once()

		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyEpochs[0], nil).Once()

		repo.On("GetLastOutputBeforeBlock",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&Output{}, nil).Once()

		repo.On("UpdateApplicationState",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	// Fail because last output and current output index are not sequential somehow
	s.Run("OutputIndexMismatchFailure", func() {
		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{{Index: 2}}, uint64(1), nil).Once()

		repo.On("GetEpochByVirtualIndex",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyEpochs[0], nil).Once()

		repo.On("GetLastOutputBeforeBlock",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(&dummyOutputs[0], nil).Once()

		repo.On("UpdateApplicationState",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		_, _, err := validator.createClaimAndProofs(nil, &app, &dummyEpochs[1])
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

}

func (s *ValidatorSuite) TestValidateApplicationSuccess() {
	app := Application{
		Name: "dummy-application-name",
	}
	s.Run("NoEpoch", func() {
		repo.On("ListEpochs", mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything).Return(([]*Epoch)(nil), uint64(0), nil).Once()

		err := validator.validateApplication(nil, &app)
		s.ErrorIs(nil, err)
		repo.AssertExpectations(s.T())
	})

	s.Run("FirstEpochNoOutputs", func() {
		input := Input{
			EpochApplicationID: app.ID,
			OutputsHash:        &validator.pristineRootHash,
		}

		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{&dummyEpochs[0]}, uint64(1), nil).Once()

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetLastInput",
			mock.Anything, app.IApplicationAddress.String(), dummyEpochs[0].Index,
		).Return(&input, nil).Once()

		repo.On("StoreClaimAndProofs",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		err := validator.validateApplication(nil, &app)
		s.ErrorIs(nil, err)
		repo.AssertExpectations(s.T())
	})
}

func (s *ValidatorSuite) TestValidateApplicationFailure() {
	app := Application{
		Name: "dummy-application-name",
	}
	xerror := fmt.Errorf("Error")

	s.Run("getProcessedEpochsFailure", func() {
		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{}, uint64(0), xerror).Once()

		err := validator.validateApplication(nil, &app)
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	s.Run("createClaimAndProofsFailure", func() {
		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{&dummyEpochs[0]}, uint64(1), nil).Once()

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), xerror).Once()

		err := validator.validateApplication(nil, &app)
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	s.Run("GetLastInputFailure", func() {
		input := Input{
			EpochApplicationID: app.ID,
			OutputsHash:        &validator.pristineRootHash,
		}

		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{&dummyEpochs[0]}, uint64(1), nil).Once()

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetLastInput",
			mock.Anything, app.IApplicationAddress.String(), dummyEpochs[0].Index,
		).Return(&input, xerror).Once()

		err := validator.validateApplication(nil, &app)
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	s.Run("InvalidInputFailure", func() {
		input := Input{
			EpochApplicationID: app.ID,
			OutputsHash:        nil, // <- this is invalid
		}

		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{&dummyEpochs[0]}, uint64(1), nil).Once()

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetLastInput",
			mock.Anything, app.IApplicationAddress.String(), dummyEpochs[0].Index,
		).Return(&input, nil).Once()

		repo.On("UpdateApplicationState",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		err := validator.validateApplication(nil, &app)
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	s.Run("ClaimMismatch", func() {
		invalidClaim := common.Hash{}
		input := Input{
			EpochApplicationID: app.ID,
			OutputsHash:        &invalidClaim,
		}

		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{&dummyEpochs[0]}, uint64(1), nil).Once()

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetLastInput",
			mock.Anything, app.IApplicationAddress.String(), dummyEpochs[0].Index,
		).Return(&input, nil).Once()

		repo.On("UpdateApplicationState",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return(nil).Once()

		err := validator.validateApplication(nil, &app)
		s.NotNil(err)
		repo.AssertExpectations(s.T())
	})

	s.Run("StoreClaimAndProofsFailure", func() {
		input := Input{
			EpochApplicationID: app.ID,
			OutputsHash:        &validator.pristineRootHash,
		}

		repo.On("ListEpochs",
			mock.Anything, app.IApplicationAddress.String(), mock.Anything, mock.Anything,
		).Return([]*Epoch{&dummyEpochs[0]}, uint64(1), nil).Once()

		repo.On("ListOutputs",
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).Return([]*Output{}, uint64(0), nil).Once()

		repo.On("GetLastInput",
			mock.Anything, app.IApplicationAddress.String(), dummyEpochs[0].Index,
		).Return(&input, nil).Once()

		repo.On("StoreClaimAndProofs",
			mock.Anything, mock.Anything, mock.Anything,
		).Return(xerror).Once()

		err := validator.validateApplication(nil, &app)
		s.ErrorIs(err, xerror)
		repo.AssertExpectations(s.T())
	})
}

type Mockrepo struct {
	mock.Mock
}

func newMockrepo() *Mockrepo {
	return new(Mockrepo)
}

func (m *Mockrepo) ListApplications(
	ctx context.Context,
	f repository.ApplicationFilter,
	pagination repository.Pagination,
) ([]*Application, uint64, error) {
	args := m.Called(ctx, f, pagination)
	return args.Get(0).([]*Application), args.Get(1).(uint64), args.Error(2)
}

func (m *Mockrepo) ListOutputs(
	ctx context.Context,
	nameOrAddress string,
	f repository.OutputFilter,
	p repository.Pagination,
) ([]*Output, uint64, error) {
	args := m.Called(ctx, nameOrAddress, f, p)
	return args.Get(0).([]*Output), args.Get(1).(uint64), args.Error(2)
}

func (m *Mockrepo) GetLastOutputBeforeBlock(
	ctx context.Context,
	nameOrAddress string,
	block uint64,
) (*Output, error) {
	args := m.Called(ctx, nameOrAddress, block)
	return args.Get(0).(*Output), args.Error(1)
}

func (m *Mockrepo) ListEpochs(
	ctx context.Context,
	nameOrAddress string,
	f repository.EpochFilter,
	p repository.Pagination,
) ([]*Epoch, uint64, error) {
	args := m.Called(ctx, nameOrAddress, f, p)
	return args.Get(0).([]*Epoch), args.Get(1).(uint64), args.Error(2)
}

func (m *Mockrepo) GetLastInput(ctx context.Context, appAddress string, epochIndex uint64) (*Input, error) {
	args := m.Called(ctx, appAddress, epochIndex)
	if input, ok := args.Get(0).(*Input); ok {
		return input, args.Error(1)
	}
	return args.Get(0).(*Input), args.Error(1)
}

func (m *Mockrepo) GetEpochByVirtualIndex(ctx context.Context, nameOrAddress string, index uint64) (*Epoch, error) {
	args := m.Called(ctx, nameOrAddress, index)
	if epoch, ok := args.Get(0).(*Epoch); ok {
		return epoch, args.Error(1)
	}
	return args.Get(0).(*Epoch), args.Error(1)
}

func (m *Mockrepo) StoreClaimAndProofs(ctx context.Context, epoch *Epoch, outputs []*Output) error {
	args := m.Called(ctx, epoch, outputs)
	return args.Error(0)
}

func (m *Mockrepo) UpdateApplicationState(ctx context.Context, appID int64, state ApplicationState, reason *string) error {
	args := m.Called(ctx, appID, state, reason)
	return args.Error(0)
}
