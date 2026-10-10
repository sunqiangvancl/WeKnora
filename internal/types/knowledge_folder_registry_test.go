package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeRegisteredFolders(t *testing.T) {
	counts := []*KnowledgeFolderCount{
		{FolderPath: "docs", Count: 3},
		{FolderPath: "docs/spec", Count: 2},
		{FolderPath: "", Count: 7},
	}

	t.Run("registered path without a projection row becomes a zero-count node", func(t *testing.T) {
		merged := MergeRegisteredFolders(counts, []string{"inbox"})
		tree := BuildKnowledgeFolderTree(merged)
		require.Len(t, tree.Folders, 2)
		var inbox *KnowledgeFolderNode
		for _, node := range tree.Folders {
			if node.Path == "inbox" {
				inbox = node
			}
		}
		require.NotNil(t, inbox, "the registered folder must appear in the tree")
		assert.Equal(t, int64(0), inbox.DocumentCount)
		assert.Equal(t, int64(0), inbox.TotalCount)
	})

	t.Run("a registered path that already has documents is not duplicated", func(t *testing.T) {
		merged := MergeRegisteredFolders(counts, []string{"docs"})
		var rows int
		var total int64
		for _, row := range merged {
			if row.FolderPath == "docs" {
				rows++
				total += row.Count
			}
		}
		assert.Equal(t, 1, rows, "one path must contribute exactly one count row")
		assert.Equal(t, int64(3), total, "and must not change the document count")
	})

	t.Run("registering a deep path materializes its ancestors", func(t *testing.T) {
		tree := BuildKnowledgeFolderTree(MergeRegisteredFolders(
			[]*KnowledgeFolderCount{}, []string{"客户/大北农/2026Q4"}))
		require.Len(t, tree.Folders, 1)
		customer := tree.Folders[0]
		assert.Equal(t, "客户", customer.Path)
		assert.Equal(t, int64(0), customer.DocumentCount)
		assert.Equal(t, int64(0), customer.TotalCount,
			"an empty subtree must not inflate counts anywhere")
		require.Len(t, customer.Children, 1)
		require.Len(t, customer.Children[0].Children, 1)
		assert.Equal(t, "客户/大北农/2026Q4", customer.Children[0].Children[0].Path)
	})

	t.Run("empty and duplicate registrations are inert", func(t *testing.T) {
		assert.Equal(t, counts, MergeRegisteredFolders(counts, nil))
		assert.Equal(t, counts, MergeRegisteredFolders(counts, []string{}))

		merged := MergeRegisteredFolders(counts, []string{"dup", "dup", "  dup  ", "", "/"})
		var dup int
		for _, row := range merged {
			if row.FolderPath == "dup" {
				dup++
			}
		}
		assert.Equal(t, 1, dup, "one registered name yields one node regardless of repeats")
	})

	t.Run("totals only count documents", func(t *testing.T) {
		tree := BuildKnowledgeFolderTree(MergeRegisteredFolders(counts, []string{"a", "b", "c"}))
		assert.Equal(t, int64(12), tree.TotalDocumentCount)
		assert.Equal(t, int64(7), tree.RootDocumentCount)
	})
}
