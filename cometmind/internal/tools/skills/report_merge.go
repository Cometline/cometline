package skills

import (
	"context"
	"encoding/json"

	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// ReportMergedSkills records which self-improvement skills a merge absorbed.
// It does not delete or archive files.
type ReportMergedSkills struct{}

func (ReportMergedSkills) Spec() ToolSpec {
	return ToolSpec{
		Name:        "report_merged_skills",
		Description: "After overwriting the surviving self-improvement skill, name the source skills whose procedures were fully absorbed. Do not use this to delete or archive files.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"target": {"type": "string"},
				"absorbed": {"type": "array", "items": {"type": "string"}}
			},
			"required": ["target", "absorbed"]
		}`),
	}
}

func (ReportMergedSkills) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Target   *string  `json:"target"`
		Absorbed []string `json:"absorbed"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	if _, bad, ok := toolkit.RequiredTrimmedString(in.Target, "target"); !ok {
		return bad, nil
	}
	return Result{OK: true, Output: "merge report recorded"}, nil
}
