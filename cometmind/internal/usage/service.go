package usage

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
	"github.com/cometline/cometmind/internal/modelcatalog"
)

const (
	KindAgentStep        = "agent_step"
	KindMemoryExtract    = "memory_extract"
	KindMemoryUpdate     = "memory_update"
	KindMemoryCompaction = "memory_compaction"
	KindEmbedding        = "embedding"
	KindSkillSynthesis   = "skill_synthesis"
	GroupByModel         = "model"
	GroupByKind          = "kind"
	defaultPageLimit     = 50
	maxPageLimit         = 200
	MaxRangeDays         = 366
	// RetentionDays is how long usage_events are kept. Distinct from the
	// query-window cap so changing MaxRangeDays does not change deletion.
	RetentionDays  = 366
	maxTZOffsetMin = 14 * 60
)

// Event is one recorded model or embedding call.
type Event struct {
	WorkspaceID string
	SessionID   string
	ProviderID  string
	ModelID     string
	CallKind    string
	Usage       cometsdk.TokenUsage
}

// Recorded is a persisted usage row with estimated cost.
type Recorded struct {
	ID           string
	CreatedAt    int64
	WorkspaceID  string
	SessionID    string
	ProviderID   string
	ModelID      string
	CallKind     string
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	EstimatedUSD float64
	Priced       bool
}

// Bucket is an aggregated usage slice (model or kind).
type Bucket struct {
	Key          string
	ProviderID   string
	ModelID      string
	CallKind     string
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	Tokens       int
	EstimatedUSD float64
	Priced       bool
	Unpriced     int
}

// Totals is the dashboard KPI set.
type Totals struct {
	Tokens         int
	PricedTokens   int
	UnpricedTokens int
	EstimatedUSD   float64
}

// Summary is the dashboard header + breakdowns.
type Summary struct {
	From    int64
	To      int64
	Totals  Totals
	ByModel []Bucket
	ByKind  []Bucket
}

// SeriesPoint is one day in the cumulative stacked area.
type SeriesPoint struct {
	Date       string
	DayTokens  map[string]int
	Cumulative map[string]int
}

// Series is grouped daily usage for the chart.
type Series struct {
	GroupBy string
	Keys    []string
	Points  []SeriesPoint
}

// Page is a paginated event list.
type Page struct {
	Items  []Recorded
	Total  int64
	Limit  int
	Offset int
}

// Recorder writes one usage event. Memory and skills depend on this seam.
type Recorder interface {
	Record(ctx context.Context, ev Event) error
}

// Service persists usage events and builds dashboard aggregates.
type Service struct {
	q *db.Queries
}

func NewService(conn *sql.DB) *Service {
	return &Service{q: db.New(conn)}
}

func nullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func stringVal(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func workspaceArg(workspaceID string) any {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil
	}
	return workspaceID
}

// Record appends one usage event. Failures should be logged by callers that
// cannot fail the parent LLM path.
func (s *Service) Record(ctx context.Context, ev Event) error {
	if s == nil || s.q == nil {
		return nil
	}
	kind := strings.TrimSpace(ev.CallKind)
	if kind == "" {
		kind = KindAgentStep
	}
	if ev.Usage.InputTokens == 0 && ev.Usage.OutputTokens == 0 && ev.Usage.CacheRead == 0 && ev.Usage.CacheWrite == 0 {
		return nil
	}
	return s.q.InsertUsageEvent(ctx, db.InsertUsageEventParams{
		ID:           id.New(),
		CreatedAt:    time.Now().UnixMilli(),
		WorkspaceID:  nullString(ev.WorkspaceID),
		SessionID:    nullString(ev.SessionID),
		ProviderID:   strings.TrimSpace(ev.ProviderID),
		ModelID:      strings.TrimSpace(ev.ModelID),
		CallKind:     kind,
		InputTokens:  int64(ev.Usage.InputTokens),
		OutputTokens: int64(ev.Usage.OutputTokens),
		CacheRead:    int64(ev.Usage.CacheRead),
		CacheWrite:   int64(ev.Usage.CacheWrite),
	})
}

func (s *Service) eventsInRange(ctx context.Context, from, to int64, workspaceID string) ([]db.UsageEvent, error) {
	from, to = ClampRange(from, to)
	return s.q.ListUsageEventsInRange(ctx, db.ListUsageEventsInRangeParams{
		FromMs:      from,
		ToMs:        to,
		WorkspaceID: workspaceArg(workspaceID),
	})
}

// PurgeOlderThan deletes ledger rows older than the retained window.
func (s *Service) PurgeOlderThan(ctx context.Context, days int) (int, error) {
	if s == nil || s.q == nil {
		return 0, nil
	}
	if days <= 0 {
		days = RetentionDays
	}
	n, err := s.q.DeleteUsageEventsBefore(ctx, time.Now().AddDate(0, 0, -days).UnixMilli())
	return int(n), err
}

type costHit struct {
	cost modelcatalog.Cost
	ok   bool
}

// costCache resolves models.dev prices once per (provider, model) per request.
type costCache map[string]costHit

func newCostCache() costCache {
	return costCache{}
}

func (c costCache) price(providerID, modelID string, in, out, cacheRead, cacheWrite int) (usd float64, priced bool) {
	key := providerID + "\x00" + modelID
	hit, ok := c[key]
	if !ok {
		cost, found := modelcatalog.ResolveCost(providerID, modelID)
		hit = costHit{cost: cost, ok: found}
		c[key] = hit
	}
	if !hit.ok {
		return 0, false
	}
	return EstimateUSD(hit.cost, in, out, cacheRead, cacheWrite), true
}

func recordedFromRow(row db.UsageEvent, prices costCache) Recorded {
	in := int(row.InputTokens)
	out := int(row.OutputTokens)
	cacheRead := int(row.CacheRead)
	cacheWrite := int(row.CacheWrite)
	if prices == nil {
		prices = newCostCache()
	}
	usd, priced := prices.price(row.ProviderID, row.ModelID, in, out, cacheRead, cacheWrite)
	return Recorded{
		ID:           row.ID,
		CreatedAt:    row.CreatedAt,
		WorkspaceID:  stringVal(row.WorkspaceID),
		SessionID:    stringVal(row.SessionID),
		ProviderID:   row.ProviderID,
		ModelID:      row.ModelID,
		CallKind:     row.CallKind,
		InputTokens:  in,
		OutputTokens: out,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		EstimatedUSD: usd,
		Priced:       priced,
	}
}

func addTokens(b *Bucket, rec Recorded) {
	b.InputTokens += rec.InputTokens
	b.OutputTokens += rec.OutputTokens
	b.CacheRead += rec.CacheRead
	b.CacheWrite += rec.CacheWrite
	tokens := rec.InputTokens + rec.OutputTokens + rec.CacheRead + rec.CacheWrite
	b.Tokens += tokens
	if rec.Priced {
		b.EstimatedUSD += rec.EstimatedUSD
		b.Priced = true
	} else {
		b.Unpriced += tokens
	}
}

func sortBuckets(items []Bucket) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].EstimatedUSD == items[j].EstimatedUSD {
			return items[i].Tokens > items[j].Tokens
		}
		return items[i].EstimatedUSD > items[j].EstimatedUSD
	})
}

func accumulate(rows []db.UsageEvent) (Totals, []Bucket, []Bucket) {
	byModel := map[string]*Bucket{}
	byKind := map[string]*Bucket{}
	prices := newCostCache()
	var totals Totals
	for _, row := range rows {
		rec := recordedFromRow(row, prices)
		tokens := rec.InputTokens + rec.OutputTokens + rec.CacheRead + rec.CacheWrite
		totals.Tokens += tokens
		if rec.Priced {
			totals.PricedTokens += tokens
			totals.EstimatedUSD += rec.EstimatedUSD
		} else {
			totals.UnpricedTokens += tokens
		}
		modelKey := rec.ProviderID + "\x00" + rec.ModelID
		if byModel[modelKey] == nil {
			byModel[modelKey] = &Bucket{Key: rec.ModelID, ProviderID: rec.ProviderID, ModelID: rec.ModelID}
		}
		addTokens(byModel[modelKey], rec)
		if byKind[rec.CallKind] == nil {
			byKind[rec.CallKind] = &Bucket{Key: rec.CallKind, CallKind: rec.CallKind}
		}
		addTokens(byKind[rec.CallKind], rec)
	}
	models := make([]Bucket, 0, len(byModel))
	for _, b := range byModel {
		models = append(models, *b)
	}
	kinds := make([]Bucket, 0, len(byKind))
	for _, b := range byKind {
		kinds = append(kinds, *b)
	}
	sortBuckets(models)
	sortBuckets(kinds)
	return totals, models, kinds
}

func RangeMS(days int) int64 {
	if days <= 0 {
		days = MaxRangeDays
	}
	return int64(days) * 24 * 60 * 60 * 1000
}

// ClampRange keeps a query window inside MaxRangeDays by sliding `from` forward.
func ClampRange(from, to int64) (int64, int64) {
	if to <= from {
		return from, to
	}
	max := RangeMS(MaxRangeDays)
	if to-from > max {
		from = to - max
		if from < 0 {
			from = 0
		}
	}
	return from, to
}

func ClampTZOffsetMin(offsetMin int) int {
	if offsetMin > maxTZOffsetMin {
		return maxTZOffsetMin
	}
	if offsetMin < -maxTZOffsetMin {
		return -maxTZOffsetMin
	}
	return offsetMin
}

func (s *Service) Summary(ctx context.Context, from, to int64, workspaceID string) (Summary, error) {
	from, to = ClampRange(from, to)
	rows, err := s.eventsInRange(ctx, from, to, workspaceID)
	if err != nil {
		return Summary{}, err
	}
	totals, models, kinds := accumulate(rows)
	return Summary{From: from, To: to, Totals: totals, ByModel: models, ByKind: kinds}, nil
}

func dayLocation(offsetMin int) *time.Location {
	return time.FixedZone("usage", ClampTZOffsetMin(offsetMin)*60)
}

func dayKey(ms int64, offsetMin int) string {
	return time.UnixMilli(ms).In(dayLocation(offsetMin)).Format("2006-01-02")
}

func eachDay(from, to int64, offsetMin int) []string {
	loc := dayLocation(offsetMin)
	start := time.UnixMilli(from).In(loc)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	end := time.UnixMilli(to - 1).In(loc)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, loc)
	if end.Before(start) {
		end = start
	}
	var days []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		days = append(days, d.Format("2006-01-02"))
		if len(days) >= MaxRangeDays {
			break
		}
	}
	return days
}

func seriesKey(rec Recorded, groupBy string) string {
	if groupBy == GroupByKind {
		return rec.CallKind
	}
	if rec.ProviderID == "" {
		return rec.ModelID
	}
	if rec.ModelID == "" {
		return rec.ProviderID
	}
	return rec.ProviderID + "/" + rec.ModelID
}

func (s *Service) Series(ctx context.Context, from, to int64, workspaceID, groupBy string, tzOffsetMin int) (Series, error) {
	from, to = ClampRange(from, to)
	if groupBy != GroupByKind {
		groupBy = GroupByModel
	}
	rows, err := s.eventsInRange(ctx, from, to, workspaceID)
	if err != nil {
		return Series{}, err
	}
	days := eachDay(from, to, tzOffsetMin)
	dayTotals := map[string]map[string]int{}
	keySet := map[string]struct{}{}
	prices := newCostCache()
	for _, row := range rows {
		rec := recordedFromRow(row, prices)
		key := seriesKey(rec, groupBy)
		if key == "" {
			continue
		}
		keySet[key] = struct{}{}
		day := dayKey(row.CreatedAt, tzOffsetMin)
		if dayTotals[day] == nil {
			dayTotals[day] = map[string]int{}
		}
		dayTotals[day][key] += rec.InputTokens + rec.OutputTokens + rec.CacheRead + rec.CacheWrite
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cumulative := map[string]int{}
	points := make([]SeriesPoint, 0, len(days))
	for _, day := range days {
		dayTokens := map[string]int{}
		cum := map[string]int{}
		for _, key := range keys {
			n := 0
			if dayTotals[day] != nil {
				n = dayTotals[day][key]
			}
			dayTokens[key] = n
			cumulative[key] += n
			cum[key] = cumulative[key]
		}
		points = append(points, SeriesPoint{Date: day, DayTokens: dayTokens, Cumulative: cum})
	}
	return Series{GroupBy: groupBy, Keys: keys, Points: points}, nil
}

func (s *Service) List(ctx context.Context, from, to int64, workspaceID string, limit, offset int) (Page, error) {
	from, to = ClampRange(from, to)
	if limit <= 0 {
		limit = defaultPageLimit
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	total, err := s.q.CountUsageEventsInRange(ctx, db.CountUsageEventsInRangeParams{
		FromMs:      from,
		ToMs:        to,
		WorkspaceID: workspaceArg(workspaceID),
	})
	if err != nil {
		return Page{}, err
	}
	rows, err := s.q.ListUsageEventsPage(ctx, db.ListUsageEventsPageParams{
		FromMs:      from,
		ToMs:        to,
		WorkspaceID: workspaceArg(workspaceID),
		Limit:       int64(limit),
		Offset:      int64(offset),
	})
	if err != nil {
		return Page{}, err
	}
	items := make([]Recorded, 0, len(rows))
	prices := newCostCache()
	for _, row := range rows {
		items = append(items, recordedFromRow(row, prices))
	}
	return Page{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}
