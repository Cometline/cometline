package server

import (
	"net/http"

	skillpkg "github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/gin-gonic/gin"
)

func (a *App) handleListArchivedSkills(c *gin.Context) {
	if a.curator == nil {
		c.JSON(http.StatusOK, gin.H{"skills": []skillResource{}})
		return
	}
	names, err := a.curator.ArchivedNames(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "skill_error", err.Error())
		return
	}
	items := make([]skillResource, 0, len(names))
	for _, name := range names {
		skill, _, err := skillpkg.ReadArchivedSkill(name)
		if err != nil {
			continue
		}
		resource := a.skillResourceFromModel(skill)
		resource.Status = skillpkg.CuratorStatusArchived
		items = append(items, resource)
	}
	c.JSON(http.StatusOK, gin.H{"skills": items})
}

func (a *App) handlePinSkill(c *gin.Context) {
	a.setSkillPin(c, true)
}

func (a *App) handleUnpinSkill(c *gin.Context) {
	a.setSkillPin(c, false)
}

func (a *App) setSkillPin(c *gin.Context, pinned bool) {
	if a.curator == nil {
		writeError(c, http.StatusInternalServerError, "skill_error", "curator is not configured")
		return
	}
	name := c.Param("name")
	if err := a.curator.SetPinned(c.Request.Context(), name, pinned); err != nil {
		writeError(c, http.StatusBadRequest, "skill_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": name, "pinned": pinned})
}

func (a *App) handleRestoreSkill(c *gin.Context) {
	if a.curator == nil {
		writeError(c, http.StatusInternalServerError, "skill_error", "curator is not configured")
		return
	}
	name := c.Param("name")
	if err := a.curator.Restore(c.Request.Context(), name); err != nil {
		writeError(c, http.StatusBadRequest, "skill_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": name, "status": skillpkg.CuratorStatusActive})
}
