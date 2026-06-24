package memory

import (
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestTryExplicitRememberChinese(t *testing.T) {
	msgs := []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "記住我愛炸醬麵"}},
	}}
	pm, ok := tryExplicitRemember(msgs)
	if !ok {
		t.Fatal("expected explicit remember match")
	}
	if pm.Content != "我愛炸醬麵" {
		t.Fatalf("content = %q, want %q", pm.Content, "我愛炸醬麵")
	}
	if !pm.ShouldSave || pm.Kind != "preference" {
		t.Fatalf("unexpected proposal: %+v", pm)
	}
}

func TestTryExplicitRememberEnglish(t *testing.T) {
	msgs := []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Remember that I prefer dark mode."}},
	}}
	pm, ok := tryExplicitRemember(msgs)
	if !ok {
		t.Fatal("expected explicit remember match")
	}
	if pm.Content != "I prefer dark mode" {
		t.Fatalf("content = %q", pm.Content)
	}
}

func TestTryExplicitRememberNoMatch(t *testing.T) {
	msgs := []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is zhajiangmian?"}},
	}}
	if _, ok := tryExplicitRemember(msgs); ok {
		t.Fatal("expected no explicit remember match")
	}
}
