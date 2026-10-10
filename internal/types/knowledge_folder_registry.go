package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// KnowledgeFolder registers a directory inside a knowledge base so that it stays
// visible after its last document is removed, and so that an empty one can be
// created at all. The sidebar tree is otherwise a projection of
// knowledges.folder_path (see BuildKnowledgeFolderTree), which cannot represent
// a folder that holds nothing.
//
// It stores a canonical path rather than a parent_id/name pair on purpose: the
// projection side only ever yields paths, so keeping the same vocabulary makes
// the union a string comparison and lets folder renames reuse the same prefix
// rewrite that renaming a projected folder already performs.
type KnowledgeFolder struct {
	ID string `json:"id" gorm:"type:varchar(36);primaryKey"`
	// TenantID mirrors the owning tenant so that every read can be scoped the
	// same way the projection query is, without joining knowledge_bases.
	TenantID        uint64         `json:"tenant_id" gorm:"not null"`
	KnowledgeBaseID string         `json:"knowledge_base_id" gorm:"type:varchar(36);not null"`
	Path            string         `json:"path" gorm:"type:varchar(1024);not null"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName pins the table name to migration 000900.
func (KnowledgeFolder) TableName() string { return "knowledge_folders" }

// NewKnowledgeFolder builds a registry row for an already-normalized path.
func NewKnowledgeFolder(tenantID uint64, kbID, normalizedPath string) *KnowledgeFolder {
	return &KnowledgeFolder{
		ID:              uuid.New().String(),
		TenantID:        tenantID,
		KnowledgeBaseID: kbID,
		Path:            normalizedPath,
	}
}

// MergeRegisteredFolders unions the registry with the per-folder counts that
// BuildKnowledgeFolderTree consumes. A registered path that already has a
// projection row contributes nothing beyond the row it already has; one without
// a row becomes a zero-count entry, which is what makes an empty folder show up.
//
// Ancestors are not inserted here: BuildKnowledgeFolderTree materializes missing
// intermediate folders itself, so registering only "a/b/c" still renders "a" and
// "b" as connected nodes.
func MergeRegisteredFolders(counts []*KnowledgeFolderCount, registered []string) []*KnowledgeFolderCount {
	if len(registered) == 0 {
		return counts
	}
	seen := make(map[string]struct{}, len(counts)+len(registered))
	for _, row := range counts {
		if row == nil {
			continue
		}
		seen[NormalizeKnowledgeFolderPath(row.FolderPath)] = struct{}{}
	}
	merged := counts
	for _, raw := range registered {
		path := NormalizeKnowledgeFolderPath(raw)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		merged = append(merged, &KnowledgeFolderCount{FolderPath: path, Count: 0})
	}
	return merged
}
