package server

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
	skillpkg "github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/gin-gonic/gin"
)

type skillResource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	Internal    bool   `json:"internal"`
	IsSymlink   bool   `json:"is_symlink"`
	CanDelete   bool   `json:"can_delete"`
	CanExport   bool   `json:"can_export"`
	CanEdit     bool   `json:"can_edit"`
	Origin      string `json:"origin,omitempty"`
}

type skillDetailResponse struct {
	Skill   skillResource `json:"skill"`
	Content string        `json:"content"`
}

type listSkillsResponse struct {
	Skills []skillResource `json:"skills"`
	Errors []string        `json:"errors,omitempty"`
}

type syncSkillsResponse struct {
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

func (a *App) handleListSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	items := make([]skillResource, 0, len(reg.Skills))
	for _, skill := range reg.Skills {
		items = append(items, skillResourceFromModel(skill))
	}
	c.JSON(http.StatusOK, listSkillsResponse{Skills: items, Errors: reg.Errors})
}

func (a *App) handleSyncSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	created, skipped, err := reg.SyncMirror(filepath.Join("~", ".cometmind", "skills"))
	if err != nil {
		writeError(c, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, syncSkillsResponse{Created: created, Skipped: skipped, Errors: reg.Errors})
}

func (a *App) handleExportSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	name := strings.TrimSpace(c.Param("name"))
	skill, ok := reg.Find(name)
	if !ok {
		writeError(c, http.StatusNotFound, "skill_not_found", "unknown skill: "+name)
		return
	}
	caps, err := skillpkg.SkillCapabilities(skill)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !caps.CanExport {
		writeError(c, http.StatusForbidden, "export_forbidden", "skill cannot be exported")
		return
	}
	data, err := skillpkg.ExportSkill(skill)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".zip"))
	c.Data(http.StatusOK, "application/zip", data)
}

func (a *App) handleDeleteSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	name := strings.TrimSpace(c.Param("name"))
	skill, ok := reg.Find(name)
	if !ok {
		writeError(c, http.StatusNotFound, "skill_not_found", "unknown skill: "+name)
		return
	}
	caps, err := skillpkg.SkillCapabilities(skill)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !caps.CanDelete {
		writeError(c, http.StatusForbidden, "delete_forbidden", "workspace and bundled skills cannot be deleted")
		return
	}
	if err := skillpkg.DeleteManagedSkill(skill); err != nil {
		writeError(c, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, statusResponse{Status: "deleted"})
}

func (a *App) handleGetSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	name := strings.TrimSpace(c.Param("name"))
	skill, content, err := reg.SkillMarkdown(name)
	if err != nil {
		writeDiscoveredSkillError(c, err)
		return
	}
	c.JSON(http.StatusOK, skillDetailResponse{Skill: skillResourceFromModel(skill), Content: content})
}

func (a *App) handleUpdateSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	name := strings.TrimSpace(c.Param("name"))
	skill, ok := reg.Find(name)
	if !ok {
		writeError(c, http.StatusNotFound, "skill_not_found", "unknown skill: "+name)
		return
	}
	var req apigen.UpdateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if err := skillpkg.UpdateDiscoveredSkill(skill, req.Content); err != nil {
		writeDiscoveredSkillError(c, err)
		return
	}
	updated, err := skillpkg.ReadSkill(skill.Path)
	if err != nil {
		writeDiscoveredSkillError(c, err)
		return
	}
	_, content, err := reg.SkillMarkdown(name)
	if err != nil {
		writeDiscoveredSkillError(c, err)
		return
	}
	c.JSON(http.StatusOK, skillDetailResponse{Skill: skillResourceFromModel(updated), Content: content})
}

func writeDiscoveredSkillError(c *gin.Context, err error) {
	if errors.Is(err, skillpkg.ErrSkillNotEditable) {
		writeError(c, http.StatusForbidden, "edit_forbidden", err.Error())
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown skill") {
		writeError(c, http.StatusNotFound, "skill_not_found", msg)
		return
	}
	if strings.Contains(msg, "invalid") || strings.Contains(msg, "must match") || strings.Contains(msg, "required") {
		writeError(c, http.StatusConflict, "skill_conflict", msg)
		return
	}
	writeError(c, http.StatusInternalServerError, "skill_error", msg)
}

func (a *App) skillsForRequest(c *gin.Context) skillpkg.Registry {
	workspacePath := strings.TrimSpace(c.Query("workspace_path"))
	if workspacePath == "" && strings.TrimSpace(c.Query("workspace_id")) != "" {
		if path, err := a.sessions.WorkspacePath(c.Request.Context(), strings.TrimSpace(c.Query("workspace_id"))); err == nil {
			workspacePath = path
		}
	}
	return skillpkg.Discover(workspacePath, a.config.SkillSettings())
}

func skillResourceFromModel(skill skillpkg.Skill) skillResource {
	caps, _ := skillpkg.SkillCapabilities(skill)
	return skillResource{
		Name:        skill.Name,
		Description: skill.Description,
		Path:        skill.Path,
		Source:      skill.Source,
		Internal:    skill.Internal,
		IsSymlink:   caps.IsSymlink,
		CanDelete:   caps.CanDelete,
		CanExport:   caps.CanExport,
		CanEdit:     caps.CanEdit,
		Origin:      skill.Origin,
	}
}
