package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// This file owns knowledge_folders, the registry that lets a directory exist
// without holding a document. Everything else about folders stays where it was:
// the tree is still projected from knowledges.folder_path and the two sources
// are unioned in the service layer.

// folderSubtreeCondition is the WHERE fragment shared by every "this path or
// below it" query, so a folder and its descendants are always matched the same
// way. The ESCAPE clause pairs with escapeLikeKeyword, which is required because
// folder paths routinely contain "_" and "%" (dates, client names).
const folderSubtreeCondition = "path = ? OR path LIKE ? ESCAPE ?"

// knowledgeSubtreeCondition is the same idea for knowledges.folder_path.
const knowledgeSubtreeCondition = "folder_path = ? OR folder_path LIKE ? ESCAPE ?"

// subtreeArgs builds the bind parameters for a "this path or below" condition.
// The escape character is knowledge.go's own likeEscapeChar, so the clause and
// the escaping of the pattern can never drift apart.
func subtreeArgs(path string) []interface{} {
	return []interface{}{path, escapeLikeKeyword(path) + "/%", likeEscapeChar}
}

// ListRegisteredFolderPaths returns every live registry path in a knowledge base.
func (r *knowledgeRepository) ListRegisteredFolderPaths(
	ctx context.Context,
	tenantID uint64,
	kbID string,
) ([]string, error) {
	var paths []string
	err := r.db.WithContext(ctx).
		Model(&types.KnowledgeFolder{}).
		Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, kbID).
		Order("path ASC").
		Pluck("path", &paths).Error
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// FindRegisteredFolder returns the live registry row for an exact path, or nil
// when there is none. nil is also the answer for a projected folder, which is
// what lets the delete path explain "this folder comes from its documents".
func (r *knowledgeRepository) FindRegisteredFolder(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	path string,
) (*types.KnowledgeFolder, error) {
	var folder types.KnowledgeFolder
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ? AND path = ?", tenantID, kbID, path).
		First(&folder).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

// CreateRegisteredFolder inserts one registry row.
func (r *knowledgeRepository) CreateRegisteredFolder(
	ctx context.Context,
	folder *types.KnowledgeFolder,
) error {
	if folder == nil || folder.Path == "" {
		return errors.New("registered folder requires a non-empty path")
	}
	return r.db.WithContext(ctx).Create(folder).Error
}

// DeleteRegisteredFolder soft-deletes one registry row, scoped by tenant and KB
// so a request against one KB cannot unregister another KB's folder.
func (r *knowledgeRepository) DeleteRegisteredFolder(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	path string,
) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ? AND path = ?", tenantID, kbID, path).
		Delete(&types.KnowledgeFolder{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountFolderSubtreeDocuments counts entries stored directly in path or in any
// folder below it. The parse_status exclusion matches ListKnowledgeFolderCounts
// deliberately: if it counted rows the tree hides, a folder would show "0" in
// the sidebar yet refuse to be deleted as non-empty.
func (r *knowledgeRepository) CountFolderSubtreeDocuments(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	path string,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.Knowledge{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND parse_status <> ? AND ("+knowledgeSubtreeCondition+")",
			append([]interface{}{tenantID, kbID, types.ParseStatusDeleting}, subtreeArgs(path)...)...).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListRegisteredChildFolders returns the registered folders directly below path.
// Projected children are not needed here: one implies a document below it, which
// CountFolderSubtreeDocuments already reports.
func (r *knowledgeRepository) ListRegisteredChildFolders(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	path string,
) ([]string, error) {
	var paths []string
	err := r.db.WithContext(ctx).
		Model(&types.KnowledgeFolder{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND path LIKE ? ESCAPE ?",
			tenantID, kbID, escapeLikeKeyword(path)+"/%", likeEscapeChar).
		Order("path ASC").
		Pluck("path", &paths).Error
	if err != nil {
		return nil, err
	}
	direct := make([]string, 0, len(paths))
	prefix := path + "/"
	for _, candidate := range paths {
		suffix := strings.TrimPrefix(candidate, prefix)
		if suffix == "" || strings.Contains(suffix, "/") {
			continue
		}
		direct = append(direct, candidate)
	}
	return direct, nil
}

// renameRegisteredFolders applies a folder rename to the registry so a rename
// cannot leave a row pointing at a path that no longer exists. It runs on the
// caller's transaction handle, because the document-side rewrite and this one
// have to commit or roll back together.
//
// Renaming onto an existing folder merges the two on the document side already,
// so the registry merges too: a row whose new path is taken is dropped instead
// of violating the partial unique index.
func renameRegisteredFolders(
	tx *gorm.DB,
	tenantID uint64,
	kbID string,
	from string,
	to string,
) (int64, error) {
	var moving []types.KnowledgeFolder
	if err := tx.
		Where("tenant_id = ? AND knowledge_base_id = ? AND ("+folderSubtreeCondition+")",
			append([]interface{}{tenantID, kbID}, subtreeArgs(from)...)...).
		Order("path ASC").
		Find(&moving).Error; err != nil {
		return 0, err
	}
	if len(moving) == 0 {
		return 0, nil
	}

	// Paths on the destination side that are not part of the move already exist
	// and win the merge.
	var staying []string
	if err := tx.Model(&types.KnowledgeFolder{}).
		Where("tenant_id = ? AND knowledge_base_id = ? AND path LIKE ? ESCAPE ? AND path NOT LIKE ? ESCAPE ?",
			tenantID, kbID,
			escapeLikeKeyword(to)+"/%", likeEscapeChar,
			escapeLikeKeyword(from)+"/%", likeEscapeChar,
		).
		Pluck("path", &staying).Error; err != nil {
		return 0, err
	}
	taken := make(map[string]struct{}, len(staying)+1)
	for _, path := range staying {
		taken[path] = struct{}{}
	}
	if _, ok := taken[to]; !ok && to != from {
		var target types.KnowledgeFolder
		err := tx.Where("tenant_id = ? AND knowledge_base_id = ? AND path = ?", tenantID, kbID, to).
			First(&target).Error
		if err == nil {
			taken[to] = struct{}{}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}

	var affected int64
	for _, folder := range moving {
		newPath := types.NormalizeKnowledgeFolderPath(to + strings.TrimPrefix(folder.Path, from))
		if newPath == "" {
			continue
		}
		if _, clash := taken[newPath]; clash {
			// Merged into an existing folder: drop the moving row rather than
			// create a second row for one path.
			if err := tx.Where("id = ?", folder.ID).Delete(&types.KnowledgeFolder{}).Error; err != nil {
				return 0, err
			}
			affected++
			continue
		}
		taken[newPath] = struct{}{}
		if newPath == folder.Path {
			continue
		}
		if err := tx.Model(&types.KnowledgeFolder{}).
			Where("id = ?", folder.ID).
			Updates(map[string]interface{}{"path": newPath, "updated_at": time.Now()}).Error; err != nil {
			return 0, err
		}
		affected++
	}
	return affected, nil
}
