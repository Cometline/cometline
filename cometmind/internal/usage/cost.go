package usage

import (
	"strings"

	"github.com/cometline/cometmind/internal/modelcatalog"
)

// BilledUsage is the disjoint token classes used for totals and estimates.
type BilledUsage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
}

// Tokens is the non-overlapping display total.
func (b BilledUsage) Tokens() int {
	return b.Input + b.Output + b.CacheRead + b.CacheWrite
}

func clampNonNeg(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// inputIncludesCache reports whether provider input totals already contain
// cache read (and sometimes cache write). Unknown providers default to exclusive
// so a gateway cannot underflow billed input to zero.
//
// This table is the billing contract. HTTP responses expose billed_input so
// the desktop client must not copy these IDs.
func inputIncludesCache(providerID string) bool {
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "openai", "xai", "codex", "opencode-go", "opencode", "google":
		return true
	default:
		return false
	}
}

// NormalizeUsage splits a provider usage payload into disjoint billed classes.
// Exclusive providers (Anthropic) keep input as-is. Inclusive providers
// subtract cache from input so cache is not charged at the full input rate.
func NormalizeUsage(providerID string, input, output, cacheRead, cacheWrite int) BilledUsage {
	input = clampNonNeg(input)
	output = clampNonNeg(output)
	cacheRead = clampNonNeg(cacheRead)
	cacheWrite = clampNonNeg(cacheWrite)
	fresh := input
	if inputIncludesCache(providerID) {
		if cacheRead+cacheWrite <= input {
			fresh = input - cacheRead - cacheWrite
		} else {
			fresh = clampNonNeg(input - cacheRead)
		}
	}
	return BilledUsage{
		Input:      fresh,
		Output:     output,
		CacheRead:  cacheRead,
		CacheWrite: cacheWrite,
	}
}

// EstimateUSD applies models.dev per-1M-token rates to disjoint token classes.
func EstimateUSD(cost modelcatalog.Cost, input, output, cacheRead, cacheWrite int) float64 {
	if !cost.Found {
		return 0
	}
	return (float64(input)*cost.Input +
		float64(output)*cost.Output +
		float64(cacheRead)*cost.CacheRead +
		float64(cacheWrite)*cost.CacheWrite) / 1_000_000
}
