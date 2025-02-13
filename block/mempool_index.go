package block

import (
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LaneTxEntry holds a reference to a transaction that is already in the mempool,
// along with the lane it belongs to and its priority.
type LaneTxEntry struct {
	LaneName     string // e.g. "mev", "default", "custom"
	LanePriority int    // a numeric priority (lower number = higher priority)
	Tx           sdk.Tx
}

// TxIndex maps signer addresses (as string) to a slice of entries.
type TxIndex struct {
	mu    sync.RWMutex
	index map[string][]*LaneTxEntry
}

// NewTxIndex creates a new index.
func NewTxIndex() *TxIndex {
	return &TxIndex{
		index: make(map[string][]*LaneTxEntry),
	}
}

// Insert adds a transaction to the index.
func (idx *TxIndex) Insert(signer string, laneName string, lanePriority int, tx sdk.Tx) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	entry := &LaneTxEntry{
		LaneName:     laneName,
		LanePriority: lanePriority,
		Tx:           tx,
	}
	idx.index[signer] = append(idx.index[signer], entry)
}

// Remove deletes a transaction from the index.
func (idx *TxIndex) Remove(signer string, laneName string, tx sdk.Tx) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	entries, ok := idx.index[signer]
	if !ok {
		return
	}

	for i, entry := range entries {
		if entry.LaneName == laneName && entry.Tx == tx {
			entries = append(entries[:i], entries[i+1:]...)
			break
		}
	}

	if len(entries) == 0 {
		delete(idx.index, signer)
	} else {
		idx.index[signer] = entries
	}
}

// DoesExistInLowerPriorityLane checks whether there exists a transaction from the signer
// in any lane with a priority number greater than currentPriority (i.e. lower priority).
func (idx *TxIndex) DoesExistInLowerPriorityLane(signer string, currentPriority int) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entries, ok := idx.index[signer]
	if !ok {
		return false
	}

	for _, entry := range entries {
		if entry.LanePriority > currentPriority {
			return true
		}
	}
	return false
}
