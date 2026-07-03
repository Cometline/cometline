package server

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/skills"
	"github.com/gin-gonic/gin"
)

type skillDraftResource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type listSkillDraftsResponse struct {
	Drafts []skillDraftResource `json:"drafts"`
}

type skillDraftDetailResponse struct {
	Draft   skillDraftResource `json:"draft"`
	Content string             `json:"content"`
}

func skillDraftResourceFromModel(draft skills.Draft) skillDraftResource {
	return skillDraftResource{
		Name:        draft.Name,
		Description: draft.Description,
		Path:        draft.Path,
		CreatedAt:   draft.CreatedAt,
		UpdatedAt:   draft.UpdatedAt,
	}
}

func writeSkillDraftError(c *gin.Context, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeError(c, http.StatusNotFound, "draft_not_found", "unknown skill draft")
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid") || strings.Contains(msg, "already exists") {
		writeError(c, http.StatusConflict, "draft_conflict", msg)
		return
	}
	writeError(c, http.StatusInternalServerError, "draft_error", msg)
}

func (a *App) handleListSkillDrafts(c *gin.Context) {
	drafts, err := skills.ListDrafts()
	if err != nil {
		writeSkillDraftError(c, err)
		return
	}
	items := make([]skillDraftResource, 0, len(drafts))
	for _, draft := range drafts {
		items = append(items, skillDraftResourceFromModel(draft))
	}
	c.JSON(http.StatusOK, listSkillDraftsResponse{Drafts: items})
}

func (a *App) handleGetSkillDraft(c *gin.Context) {
	draft, content, err := skills.DraftMarkdown(c.Param("name"))
	if err != nil {
		writeSkillDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, skillDraftDetailResponse{Draft: skillDraftResourceFromModel(draft), Content: content})
}

func (a *App) handlePromoteSkillDraft(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if err := skills.PromoteDraft(name); err != nil {
		writeSkillDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, statusResponse{Status: "promoted"})
}

func (a *App) handleRejectSkillDraft(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if err := skills.RejectDraft(name); err != nil {
		writeSkillDraftError(c, err)
		return
	}
	c.JSON(http.StatusOK, statusResponse{Status: "rejected"})
}
