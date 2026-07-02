package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/planning"
)

type PlanWrite struct {
	Planning  *planning.Service
	SessionID string
}

func (PlanWrite) Spec() ToolSpec {
	return ToolSpec{
		Name:        "plan_write",
		Description: "Replace the current session plan with an ordered checklist of steps for this multi-step turn.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"steps":{
					"type":"array",
					"items":{
						"type":"object",
						"properties":{
							"description":{"type":"string"},
							"status":{"type":"string","enum":["pending","in_progress","completed","blocked"]},
							"blocker_reason":{"type":"string"}
						},
						"required":["description"]
					}
				}
			},
			"required":["steps"]
		}`),
	}
}

func (t PlanWrite) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.Planning == nil || strings.TrimSpace(t.SessionID) == "" {
		return Result{OK: false, Output: "planning service or session unavailable"}, nil
	}
	var in struct {
		Steps []planning.StepInput `json:"steps"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	steps, err := t.Planning.SetPlan(ctx, t.SessionID, in.Steps)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Saved plan with %d steps.", len(steps))}, nil
}

type PlanUpdate struct {
	Planning  *planning.Service
	SessionID string
}

func (PlanUpdate) Spec() ToolSpec {
	return ToolSpec{
		Name:        "plan_update",
		Description: "Update one step in the current session plan after meaningful progress or when a blocker appears.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"step_index":{"type":"integer","minimum":0},
				"status":{"type":"string","enum":["pending","in_progress","completed","blocked"]},
				"blocker_reason":{"type":"string"}
			},
			"required":["step_index","status"]
		}`),
	}
}

func (t PlanUpdate) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.Planning == nil || strings.TrimSpace(t.SessionID) == "" {
		return Result{OK: false, Output: "planning service or session unavailable"}, nil
	}
	var in struct {
		StepIndex     int64  `json:"step_index"`
		Status        string `json:"status"`
		BlockerReason string `json:"blocker_reason"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	steps, err := t.Planning.UpdateStep(ctx, t.SessionID, in.StepIndex, in.Status, in.BlockerReason)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Updated plan step %d. Plan has %d steps.", in.StepIndex, len(steps))}, nil
}
