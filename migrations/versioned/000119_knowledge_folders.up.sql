-- Migration 000119: knowledge_folders, the registry of folders that exist
-- without holding a document
--
-- The knowledge-base sidebar tree is a projection: ListKnowledgeFolderCounts
-- groups knowledges.folder_path, and BuildKnowledgeFolderTree materializes the
-- ancestors, so a folder is visible only while some document's path mentions
-- it. That makes an empty folder unrepresentable, which is what this table is
-- for. It is deliberately a *registry of paths*, not a tree: rows are unioned
-- with the projection in ListKnowledgeFolderTree, so every existing write path
-- (folder upload, move, data-source sync) keeps working untouched and can never
-- "lose" a folder by forgetting to register it.
--
-- Numbering: 000119, immediately after upstream v0.8.3's last migration (118).
-- This file was originally 000900, in a high band meant to avoid collisions
-- when merging upstream. That band backfired: golang-migrate only applies
-- versions above the recorded one, so a database already at 900 silently
-- skipped every upstream migration below it. Keep local migrations sequential
-- with upstream and renumber them whenever a new upstream tag lands.
--
-- PostgreSQL only. migrations/sqlite/ is a separate lineage and is not touched:
-- this deployment runs PostgreSQL, and mirroring the table into a lineage we
-- never execute would create a second source of truth to keep in step by hand.
CREATE TABLE IF NOT EXISTS knowledge_folders (
    id                VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id         BIGINT      NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    -- Canonical form produced by types.NormalizeKnowledgeFolderPath: forward
    -- slashes, no leading/trailing separator, depth <= 16, <= 1024 bytes.
    -- Same vocabulary as knowledges.folder_path, which is what lets the two
    -- sources merge by string comparison.
    path              VARCHAR(1024) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMPTZ
);

-- One live row per (kb, path). Partial so a deleted path can be created again
-- without resurrecting history, and so the uniqueness rule matches the
-- soft-delete scope GORM applies to queries.
CREATE UNIQUE INDEX IF NOT EXISTS knowledge_folders_kb_path_active
    ON knowledge_folders(knowledge_base_id, path) WHERE deleted_at IS NULL;

-- Every read filters by kb + tenant.
CREATE INDEX IF NOT EXISTS knowledge_folders_kb_tenant
    ON knowledge_folders(knowledge_base_id, tenant_id);
