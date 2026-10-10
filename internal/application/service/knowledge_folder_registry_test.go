package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// folderRegistryStub stands in for the knowledge repository's folder surface.
// It embeds the interface so unrelated methods stay unavailable by construction,
// exactly like the existing folder move stub.
type folderRegistryStub struct {
	interfaces.KnowledgeRepository

	paths       []string // registry contents
	documents   int64    // CountFolderSubtreeDocuments answer
	registered  []string // ListRegisteredChildFolders answer
	counts      []*types.KnowledgeFolderCount
	created     *types.KnowledgeFolder
	deletedPath string
	notFound    bool
}

func (r *folderRegistryStub) ListRegisteredFolderPaths(context.Context, uint64, string) ([]string, error) {
	return r.paths, nil
}

func (r *folderRegistryStub) FindRegisteredFolder(
	_ context.Context, _ uint64, _, path string,
) (*types.KnowledgeFolder, error) {
	for _, live := range r.paths {
		if live == path {
			return types.NewKnowledgeFolder(1, "kb-1", path), nil
		}
	}
	return nil, nil
}

func (r *folderRegistryStub) CreateRegisteredFolder(_ context.Context, f *types.KnowledgeFolder) error {
	r.created = f
	return nil
}

func (r *folderRegistryStub) DeleteRegisteredFolder(_ context.Context, _ uint64, _, path string) error {
	r.deletedPath = path
	return nil
}

func (r *folderRegistryStub) CountFolderSubtreeDocuments(context.Context, uint64, string, string) (int64, error) {
	return r.documents, nil
}

func (r *folderRegistryStub) ListRegisteredChildFolders(context.Context, uint64, string, string) ([]string, error) {
	return r.registered, nil
}

func (r *folderRegistryStub) ListKnowledgeFolderCounts(context.Context, uint64, string) ([]*types.KnowledgeFolderCount, error) {
	return r.counts, nil
}

func newFolderService(repo *folderRegistryStub) *knowledgeService {
	return &knowledgeService{
		repo:      repo,
		kbService: &writeKBLookup{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 1}},
	}
}

func TestCreateKnowledgeFolder(t *testing.T) {
	ctx := folderMoveContext()

	t.Run("registers the normalized path", func(t *testing.T) {
		repo := &folderRegistryStub{}
		folder, err := newFolderService(repo).CreateKnowledgeFolder(ctx, "kb-1", "/客户//大北农./")
		require.NoError(t, err)
		require.NotNil(t, repo.created)
		assert.Equal(t, "客户/大北农", repo.created.Path)
		assert.NotEmpty(t, folder.ID)
	})

	t.Run("refuses the knowledge base root", func(t *testing.T) {
		repo := &folderRegistryStub{}
		_, err := newFolderService(repo).CreateKnowledgeFolder(ctx, "kb-1", "  /  ")
		require.Error(t, err)
		assert.Nil(t, repo.created, "nothing may be written for a rejected request")
	})

	t.Run("refuses an already registered path", func(t *testing.T) {
		repo := &folderRegistryStub{paths: []string{"inbox"}}
		_, err := newFolderService(repo).CreateKnowledgeFolder(ctx, "kb-1", "inbox")
		require.Error(t, err)
		assert.Nil(t, repo.created)
	})

	t.Run("refuses a path the projection already renders", func(t *testing.T) {
		repo := &folderRegistryStub{documents: 4}
		_, err := newFolderService(repo).CreateKnowledgeFolder(ctx, "kb-1", "docs/spec")
		require.Error(t, err)
		assert.Nil(t, repo.created)
	})
}

func TestDeleteKnowledgeFolder(t *testing.T) {
	ctx := folderMoveContext()

	t.Run("a projected folder is not deletable and says why", func(t *testing.T) {
		repo := &folderRegistryStub{paths: []string{"registered"}}
		err := newFolderService(repo).DeleteKnowledgeFolder(ctx, "kb-1", "projected-only")
		require.Error(t, err)
		assert.Empty(t, repo.deletedPath)
	})

	t.Run("documents anywhere below block the delete", func(t *testing.T) {
		repo := &folderRegistryStub{paths: []string{"客户/大北农"}, documents: 11}
		err := newFolderService(repo).DeleteKnowledgeFolder(ctx, "kb-1", "客户/大北农")
		require.Error(t, err)
		assert.Empty(t, repo.deletedPath)
	})

	t.Run("an empty child folder blocks the parent", func(t *testing.T) {
		// This is the case a document count cannot see: no files, but a
		// registered sub-folder still exists, so the parent is not empty.
		repo := &folderRegistryStub{paths: []string{"客户", "客户/大北农"}, registered: []string{"客户/大北农"}}
		err := newFolderService(repo).DeleteKnowledgeFolder(ctx, "kb-1", "客户")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "大北农", "the refusal must name what is left inside")
		assert.Empty(t, repo.deletedPath)
	})

	t.Run("a genuinely empty folder is unregistered", func(t *testing.T) {
		repo := &folderRegistryStub{paths: []string{"客户/大北农"}}
		require.NoError(t, newFolderService(repo).DeleteKnowledgeFolder(ctx, "kb-1", "客户/大北农"))
		assert.Equal(t, "客户/大北农", repo.deletedPath)
	})
}

func TestListKnowledgeFolderTreeUnionsRegistry(t *testing.T) {
	ctx := folderMoveContext()
	repo := &folderRegistryStub{
		paths:  []string{"客户/空客户"},
		counts: []*types.KnowledgeFolderCount{{FolderPath: "客户/大北农", Count: 11}},
	}
	tree, err := newFolderService(repo).ListKnowledgeFolderTree(ctx, "kb-1")
	require.NoError(t, err)
	require.Len(t, tree.Folders, 1)
	client := tree.Folders[0]
	assert.Equal(t, "客户", client.Path)
	assert.Equal(t, int64(11), client.TotalCount)
	require.Len(t, client.Children, 2, "the registered empty folder must sit beside the projected one")
	assert.Equal(t, "客户/大北农", client.Children[0].Path)
	assert.Equal(t, "客户/空客户", client.Children[1].Path)
	assert.Equal(t, int64(0), client.Children[1].DocumentCount)
	assert.Equal(t, int64(11), tree.TotalDocumentCount, "empty folders add no documents to the total")
}
