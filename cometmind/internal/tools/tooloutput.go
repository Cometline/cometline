package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/paths"
	"github.com/oklog/ulid/v2"
)

const (
	toolOutputMaxPreviewRunes = 12000
	toolOutputHeadRunes       = 6000
	toolOutputTailRunes       = 4000
	toolOutputRetention       = 7 * 24 * time.Hour
)

// boundToolOutput returns text unchanged when small enough; otherwise writes the
// full text under ~/.cometmind/tool-output/ and returns a head+tail preview plus path.
func boundToolOutput(text string) string {
	if len([]rune(text)) <= toolOutputMaxPreviewRunes {
		return text
	}
	path, err := writeToolOutputFile(text)
	preview := headTailPreview(text, toolOutputHeadRunes, toolOutputTailRunes)
	if err != nil {
		return preview + fmt.Sprintf(
			"\n\n(output truncated; failed to retain full output: %v)", err,
		)
	}
	return preview + fmt.Sprintf(
		"\n\n(output truncated to ~%d chars; full output: %s)",
		toolOutputMaxPreviewRunes, path,
	)
}

func writeToolOutputFile(text string) (string, error) {
	dir, err := toolOutputDir()
	if err != nil {
		return "", err
	}
	// Best-effort cleanup of old files; ignore errors.
	_ = cleanupToolOutputDir(dir, toolOutputRetention)

	name := "tool_" + strings.ToLower(ulid.Make().String()) + ".txt"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", err
	}
	return "@runtime/tool-output/" + filepath.Base(path), nil
}

func toolOutputDir() (string, error) {
	return paths.ToolOutputDir()
}

func headTailPreview(text string, headRunes, tailRunes int) string {
	runes := []rune(text)
	if len(runes) <= headRunes+tailRunes {
		return text
	}
	head := string(runes[:headRunes])
	tail := string(runes[len(runes)-tailRunes:])
	omitted := len(runes) - headRunes - tailRunes
	return head + fmt.Sprintf("\n\n... [%d characters omitted] ...\n\n", omitted) + tail
}

func cleanupToolOutputDir(dir string, maxAge time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "tool_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}
