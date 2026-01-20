package block

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTxIndexInsertDeduplicates(t *testing.T) {
	idx := NewTxIndex()

	idx.Insert("signer1", "lane-a", 2, "first-signer", 10)
	idx.Insert("signer1", "lane-a", 3, "first-signer", 10)

	entries := idx.index["signer1"]
	require.Len(t, entries, 1)
	require.Equal(t, 3, entries[0].LanePriority)
}

func TestTxIndexRemoveFiltersAllMatches(t *testing.T) {
	idx := NewTxIndex()
	idx.index["signer1"] = []*LaneTxEntry{
		{
			LaneName:              "lane-a",
			LanePriority:          1,
			FirstSignerIdentifier: "first-signer",
			FirstSignerNonce:      10,
		},
		{
			LaneName:              "lane-a",
			LanePriority:          2,
			FirstSignerIdentifier: "first-signer",
			FirstSignerNonce:      10,
		},
		{
			LaneName:              "lane-b",
			LanePriority:          3,
			FirstSignerIdentifier: "first-signer",
			FirstSignerNonce:      11,
		},
	}

	idx.Remove("signer1", "lane-a", "first-signer", 10)

	entries := idx.index["signer1"]
	require.Len(t, entries, 1)
	require.Equal(t, "lane-b", entries[0].LaneName)
	require.Equal(t, uint64(11), entries[0].FirstSignerNonce)
}
