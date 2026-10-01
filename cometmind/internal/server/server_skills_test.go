package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
)

func TestSkillsDeleteAndExport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	content := "---\nname: api-skill\ndescription: api test skill\n---\n\n# API\n"
	if err := skills.WriteSkill("api-skill", content, false); err != nil {
		t.Fatalf("WriteSkill() error = %v", err)
	}

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	exportRec := httptest.NewRecorder()
	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/skills/api-skill/archive", nil)
	engine.ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d body=%s", exportRec.Code, http.StatusOK, exportRec.Body.String())
	}
	if ct := exportRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/zip") {
		t.Fatalf("export content-type = %q", ct)
	}
	if exportRec.Body.Len() == 0 {
		t.Fatal("export body is empty")
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/skills/api-skill", nil)
	engine.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d body=%s", deleteRec.Code, http.StatusOK, deleteRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)
	engine.ServeHTTP(listRec, listReq)
	var list listSkillsResponse
	decodeJSON(t, listRec.Body.Bytes(), &list)
	for _, skill := range list.Skills {
		if skill.Name == "api-skill" {
			t.Fatalf("skill still listed after delete: %+v", list.Skills)
		}
	}
}

func TestSkillsGetAndUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	content := "---\nname: api-skill\ndescription: api test skill\n---\n\n# API\n"
	if err := skills.WriteSkill("api-skill", content, false); err != nil {
		t.Fatalf("WriteSkill() error = %v", err)
	}

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	missingRec := httptest.NewRecorder()
	engine.ServeHTTP(missingRec, httptest.NewRequest(http.MethodGet, "/api/v1/skills/missing-skill", nil))
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want %d body=%s", missingRec.Code, http.StatusNotFound, missingRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	engine.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/v1/skills/api-skill", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var detail skillDetailResponse
	decodeJSON(t, getRec.Body.Bytes(), &detail)
	if detail.Skill.Name != "api-skill" || !detail.Skill.CanEdit || !strings.Contains(detail.Content, "api test skill") {
		t.Fatalf("get detail = %+v", detail)
	}

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/skills/api-skill", bytes.NewBufferString(`{"content":"---\nname: api-skill\ndescription: edited api skill\n---\n\n# API\n\nUpdated body\n"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}
	decodeJSON(t, updateRec.Body.Bytes(), &detail)
	if detail.Skill.Description != "edited api skill" || !strings.Contains(detail.Content, "Updated body") {
		t.Fatalf("updated skill = %+v", detail)
	}

	mismatchRec := httptest.NewRecorder()
	mismatchReq := httptest.NewRequest(http.MethodPut, "/api/v1/skills/api-skill", bytes.NewBufferString(`{"content":"---\nname: other\ndescription: wrong\n---\n\n# No\n"}`))
	mismatchReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(mismatchRec, mismatchReq)
	if mismatchRec.Code != http.StatusConflict {
		t.Fatalf("mismatch status = %d, want %d body=%s", mismatchRec.Code, http.StatusConflict, mismatchRec.Body.String())
	}

	builtinRec := httptest.NewRecorder()
	builtinReq := httptest.NewRequest(http.MethodPut, "/api/v1/skills/llm-wiki", bytes.NewBufferString(`{"content":"---\nname: llm-wiki\ndescription: edited builtin\n---\n\n# No\n"}`))
	builtinReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(builtinRec, builtinReq)
	if builtinRec.Code != http.StatusForbidden {
		t.Fatalf("builtin status = %d, want %d body=%s", builtinRec.Code, http.StatusForbidden, builtinRec.Body.String())
	}
}

func TestSkillDraftHandlersPromoteAndReject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	draftContent := "---\nname: api-draft\ndescription: api draft skill\n---\n\n# Draft\n"
	if err := skills.WriteDraft("api-draft", draftContent, false); err != nil {
		t.Fatalf("WriteDraft() error = %v", err)
	}

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/skill-drafts", nil)
	engine.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var list listSkillDraftsResponse
	decodeJSON(t, listRec.Body.Bytes(), &list)
	if len(list.Drafts) != 1 || list.Drafts[0].Name != "api-draft" {
		t.Fatalf("draft list = %+v, want api-draft", list.Drafts)
	}

	detailRec := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/skill-drafts/api-draft", nil)
	engine.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d body=%s", detailRec.Code, http.StatusOK, detailRec.Body.String())
	}
	var detail skillDraftDetailResponse
	decodeJSON(t, detailRec.Body.Bytes(), &detail)
	if !strings.Contains(detail.Content, "api draft skill") {
		t.Fatalf("detail content missing draft text: %q", detail.Content)
	}

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/skill-drafts/api-draft", bytes.NewBufferString(`{"content":"---\nname: api-draft\ndescription: edited api draft skill\n---\n\n# Draft\n\nUpdated body\n"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}
	decodeJSON(t, updateRec.Body.Bytes(), &detail)
	if detail.Draft.Description != "edited api draft skill" || !strings.Contains(detail.Content, "Updated body") {
		t.Fatalf("updated draft = %+v", detail)
	}

	promoteRec := httptest.NewRecorder()
	promoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/skill-drafts/api-draft/promote", nil)
	engine.ServeHTTP(promoteRec, promoteReq)
	if promoteRec.Code != http.StatusOK {
		t.Fatalf("promote status = %d, want %d body=%s", promoteRec.Code, http.StatusOK, promoteRec.Body.String())
	}

	skillsRec := httptest.NewRecorder()
	skillsReq := httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)
	engine.ServeHTTP(skillsRec, skillsReq)
	if skillsRec.Code != http.StatusOK {
		t.Fatalf("skills status = %d, want %d body=%s", skillsRec.Code, http.StatusOK, skillsRec.Body.String())
	}
	var skillsList listSkillsResponse
	decodeJSON(t, skillsRec.Body.Bytes(), &skillsList)
	found := false
	for _, skill := range skillsList.Skills {
		if skill.Name == "api-draft" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("promoted skill not listed: %+v", skillsList.Skills)
	}

	rejectContent := "---\nname: rejected-draft\ndescription: rejected draft skill\n---\n\n# Reject\n"
	if err := skills.WriteDraft("rejected-draft", rejectContent, false); err != nil {
		t.Fatalf("WriteDraft(rejected) error = %v", err)
	}
	rejectRec := httptest.NewRecorder()
	rejectReq := httptest.NewRequest(http.MethodDelete, "/api/v1/skill-drafts/rejected-draft", nil)
	engine.ServeHTTP(rejectRec, rejectReq)
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("reject status = %d, want %d body=%s", rejectRec.Code, http.StatusOK, rejectRec.Body.String())
	}

	missingRec := httptest.NewRecorder()
	missingReq := httptest.NewRequest(http.MethodDelete, "/api/v1/skill-drafts/rejected-draft", nil)
	engine.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing reject status = %d, want %d body=%s", missingRec.Code, http.StatusNotFound, missingRec.Body.String())
	}
}
