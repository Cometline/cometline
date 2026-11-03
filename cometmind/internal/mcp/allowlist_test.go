package mcp

import "testing"

func TestToolExposeMode(t *testing.T) {
	if got := ToolExposeMode(nil); got != "all" {
		t.Fatalf("nil = %q, want all", got)
	}
	if got := ToolExposeMode([]string{}); got != "all" {
		t.Fatalf("empty = %q, want all", got)
	}
	if got := ToolExposeMode([]string{" " + DenyAllTools + " "}); got != "none" {
		t.Fatalf("deny-all = %q, want none", got)
	}
	if got := ToolExposeMode([]string{"deploy"}); got != "list" {
		t.Fatalf("subset = %q, want list", got)
	}
	if got := ToolExposeMode([]string{DenyAllTools, "deploy"}); got != "list" {
		t.Fatalf("marker plus name = %q, want list", got)
	}
}

func TestAllowedToolSetIgnoresDenyAll(t *testing.T) {
	if got := allowedToolSet([]string{DenyAllTools}); got != nil {
		t.Fatalf("deny-all set = %#v, want nil", got)
	}
	got := allowedToolSet([]string{" deploy ", ""})
	if _, ok := got["deploy"]; !ok || len(got) != 1 {
		t.Fatalf("subset = %#v", got)
	}
}
