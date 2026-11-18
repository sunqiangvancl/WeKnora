package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBySourceRef_SQLiteMatchesExactAndTitledRefs(t *testing.T) {
	db := setupWikiPagesTestDB(t)
	repo := NewWikiPageRepository(db)
	ctx := context.Background()

	exact := makeWikiPage("kb-src", "summary/exact", types.WikiPageTypeSummary, types.WikiPageStatusPublished)
	exact.SourceRefs = types.StringArray{"knowledge-1"}
	titled := makeWikiPage("kb-src", "entity/titled", types.WikiPageTypeEntity, types.WikiPageStatusPublished)
	titled.SourceRefs = types.StringArray{"knowledge-1|Doc title"}
	otherKB := makeWikiPage("kb-other", "entity/leaked", types.WikiPageTypeEntity, types.WikiPageStatusPublished)
	otherKB.SourceRefs = types.StringArray{"knowledge-1"}
	prefixOnly := makeWikiPage("kb-src", "entity/prefix-only", types.WikiPageTypeEntity, types.WikiPageStatusPublished)
	prefixOnly.SourceRefs = types.StringArray{"knowledge-10|Different"}
	escapedLike := makeWikiPage("kb-src", "entity/escaped-like", types.WikiPageTypeEntity, types.WikiPageStatusPublished)
	escapedLike.SourceRefs = types.StringArray{"knowledge_2|Doc title"}
	likeNeighbor := makeWikiPage("kb-src", "entity/like-neighbor", types.WikiPageTypeEntity, types.WikiPageStatusPublished)
	likeNeighbor.SourceRefs = types.StringArray{"knowledgeX2|Different"}

	for _, p := range []*types.WikiPage{exact, titled, otherKB, prefixOnly, escapedLike, likeNeighbor} {
		require.NoError(t, repo.Create(ctx, p))
	}

	pages, err := repo.ListBySourceRef(ctx, "kb-src", "knowledge-1")
	require.NoError(t, err)
	require.Len(t, pages, 2)
	assert.ElementsMatch(t, []string{"summary/exact", "entity/titled"}, []string{pages[0].Slug, pages[1].Slug})

	slugs, err := repo.ListSlugsBySourceRef(ctx, "kb-src", "knowledge-1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"summary/exact", "entity/titled"}, slugs)

	escapedPages, err := repo.ListBySourceRef(ctx, "kb-src", "knowledge_2")
	require.NoError(t, err)
	require.Len(t, escapedPages, 1)
	assert.Equal(t, "entity/escaped-like", escapedPages[0].Slug)
}

func TestListSummariesByKnowledgeIDs_SQLiteMatchesSourceRefs(t *testing.T) {
	db := setupWikiPagesTestDB(t)
	repo := NewWikiPageRepository(db)
	ctx := context.Background()

	exact := makeWikiPage("kb-src", "summary/exact", types.WikiPageTypeSummary, types.WikiPageStatusPublished)
	exact.Content = "exact summary"
	exact.SourceRefs = types.StringArray{"knowledge-1"}
	titled := makeWikiPage("kb-src", "summary/titled", types.WikiPageTypeSummary, types.WikiPageStatusPublished)
	titled.Content = "titled summary"
	titled.SourceRefs = types.StringArray{"knowledge-2|Doc title"}
	entity := makeWikiPage("kb-src", "entity/not-summary", types.WikiPageTypeEntity, types.WikiPageStatusPublished)
	entity.SourceRefs = types.StringArray{"knowledge-1"}
	archived := makeWikiPage("kb-src", "summary/archived", types.WikiPageTypeSummary, types.WikiPageStatusArchived)
	archived.SourceRefs = types.StringArray{"knowledge-3"}
	otherKB := makeWikiPage("kb-other", "summary/leaked", types.WikiPageTypeSummary, types.WikiPageStatusPublished)
	otherKB.SourceRefs = types.StringArray{"knowledge-1"}

	for _, p := range []*types.WikiPage{exact, titled, entity, archived, otherKB} {
		require.NoError(t, repo.Create(ctx, p))
	}

	got, err := repo.ListSummariesByKnowledgeIDs(ctx, "kb-src", []string{"knowledge-1", "knowledge-2", "knowledge-3"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"knowledge-1": "exact summary",
		"knowledge-2": "titled summary",
	}, got)
}
