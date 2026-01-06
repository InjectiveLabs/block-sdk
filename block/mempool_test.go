package block_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	signerextraction "github.com/skip-mev/block-sdk/v2/adapters/signer_extraction_adapter"
	"github.com/skip-mev/block-sdk/v2/block"
	"github.com/skip-mev/block-sdk/v2/block/base"
)

func matchHandlerAlwaysTrue(ctx sdk.Context, tx sdk.Tx) bool {
	return true
}

type EmptyTx struct{}

func (e EmptyTx) GetMsgs() []sdk.Msg {
	return []sdk.Msg{}
}

func (e EmptyTx) GetMsgsV2() ([]protov2.Message, error) {
	return []protov2.Message{}, nil
}

func NoopEncoder(tx sdk.Tx) ([]byte, error) {
	return []byte{}, nil
}

func NoopDecoder(txBytes []byte) (sdk.Tx, error) {
	return EmptyTx{}, nil
}

type NoopAdapter struct{}

func (n NoopAdapter) GetSigners(tx sdk.Tx) ([]signerextraction.SignerData, error) {
	return []signerextraction.SignerData{
		{
			Signer:   []byte("noop"),
			Sequence: 0,
		},
	}, nil
}

// TestMempoolLaneWithMaxTxs tests that a laned mempool with a lane that has MaxTxs
// set to 1 only allows one transaction to be inserted into that lane, but allows
// additional transactions to be inserted into other lanes.
func TestMempoolLaneWithMaxTxs(t *testing.T) {
	priorityLane, err := base.NewBaseLane(
		base.LaneConfig{
			Logger:          log.NewNopLogger(),
			TxEncoder:       NoopEncoder,
			TxDecoder:       NoopDecoder,
			SignerExtractor: NoopAdapter{},
			MaxTxs:          1,
			MaxBlockSpace:   sdkmath.LegacyMustNewDecFromStr("0.5"),
		},
		"priority",
	)
	require.NoError(t, err)
	priorityLane = priorityLane.WithOptions(base.WithMatchHandler(
		matchHandlerAlwaysTrue,
	))

	defaultLane, err := base.NewBaseLane(
		base.LaneConfig{
			Logger:          log.NewNopLogger(),
			TxEncoder:       NoopEncoder,
			TxDecoder:       NoopDecoder,
			SignerExtractor: NoopAdapter{},
			MaxTxs:          0,
			MaxBlockSpace:   sdkmath.LegacyNewDec(0),
		},
		"default",
	)
	require.NoError(t, err)
	defaultLane = defaultLane.WithOptions(base.WithMatchHandler(
		matchHandlerAlwaysTrue,
	))

	mempool, err := block.NewLanedMempool(
		log.NewNopLogger(),
		[]block.Lane{
			priorityLane,
			defaultLane,
		},
	)
	require.NoError(t, err)

	ctx := sdk.Context{}

	err = mempool.Insert(ctx, EmptyTx{})
	require.NoError(t, err)

	err = mempool.Insert(ctx, EmptyTx{})
	require.NoError(t, err)

	require.Equal(t, 2, mempool.CountTx())
}
