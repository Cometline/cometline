package tools

import (
	"strings"
	"testing"
)

func TestRunCommandTimeoutParam(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(t.Context(), []byte(`{"command":"sleep 2","timeout_sec":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "timed out") {
		t.Fatalf("result = %+v", res)
	}
}
