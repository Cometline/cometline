package agent

import (
	"encoding/json"
	"fmt"
	"reflect"

	cometsdk "github.com/cometline/comet-sdk"
)

// DoomLoopThreshold matches OpenCode: the Nth consecutive identical tool+input
// is treated as a stuck loop.
const DoomLoopThreshold = 3

// ToolFingerprint is a name + canonical JSON argument pair.
type ToolFingerprint struct {
	Name  string
	Input any
}

// FingerprintTool unmarshals tool arguments so key order does not matter.
func FingerprintTool(name string, raw json.RawMessage) ToolFingerprint {
	var v any
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		v = string(raw)
	}
	return ToolFingerprint{Name: name, Input: v}
}

// SameTool reports whether two fingerprints are the same tool and arguments.
func SameTool(a, b ToolFingerprint) bool {
	return a.Name == b.Name && reflect.DeepEqual(a.Input, b.Input)
}

// IsDoomLoop reports whether the trailing window is threshold copies of one call.
func IsDoomLoop(recent []ToolFingerprint, threshold int) bool {
	if threshold <= 0 {
		threshold = DoomLoopThreshold
	}
	if len(recent) < threshold {
		return false
	}
	window := recent[len(recent)-threshold:]
	first := window[0]
	for _, fp := range window[1:] {
		if !SameTool(first, fp) {
			return false
		}
	}
	return true
}

func toolInputsEqual(a, b json.RawMessage) bool {
	return SameTool(FingerprintTool("", a), FingerprintTool("", b))
}

func toolFingerprintsSinceLastUser(msgs []cometsdk.Message) []ToolFingerprint {
	var out []ToolFingerprint
	for _, msg := range msgs {
		if msg.Role == cometsdk.RoleUser {
			out = out[:0]
			continue
		}
		if msg.Role != cometsdk.RoleAssistant {
			continue
		}
		for _, block := range msg.Content {
			tc, ok := block.(cometsdk.ToolCallBlock)
			if !ok {
				continue
			}
			out = append(out, FingerprintTool(tc.Name, tc.Input))
		}
	}
	return out
}

func doomLoopToolResult(name string) string {
	return fmt.Sprintf(
		"Repeated identical tool call blocked (doom loop). You already called %q %d times with the same arguments. Do not repeat this call. Use the previous tool result or try a different approach.",
		name,
		DoomLoopThreshold,
	)
}
