package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/usage"
	"github.com/gin-gonic/gin"
)

func (a *App) handleGetUsageSummary(c *gin.Context) {
	if a.usage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage unavailable"})
		return
	}
	from, to, ok := parseUsageRange(c)
	if !ok {
		return
	}
	summary, err := a.usage.Summary(c.Request.Context(), from, to, c.Query("workspace_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"from":     summary.From,
		"to":       summary.To,
		"totals":   usageTotalsJSON(summary.Totals),
		"by_model": usageBucketsJSON(summary.ByModel),
		"by_kind":  usageBucketsJSON(summary.ByKind),
	})
}

func (a *App) handleGetUsageSeries(c *gin.Context) {
	if a.usage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage unavailable"})
		return
	}
	from, to, ok := parseUsageRange(c)
	if !ok {
		return
	}
	groupBy := c.DefaultQuery("group_by", usage.GroupByModel)
	tzOffsetMin, ok := parseUsageTZOffset(c)
	if !ok {
		return
	}
	series, err := a.usage.Series(c.Request.Context(), from, to, c.Query("workspace_id"), groupBy, tzOffsetMin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	points := make([]gin.H, 0, len(series.Points))
	for _, p := range series.Points {
		points = append(points, gin.H{
			"date":       p.Date,
			"day_tokens": p.DayTokens,
			"cumulative": p.Cumulative,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"group_by": series.GroupBy,
		"keys":     series.Keys,
		"points":   points,
	})
}

func (a *App) handleListUsageEvents(c *gin.Context) {
	if a.usage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage unavailable"})
		return
	}
	from, to, ok := parseUsageRange(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	page, err := a.usage.List(c.Request.Context(), from, to, c.Query("workspace_id"), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, gin.H{
			"id":            item.ID,
			"created_at":    item.CreatedAt,
			"workspace_id":  item.WorkspaceID,
			"session_id":    item.SessionID,
			"provider_id":   item.ProviderID,
			"model_id":      item.ModelID,
			"call_kind":     item.CallKind,
			"input_tokens":  item.InputTokens,
			"output_tokens": item.OutputTokens,
			"cache_read":    item.CacheRead,
			"cache_write":   item.CacheWrite,
			"estimated_usd": item.EstimatedUSD,
			"priced":        item.Priced,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"total":  page.Total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

func parseUsageRange(c *gin.Context) (from, to int64, ok bool) {
	now := time.Now()
	to = now.UnixMilli() + 1
	from = now.AddDate(0, 0, -7).UnixMilli()
	if raw := c.Query("from"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
			return 0, 0, false
		}
		from = n
	}
	if raw := c.Query("to"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
			return 0, 0, false
		}
		to = n
	}
	if to <= from {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to must be after from"})
		return 0, 0, false
	}
	from, to = usage.ClampRange(from, to)
	return from, to, true
}

func parseUsageTZOffset(c *gin.Context) (offsetMin int, ok bool) {
	raw := strings.TrimSpace(c.Query("tz_offset_min"))
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tz_offset_min"})
		return 0, false
	}
	return usage.ClampTZOffsetMin(n), true
}

func usageTotalsJSON(t usage.Totals) gin.H {
	return gin.H{
		"tokens":          t.Tokens,
		"priced_tokens":   t.PricedTokens,
		"unpriced_tokens": t.UnpricedTokens,
		"estimated_usd":   t.EstimatedUSD,
	}
}

func usageBucketsJSON(items []usage.Bucket) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"key":             item.Key,
			"provider_id":     item.ProviderID,
			"model_id":        item.ModelID,
			"call_kind":       item.CallKind,
			"input_tokens":    item.InputTokens,
			"output_tokens":   item.OutputTokens,
			"cache_read":      item.CacheRead,
			"cache_write":     item.CacheWrite,
			"tokens":          item.Tokens,
			"estimated_usd":   item.EstimatedUSD,
			"priced":          item.Priced,
			"unpriced_tokens": item.Unpriced,
		})
	}
	return out
}
