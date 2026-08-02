package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/session"
)

func TestSpawnGeneralAgentPlanModeRejectsCoding(t *testing.T) {
	tool := SpawnGeneralAgent{AgentMode: session.AgentModePlan}

	// Coding children must be rejected before any session work happens; the
	// tool has nil deps, so reaching the "not configured" path proves the gate.
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"fix bug","kind":"coding"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "not allowed in Plan mode") {
		t.Fatalf("coding result = %+v, want Plan rejection", res)
	}
}

func TestSpawnGeneralAgentPlanModeSpecOnlyAdvertisesResearch(t *testing.T) {
	spec := (SpawnGeneralAgent{AgentMode: session.AgentModePlan}).Spec()
	if strings.Contains(strings.ToLower(spec.Description), "coding") {
		t.Fatalf("Plan description advertises coding: %q", spec.Description)
	}
	var schema struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("decode Plan schema: %v", err)
	}
	kind := schema.Properties["kind"]
	if len(kind.Enum) != 1 || kind.Enum[0] != "research" {
		t.Fatalf("Plan kind enum = %v, want [research]", kind.Enum)
	}
}

func TestSpawnGeneralAgentAutoModeSpecAdvertisesCoding(t *testing.T) {
	spec := (SpawnGeneralAgent{AgentMode: session.AgentModeAuto}).Spec()
	if !strings.Contains(strings.ToLower(spec.Description), "kind=coding") {
		t.Fatalf("Auto description does not advertise coding: %q", spec.Description)
	}
}

func TestSpawnGeneralAgentPlanModeAllowsResearch(t *testing.T) {
	tool := SpawnGeneralAgent{AgentMode: session.AgentModePlan}

	// Research children pass the mode gate; with nil deps they fail later on
	// configuration, which proves the gate did not reject them.
	for _, kind := range []string{"", "research", "general"} {
		body := `{"task":"explore"}`
		if kind != "" {
			body = `{"task":"explore","kind":"` + kind + `"}`
		}
		res, err := tool.Execute(context.Background(), json.RawMessage(body))
		if err != nil {
			t.Fatalf("Execute(kind=%q) error = %v", kind, err)
		}
		if strings.Contains(res.Output, "not allowed in Plan mode") {
			t.Fatalf("kind=%q was rejected by Plan gate: %+v", kind, res)
		}
		if !res.OK && !strings.Contains(res.Output, "not configured") {
			t.Fatalf("kind=%q result = %+v, want config failure after passing gate", kind, res)
		}
	}
}

func TestSpawnGeneralAgentAutoModeAllowsCoding(t *testing.T) {
	tool := SpawnGeneralAgent{AgentMode: session.AgentModeAuto}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"task":"fix bug","kind":"coding"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.Contains(res.Output, "not allowed in Plan mode") {
		t.Fatalf("auto mode should not reject coding: %+v", res)
	}
	if !res.OK && !strings.Contains(res.Output, "not configured") {
		t.Fatalf("coding result = %+v, want config failure after passing gate", res)
	}
}
