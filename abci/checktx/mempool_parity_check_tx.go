package checktx

import (
	"errors"
	"fmt"
	"runtime/debug"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	cmtabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/skip-mev/block-sdk/v2/block"
)

// MempoolParityCheckTx is a CheckTx function that evicts txs that are not in the app-side mempool
// on ReCheckTx. This handler is used to enforce parity in the app-side / comet mempools.
type MempoolParityCheckTx struct {
	// logger
	logger log.Logger

	// app side mempool interface
	mempl block.Mempool

	// tx-decoder
	txDecoder sdk.TxDecoder

	// checkTxHandler to wrap
	checkTxHandler CheckTx

	// baseApp is utilized to retrieve the latest committed state and to call
	// baseapp's CheckTx method.
	baseApp BaseApp
}

// NewMempoolParityCheckTx returns a new MempoolParityCheckTx handler.
func NewMempoolParityCheckTx(
	logger log.Logger,
	mempl block.Mempool,
	txDecoder sdk.TxDecoder,
	checkTxHandler CheckTx,
	baseApp BaseApp,
) MempoolParityCheckTx {
	return MempoolParityCheckTx{
		logger:         logger,
		mempl:          mempl,
		txDecoder:      txDecoder,
		checkTxHandler: checkTxHandler,
		baseApp:        baseApp,
	}
}

// CheckTx returns a CheckTx handler that wraps a given CheckTx handler and evicts txs that are not
// in the app-side mempool on ReCheckTx.
func (m MempoolParityCheckTx) CheckTx() CheckTx {
	return func(req *cmtabci.RequestCheckTx) (checkRes *cmtabci.ResponseCheckTx, checkErr error) {
		defer func(checkRes **cmtabci.ResponseCheckTx, checkErr *error) {
			if r := recover(); r != nil {
				m.logger.Error("panic in CheckTx (MempoolParityCheckTx)", "panic", r)
				debug.PrintStack()

				if err, ok := r.(error); ok {
					*checkRes = sdkerrors.ResponseCheckTxWithEvents(
						err,
						0,
						0,
						nil,
						true,
					)
				} else {
					*checkRes = sdkerrors.ResponseCheckTxWithEvents(
						fmt.Errorf("panic in CheckTx (MempoolParityCheckTx): %v", r),
						0,
						0,
						nil,
						true,
					)
				}
			}
		}(&checkRes, &checkErr)

		// decode tx
		tx, err := m.txDecoder(req.Tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(
				fmt.Errorf("failed to decode tx: %w", err),
				0,
				0,
				nil,
				false,
			), nil
		}

		isReCheck := req.Type == cmtabci.CheckTxType_Recheck
		txInMempool := m.mempl.Contains(tx)

		// if the mode is ReCheck and the app's mempool does not contain the given tx, we fail
		// immediately, to purge the tx from the comet mempool.
		if isReCheck && !txInMempool {
			m.logger.Debug(
				"tx from comet mempool not found in app-side mempool",
				"tx", tx,
			)

			return sdkerrors.ResponseCheckTxWithEvents(
				fmt.Errorf("tx from comet mempool not found in app-side mempool"),
				0,
				0,
				nil,
				false,
			), nil
		}

		// prepare cleanup closure to remove tx if marked
		removeTx := false
		defer func() {
			if removeTx {
				// remove the tx
				if err := m.mempl.Remove(tx); err != nil {
					m.logger.Debug(
						"failed to remove tx from app-side mempool when purging for re-check failure",
						"removal-err", err,
					)
				}
			}
		}()

		// run the checkTxHandler
		res, checkTxError := m.checkTxHandler(req)

		// can fail for a variety of reasons, check the results of the checkTxHandler
		// need to remove from mempool if re-check fails and tx is in mempool.
		if isInvalidCheckTxExecution(res, checkTxError) {
			if isReCheck && txInMempool {
				removeTx = true
			}

			m.logger.Debug("failed base checkTx", "err", checkTxError, "res", fmt.Sprintf("%+v", res))
			return res, checkTxError
		}

		sdkCtx := m.GetContextForTx(req)
		lane, err := m.matchLane(sdkCtx, tx)
		if err != nil {
			if isReCheck && txInMempool {
				removeTx = true
			}

			m.logger.Debug("failed to match lane", "lane", lane, "err", err)
			return sdkerrors.ResponseCheckTxWithEvents(
				err,
				0,
				0,
				nil,
				false,
			), nil
		}

		consensusParams := sdkCtx.ConsensusParams()
		blockMaxBytes := consensusParams.GetBlock().GetMaxBytes()

		var laneSizeBytes int64
		if laneBlockSpace := lane.GetMaxBlockSpace(); laneBlockSpace.IsZero() {
			// for default lane, we use the block max bytes
			laneSizeBytes = blockMaxBytes
		} else {
			// for other lanes, we use the lane's max block space
			laneSizeBytes = laneBlockSpace.MulInt64(blockMaxBytes).TruncateInt64()
		}

		txSize := int64(len(req.Tx))
		if txSize > laneSizeBytes {
			if isReCheck && txInMempool {
				removeTx = true
			}

			m.logger.Debug(
				"tx size exceeds max lane size bytes",
				"tx", tx,
				"tx size", txSize,
				"max bytes", laneSizeBytes,
			)

			return sdkerrors.ResponseCheckTxWithEvents(
				errorsmod.Wrapf(sdkerrors.ErrTxTooLarge, "tx size exceeds max bytes for lane %s", lane.Name()),
				0,
				0,
				nil,
				false,
			), nil
		}

		return res, nil
	}
}

// matchLane returns a Lane if the given tx matches the Lane.
func (m MempoolParityCheckTx) matchLane(ctx sdk.Context, tx sdk.Tx) (block.Lane, error) {
	var lane block.Lane
	// find corresponding lane for this tx
	for _, l := range m.mempl.Registry() {
		if l.Match(ctx, tx) {
			lane = l
			break
		}
	}

	if lane == nil {
		m.logger.Debug(
			"failed match tx to lane",
			"tx", tx,
		)

		return nil, fmt.Errorf("failed match tx to lane")
	}

	return lane, nil
}

func isInvalidCheckTxExecution(resp *cmtabci.ResponseCheckTx, checkTxErr error) bool {
	return resp == nil ||
		// we ignore ErrWrongSequence cause it means we failed optimistic recheck
		((resp.Code != 0 || checkTxErr != nil) && !errors.Is(checkTxErr, sdkerrors.ErrWrongSequence))
}

// GetContextForTx is returns the latest committed state and sets the context given
// the checkTx request.
func (m MempoolParityCheckTx) GetContextForTx(req *cmtabci.RequestCheckTx) sdk.Context {
	ctx, _ := m.baseApp.GetContextForCheckTx(req.Tx).CacheContext()
	return ctx
}
