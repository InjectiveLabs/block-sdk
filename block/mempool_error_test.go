package block_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	signerextraction "github.com/skip-mev/block-sdk/v2/adapters/signer_extraction_adapter"
	"github.com/skip-mev/block-sdk/v2/block"
	"github.com/skip-mev/block-sdk/v2/block/proposals"
	"github.com/skip-mev/block-sdk/v2/block/utils"
)

type errorSignerAdapter struct{}

func (errorSignerAdapter) GetSigners(sdk.Tx) ([]signerextraction.SignerData, error) {
	return nil, fmt.Errorf("boom")
}

type emptySignerAdapter struct{}

func (emptySignerAdapter) GetSigners(sdk.Tx) ([]signerextraction.SignerData, error) {
	return []signerextraction.SignerData{}, nil
}

type testLane struct {
	name            string
	match           bool
	contains        bool
	insertErr       error
	removeErr       error
	maxBlockSpace   math.LegacyDec
	signerExtractor signerextraction.Adapter
	count           int
}

func (t *testLane) PrepareLane(sdk.Context, proposals.Proposal, block.PrepareLanesHandler) (proposals.Proposal, error) {
	return proposals.Proposal{}, nil
}

func (t *testLane) ProcessLane(sdk.Context, proposals.Proposal, []sdk.Tx, block.ProcessLanesHandler) (proposals.Proposal, error) {
	return proposals.Proposal{}, nil
}

func (t *testLane) GetMaxBlockSpace() math.LegacyDec {
	return t.maxBlockSpace
}

func (t *testLane) SetMaxBlockSpace(space math.LegacyDec) {
	t.maxBlockSpace = space
}

func (t *testLane) Logger() log.Logger {
	return log.NewNopLogger()
}

func (t *testLane) Name() string {
	return t.name
}

func (t *testLane) GetTxInfo(sdk.Context, sdk.Tx) (utils.TxWithInfo, error) {
	return utils.TxWithInfo{}, nil
}

func (t *testLane) SetAnteHandler(sdk.AnteHandler) {}

func (t *testLane) Match(sdk.Context, sdk.Tx) bool {
	return t.match
}

func (t *testLane) Contains(sdk.Tx) bool {
	return t.contains
}

func (t *testLane) CountTx() int {
	return t.count
}

func (t *testLane) Insert(context.Context, sdk.Tx) error {
	if t.insertErr != nil {
		return t.insertErr
	}
	t.count++
	t.contains = true
	return nil
}

func (t *testLane) Remove(sdk.Tx) error {
	if t.removeErr != nil {
		return t.removeErr
	}
	if t.count > 0 {
		t.count--
	}
	t.contains = false
	return nil
}

func (t *testLane) Select(context.Context, [][]byte) sdkmempool.Iterator {
	return nil
}

func (t *testLane) Compare(sdk.Context, sdk.Tx, sdk.Tx) (int, error) {
	return 0, nil
}

func (t *testLane) Priority(sdk.Context, sdk.Tx) any {
	return 0
}

func (t *testLane) SignerExtractor() signerextraction.Adapter {
	return t.signerExtractor
}

func TestLanedMempoolInsertReturnsSignerError(t *testing.T) {
	lane := &testLane{
		name:            "test",
		match:           true,
		signerExtractor: errorSignerAdapter{},
	}

	mempool, err := block.NewLanedMempool(
		log.NewNopLogger(),
		[]block.Lane{lane},
	)
	require.NoError(t, err)

	err = mempool.Insert(context.Background(), EmptyTx{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to extract signers upon insertion")
}

func TestLanedMempoolRemoveReturnsSignerError(t *testing.T) {
	lane := &testLane{
		name:            "test",
		contains:        true,
		signerExtractor: errorSignerAdapter{},
	}

	mempool, err := block.NewLanedMempool(
		log.NewNopLogger(),
		[]block.Lane{lane},
	)
	require.NoError(t, err)

	err = mempool.Remove(EmptyTx{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to extract signers upon removal")
}

func TestLanedMempoolInsertRejectsEmptySigners(t *testing.T) {
	lane := &testLane{
		name:            "test",
		match:           true,
		signerExtractor: emptySignerAdapter{},
	}

	mempool, err := block.NewLanedMempool(
		log.NewNopLogger(),
		[]block.Lane{lane},
	)
	require.NoError(t, err)

	err = mempool.Insert(context.Background(), EmptyTx{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no signers found for tx during insertion")
}

func TestLanedMempoolRemoveRejectsEmptySigners(t *testing.T) {
	lane := &testLane{
		name:            "test",
		contains:        true,
		signerExtractor: emptySignerAdapter{},
	}

	mempool, err := block.NewLanedMempool(
		log.NewNopLogger(),
		[]block.Lane{lane},
	)
	require.NoError(t, err)

	err = mempool.Remove(EmptyTx{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no signers found for tx during removal")
}
