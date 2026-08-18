package usage

import "github.com/cometline/cometmind/internal/modelcatalog"

// EstimateUSD applies models.dev per-1M-token rates.
func EstimateUSD(cost modelcatalog.Cost, input, output, cacheRead, cacheWrite int) float64 {
	if !cost.Found {
		return 0
	}
	return (float64(input)*cost.Input +
		float64(output)*cost.Output +
		float64(cacheRead)*cost.CacheRead +
		float64(cacheWrite)*cost.CacheWrite) / 1_000_000
}
