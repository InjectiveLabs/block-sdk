package base_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	signerextraction "github.com/skip-mev/block-sdk/v2/adapters/signer_extraction_adapter"
	"github.com/skip-mev/block-sdk/v2/block/base"
)

type testFeeTx struct {
	gas uint64
}

func (t testFeeTx) GetMsgs() []sdk.Msg {
	return nil
}

func (t testFeeTx) GetMsgsV2() ([]protov2.Message, error) {
	return nil, nil
}

func (t testFeeTx) GetGas() uint64 {
	return t.gas
}

func (t testFeeTx) GetFee() sdk.Coins {
	return nil
}

func (t testFeeTx) FeePayer() []byte {
	return []byte("payer")
}

func (t testFeeTx) FeeGranter() []byte {
	return nil
}

type staticSignerExtractor struct{}

func (staticSignerExtractor) GetSigners(sdk.Tx) ([]signerextraction.SignerData, error) {
	return []signerextraction.SignerData{
		{
			Signer:   sdk.AccAddress("signer"),
			Sequence: 1,
		},
	}, nil
}

func TestGetTxInfoUsesCometBFTProtoTxSize(t *testing.T) {
	txBytes := bytes.Repeat([]byte{0x1}, 256)
	lane, err := base.NewBaseLane(
		base.LaneConfig{
			Logger:          log.NewNopLogger(),
			TxEncoder:       func(sdk.Tx) ([]byte, error) { return txBytes, nil },
			TxDecoder:       func([]byte) (sdk.Tx, error) { return testFeeTx{}, nil },
			SignerExtractor: staticSignerExtractor{},
			MaxBlockSpace:   math.LegacyZeroDec(),
		},
		"default",
	)
	require.NoError(t, err)

	txInfo, err := lane.GetTxInfo(sdk.Context{}, testFeeTx{gas: 10})
	require.NoError(t, err)

	expectedSize := cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{txBytes})
	require.Equal(t, expectedSize, txInfo.Size)
	require.Greater(t, txInfo.Size, int64(len(txBytes)))
	require.NoError(t, cmttypes.Txs{cmttypes.Tx(txBytes)}.Validate(txInfo.Size))
	require.ErrorContains(t, cmttypes.Txs{cmttypes.Tx(txBytes)}.Validate(txInfo.Size-1), "transaction data size exceeds maximum")
}
