package service

import (
	"context"
	"fmt"
	"strings"

	werrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// This file implements the two folder operations the projection cannot express:
// creating a directory that holds nothing, and removing one. Everything else
// about folders (listing, renaming, moving documents in) keeps working off
// knowledges.folder_path untouched, because ListKnowledgeFolderTree unions the
// registry with the projection rather than replacing it.

// writableKnowledgeBaseForFolders resolves the KB and the caller's write grant,
// the same pair of checks RenameKnowledgeFolder performs.
func (s *knowledgeService) writableKnowledgeBaseForFolders(
	ctx context.Context,
	kbID string,
) (*types.KnowledgeBase, context.Context, error) {
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, ctx, err
	}
	if kb == nil || kb.ID != kbID {
		return nil, ctx, werrors.NewNotFoundError("knowledge base not found")
	}
	ctx, err = requireKBWrite(ctx, kb)
	if err != nil {
		return nil, ctx, err
	}
	return kb, ctx, nil
}

// CreateKnowledgeFolder registers an empty folder so it appears in the sidebar
// tree without holding a document.
func (s *knowledgeService) CreateKnowledgeFolder(
	ctx context.Context,
	kbID string,
	folderPath string,
) (*types.KnowledgeFolder, error) {
	normalized, err := normalizeTargetFolderPath(ctx, folderPath)
	if err != nil {
		return nil, err
	}
	if normalized == "" {
		return nil, werrors.NewValidationError("文件夹名称不能为空")
	}
	kb, ctx, err := s.writableKnowledgeBaseForFolders(ctx, kbID)
	if err != nil {
		return nil, err
	}
	tenantID := kb.TenantID

	existing, err := s.repo.FindRegisteredFolder(ctx, tenantID, kbID, normalized)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, werrors.NewConflictError("该目录已经存在")
	}
	// A subtree count covers both "documents here" and "documents in a folder
	// below, which already renders this path in the tree". Either way the folder
	// the caller asks for is already visible, so registering it would be a no-op
	// that silently succeeds.
	documents, err := s.repo.CountFolderSubtreeDocuments(ctx, tenantID, kbID, normalized)
	if err != nil {
		return nil, err
	}
	if documents > 0 {
		return nil, werrors.NewConflictError("该目录已经存在")
	}

	folder := types.NewKnowledgeFolder(tenantID, kbID, normalized)
	if err := s.repo.CreateRegisteredFolder(ctx, folder); err != nil {
		logger.Errorf(ctx, "Failed to register folder %q in kb %s: %v", normalized, kbID, err)
		return nil, err
	}
	logger.Infof(ctx, "Registered empty folder %q in kb %s", normalized, kbID)
	return folder, nil
}

// DeleteKnowledgeFolder unregisters a folder. It refuses unless the folder is
// genuinely empty: no document anywhere below it, and no registered child folder.
func (s *knowledgeService) DeleteKnowledgeFolder(
	ctx context.Context,
	kbID string,
	folderPath string,
) error {
	normalized, err := normalizeTargetFolderPath(ctx, folderPath)
	if err != nil {
		return err
	}
	if normalized == "" {
		return werrors.NewValidationError("文件夹名称不能为空")
	}
	kb, ctx, err := s.writableKnowledgeBaseForFolders(ctx, kbID)
	if err != nil {
		return err
	}
	tenantID := kb.TenantID

	folder, err := s.repo.FindRegisteredFolder(ctx, tenantID, kbID, normalized)
	if err != nil {
		return err
	}
	if folder == nil {
		// Projected folders have no row to remove; they disappear with their
		// last document. Saying "not empty" here would be the wrong explanation.
		return werrors.NewNotFoundError(
			"该目录不是新建的空目录，它由里面的文档推导而来；删掉或移出最后一样东西，它就会消失")
	}

	documents, err := s.repo.CountFolderSubtreeDocuments(ctx, tenantID, kbID, normalized)
	if err != nil {
		return err
	}
	if documents > 0 {
		return werrors.NewConflictError(fmt.Sprintf("该目录下还有 %d 篇文档，请先移出或删除它们", documents))
	}
	// Only registered children need checking here: a projected child folder
	// implies a document below it, which the count above already reported.
	children, err := s.repo.ListRegisteredChildFolders(ctx, tenantID, kbID, normalized)
	if err != nil {
		return err
	}
	if len(children) > 0 {
		names := make([]string, 0, len(children))
		for _, child := range children {
			names = append(names, types.KnowledgeFolderName(child))
		}
		return werrors.NewConflictError(fmt.Sprintf(
			"该目录下还有 %d 个子目录（%s），请先删除子目录", len(children), strings.Join(names, "、")))
	}

	if err := s.repo.DeleteRegisteredFolder(ctx, tenantID, kbID, normalized); err != nil {
		logger.Errorf(ctx, "Failed to unregister folder %q in kb %s: %v", normalized, kbID, err)
		return err
	}
	logger.Infof(ctx, "Unregistered empty folder %q in kb %s", normalized, kbID)
	return nil
}
