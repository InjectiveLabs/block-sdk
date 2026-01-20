package block

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTxIndexDoesExistInLowerPriorityLane(t *testing.T) {
	idx := NewTxIndex()
	require.False(t, idx.DoesExistInLowerPriorityLane("signer1", 0))

	idx.Insert("signer1", "lane-a", 1, "first-signer", 10)
	require.False(t, idx.DoesExistInLowerPriorityLane("signer1", 2))
	require.True(t, idx.DoesExistInLowerPriorityLane("signer1", 0))
}

func TestTxIndexRemoveMissingSignerNoop(t *testing.T) {
	idx := NewTxIndex()
	idx.Remove("missing", "lane-a", "first-signer", 10)

	require.Empty(t, idx.index)
}

func TestTxIndexRemoveDeletesEmptySignerEntry(t *testing.T) {
	idx := NewTxIndex()
	idx.Insert("signer1", "lane-a", 1, "first-signer", 10)

	idx.Remove("signer1", "lane-a", "first-signer", 10)

	_, ok := idx.index["signer1"]
	require.False(t, ok)
}
