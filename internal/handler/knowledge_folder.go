package handler

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// This file covers the two folder operations that the document projection
// cannot express: registering an empty folder and removing one again. Listing
// and renaming live on in knowledge.go, where they already were.

// CreateKnowledgeFolderRequest is the body schema for POST
// /knowledge-bases/:id/knowledge/folders.
type CreateKnowledgeFolderRequest struct {
	// FolderPath is the directory to create, relative to the knowledge base
	// root. Nested paths are created in one call; the ancestors the tree needs
	// are materialized by the same code that renders projected folders.
	FolderPath string `json:"folder_path"`
}

// CreateKnowledgeFolder godoc
// @Summary      新建空文件夹
// @Description  在知识库中登记一个不依赖文档存在的文件夹。投影模型下目录只由文档路径推导而来，
// @Description  因此本接口是唯一能创建空目录的入口；目录已存在（登记过或文档推导出来）时返回 409
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id       path      string                       true  "知识库ID"
// @Param        request  body      CreateKnowledgeFolderRequest true  "新建目录"
// @Success      201      {object}  map[string]interface{}       "新建成功"
// @Failure      400      {object}  errors.AppError              "请求参数错误"
// @Failure      403      {object}  errors.AppError              "权限不足"
// @Failure      409      {object}  errors.AppError              "目录已存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/knowledge/folders [post]
func (h *KnowledgeHandler) CreateKnowledgeFolder(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateKnowledgeFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError("Invalid request parameters: " + err.Error()))
		return
	}

	kbID, effectiveTenantID, err := h.requireKnowledgeWriteAccess(c, c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	ctx = types.WithExecutionTenant(c.Request.Context(), effectiveTenantID)

	folder, err := h.kgService.CreateKnowledgeFolder(ctx, kbID, req.FolderPath)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge folder created, kb_id=%s path=%s",
		secutils.SanitizeForLog(kbID), secutils.SanitizeForLog(folder.Path))
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":          folder.ID,
			"folder_path": folder.Path,
			"name":        types.KnowledgeFolderName(folder.Path),
		},
	})
}

// DeleteKnowledgeFolder godoc
// @Summary      删除空文件夹
// @Description  注销一个新建的空目录。目录内（含任意层级）还有文档、或还有子目录时拒绝删除并说明原因，
// @Description  不会隐式移动或删除任何文档
// @Tags         知识管理
// @Produce      json
// @Param        id           path      string  true  "知识库ID"
// @Param        folder_path  query     string  true  "要删除的目录路径"
// @Success      204          "删除成功"
// @Failure      400          {object}  errors.AppError  "请求参数错误"
// @Failure      403          {object}  errors.AppError  "权限不足"
// @Failure      404          {object}  errors.AppError  "该目录不是新建的空目录"
// @Failure      409          {object}  errors.AppError  "目录非空"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/knowledge/folders [delete]
func (h *KnowledgeHandler) DeleteKnowledgeFolder(c *gin.Context) {
	ctx := c.Request.Context()

	kbID, effectiveTenantID, err := h.requireKnowledgeWriteAccess(c, c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	ctx = types.WithExecutionTenant(c.Request.Context(), effectiveTenantID)

	folderPath := c.Query("folder_path")
	if err := h.kgService.DeleteKnowledgeFolder(ctx, kbID, folderPath); err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge folder deleted, kb_id=%s path=%s",
		secutils.SanitizeForLog(kbID), secutils.SanitizeForLog(folderPath))
	c.Status(http.StatusNoContent)
}
