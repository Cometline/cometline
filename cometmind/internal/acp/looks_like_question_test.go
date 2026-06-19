package acp

import "testing"

func TestLooksLikeQuestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want bool
	}{
		{"Which branch should I use?", true},
		{"請問你想在哪個分支上工作？", true},
		{"Done. All tests passed.", false},
		{"Let me run the tests now.", false},
		{"你要用 main 还是 feat/foo？", true},
	}

	for _, tc := range tests {
		if got := looksLikeQuestion(tc.text); got != tc.want {
			t.Errorf("looksLikeQuestion(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
