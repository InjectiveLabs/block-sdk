package block_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	signerextraction "github.com/skip-mev/block-sdk/v2/adapters/signer_extraction_adapter"
	"github.com/skip-mev/block-sdk/v2/block"
)

type staticSignerAdapter struct {
	signers []signerextraction.SignerData
}

func (s staticSignerAdapter) GetSigners(sdk.Tx) ([]signerextraction.SignerData, error) {
	return s.signers, nil
}

func TestLanedMempoolInsertTricklesOnCapacity(t *testing.T) {
	signer := signerextraction.SignerData{Signer: sdk.AccAddress("addr1"), Sequence: 1}
	adapter := staticSignerAdapter{signers: []signerextraction.SignerData{signer}}

	priorityLane := &testLane{
		name:            "priority",
		match:           true,
		insertErr:       sdkmempool.ErrMempoolTxMaxCapacity,
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyMustNewDecFromStr("0.5"),
	}
	defaultLane := &testLane{
		name:            "default",
		match:           true,
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyZeroDec(),
	}

	mempool, err := block.NewLanedMempool(log.NewNopLogger(), []block.Lane{priorityLane, defaultLane})
	require.NoError(t, err)

	err = mempool.Insert(sdk.WrapSDKContext(sdk.Context{}), EmptyTx{})
	require.NoError(t, err)
	require.Equal(t, 0, priorityLane.CountTx())
	require.Equal(t, 1, defaultLane.CountTx())
}

func TestLanedMempoolInsertSkipsHigherLaneWhenLowerExists(t *testing.T) {
	signer := signerextraction.SignerData{Signer: sdk.AccAddress("addr1"), Sequence: 1}
	adapter := staticSignerAdapter{signers: []signerextraction.SignerData{signer}}

	priorityLane := &testLane{
		name:            "priority",
		match:           false,
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyMustNewDecFromStr("0.5"),
	}
	defaultLane := &testLane{
		name:            "default",
		match:           true,
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyZeroDec(),
	}

	mempool, err := block.NewLanedMempool(log.NewNopLogger(), []block.Lane{priorityLane, defaultLane})
	require.NoError(t, err)

	err = mempool.Insert(sdk.WrapSDKContext(sdk.Context{}), EmptyTx{})
	require.NoError(t, err)
	require.Equal(t, 1, defaultLane.CountTx())

	priorityLane.match = true
	err = mempool.Insert(sdk.WrapSDKContext(sdk.Context{}), EmptyTx{})
	require.NoError(t, err)
	require.Equal(t, 0, priorityLane.CountTx())
	require.Equal(t, 2, defaultLane.CountTx())
}

func TestLanedMempoolInsertPropagatesLaneError(t *testing.T) {
	signer := signerextraction.SignerData{Signer: sdk.AccAddress("addr1"), Sequence: 1}
	adapter := staticSignerAdapter{signers: []signerextraction.SignerData{signer}}

	priorityLane := &testLane{
		name:            "priority",
		match:           true,
		insertErr:       fmt.Errorf("insert failed"),
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyOneDec(),
	}

	mempool, err := block.NewLanedMempool(log.NewNopLogger(), []block.Lane{priorityLane})
	require.NoError(t, err)

	err = mempool.Insert(sdk.WrapSDKContext(sdk.Context{}), EmptyTx{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insert failed")
}

func TestLanedMempoolRemovePropagatesLaneError(t *testing.T) {
	signer := signerextraction.SignerData{Signer: sdk.AccAddress("addr1"), Sequence: 1}
	adapter := staticSignerAdapter{signers: []signerextraction.SignerData{signer}}

	priorityLane := &testLane{
		name:            "priority",
		contains:        true,
		removeErr:       fmt.Errorf("remove failed"),
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyOneDec(),
	}

	mempool, err := block.NewLanedMempool(log.NewNopLogger(), []block.Lane{priorityLane})
	require.NoError(t, err)

	err = mempool.Remove(EmptyTx{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "remove failed")
}

func TestLanedMempoolRemoveSuccess(t *testing.T) {
	signer := signerextraction.SignerData{Signer: sdk.AccAddress("addr1"), Sequence: 1}
	adapter := staticSignerAdapter{signers: []signerextraction.SignerData{signer}}

	priorityLane := &testLane{
		name:            "priority",
		match:           false,
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyMustNewDecFromStr("0.5"),
	}
	defaultLane := &testLane{
		name:            "default",
		match:           true,
		signerExtractor: adapter,
		maxBlockSpace:   math.LegacyZeroDec(),
	}

	mempool, err := block.NewLanedMempool(log.NewNopLogger(), []block.Lane{priorityLane, defaultLane})
	require.NoError(t, err)

	err = mempool.Insert(sdk.WrapSDKContext(sdk.Context{}), EmptyTx{})
	require.NoError(t, err)
	require.Equal(t, 1, defaultLane.CountTx())

	err = mempool.Remove(EmptyTx{})
	require.NoError(t, err)
	require.Equal(t, 0, defaultLane.CountTx())
}
