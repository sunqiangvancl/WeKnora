-- Drop the registry. The table is additive and nothing references it with a
-- foreign key, so this only removes registered-but-empty folders; documents,
-- chunks, embeddings and stored files are untouched.
DROP INDEX IF EXISTS knowledge_folders_kb_tenant;
DROP INDEX IF EXISTS knowledge_folders_kb_path_active;
DROP TABLE IF EXISTS knowledge_folders;
