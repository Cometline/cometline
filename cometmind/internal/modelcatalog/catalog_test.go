package modelcatalog_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/modelcatalog"
)

func TestMain(m *testing.M) {
	modelcatalog.ResetCacheForTest()
	code := m.Run()
	modelcatalog.ResetCacheForTest()
	os.Exit(code)
}

func loadFixture(t *testing.T) {
	t.Helper()
	modelcatalog.ResetCacheForTest()
	data, err := os.ReadFile(filepath.Join("testdata", "models-dev-snippet.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := modelcatalog.LoadFromJSONForTest(data); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
}

func TestResolveLimitsCatalogHit(t *testing.T) {
	loadFixture(t)

	got := modelcatalog.ResolveLimits("anthropic", "anthropic", "claude-opus-4-1")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	if got.Context != 200_000 {
		t.Fatalf("context = %d, want 200000", got.Context)
	}
	if got.Output != 32_000 {
		t.Fatalf("output = %d, want 32000", got.Output)
	}
	if !got.VisionKnown || !got.Vision {
		t.Fatalf("vision = known=%v value=%v, want known vision", got.VisionKnown, got.Vision)
	}
	if len(got.InputModalities) != 3 || got.InputModalities[0] != "text" || got.InputModalities[1] != "image" || got.InputModalities[2] != "pdf" {
		t.Fatalf("input modalities = %v, want [text image pdf]", got.InputModalities)
	}
}

func TestResolveLimitsVisionFalse(t *testing.T) {
	loadFixture(t)

	got := modelcatalog.ResolveLimits("openai", "openai", "o3-mini")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	if !got.VisionKnown {
		t.Fatal("expected VisionKnown")
	}
	if got.Vision {
		t.Fatal("o3-mini should not support vision")
	}
	if len(got.InputModalities) != 1 || got.InputModalities[0] != "text" {
		t.Fatalf("input modalities = %v, want [text]", got.InputModalities)
	}
}

func TestResolveLimitsCodexMapsToOpenAI(t *testing.T) {
	loadFixture(t)

	got := modelcatalog.ResolveLimits("codex", "codex", "gpt-5-codex")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	if got.Context != 400_000 {
		t.Fatalf("context = %d, want 400000", got.Context)
	}
}

func TestResolveLimitsOpencodeGoMapsToOpencode(t *testing.T) {
	loadFixture(t)

	got := modelcatalog.ResolveLimits("opencode-go", "opencode-go", "claude-opus-4-8")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	if got.Context != 1_000_000 {
		t.Fatalf("context = %d, want 1000000", got.Context)
	}
}

func TestResolveLimitsOpencodeGoPrefersOpencodeGoBucket(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)
	payload := []byte(`{
		"opencode": {
			"id": "opencode",
			"models": {
				"qwen3.5-plus": {
					"id": "qwen3.5-plus",
					"modalities": {"input": ["text", "image", "video"]},
					"limit": {"context": 262144, "output": 65536}
				}
			}
		},
		"opencode-go": {
			"id": "opencode-go",
			"models": {
				"qwen3.7-max": {
					"id": "qwen3.7-max",
					"modalities": {"input": ["text"]},
					"limit": {"context": 1000000, "output": 65536}
				},
				"qwen3.7-plus": {
					"id": "qwen3.7-plus",
					"modalities": {"input": ["text", "image", "video"]},
					"limit": {"context": 1000000, "output": 65536}
				},
				"qwen3.5-plus": {
					"id": "qwen3.5-plus",
					"modalities": {"input": ["text", "image"]},
					"limit": {"context": 262144, "output": 65536}
				}
			}
		}
	}`)
	if err := modelcatalog.LoadFromJSONForTest(payload); err != nil {
		t.Fatal(err)
	}

	max := modelcatalog.ResolveLimits("opencode-go", "opencode-go", "qwen3.7-max")
	if max.Source != modelcatalog.SourceCatalog || max.Context != 1_000_000 {
		t.Fatalf("qwen3.7-max = %+v, want opencode-go catalog hit", max)
	}
	if !max.VisionKnown || max.Vision || len(max.InputModalities) != 1 || max.InputModalities[0] != "text" {
		t.Fatalf("qwen3.7-max modalities = %+v", max)
	}

	plus := modelcatalog.ResolveLimits("opencode-go", "opencode-go", "qwen3.7-plus")
	if plus.Source != modelcatalog.SourceCatalog || !plus.Vision || !hasAll(plus.InputModalities, "text", "image", "video") {
		t.Fatalf("qwen3.7-plus = %+v", plus)
	}

	// Still in opencode-go bucket when both providers list the id.
	shared := modelcatalog.ResolveLimits("opencode-go", "opencode-go", "qwen3.5-plus")
	if shared.Context != 262144 || !hasAll(shared.InputModalities, "text", "image") || hasAll(shared.InputModalities, "video") {
		t.Fatalf("shared should prefer opencode-go entry = %+v", shared)
	}

	// Zen method can fall through to opencode-go for models only listed there.
	zen := modelcatalog.ResolveLimits("opencode", "opencode", "qwen3.7-plus")
	if zen.Source != modelcatalog.SourceCatalog || zen.Context != 1_000_000 || !zen.Vision {
		t.Fatalf("opencode sibling fallback = %+v", zen)
	}
}

func hasAll(got []string, want ...string) bool {
	set := map[string]struct{}{}
	for _, g := range got {
		set[g] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}

func TestResolveLimitsMissAndCustomFallback(t *testing.T) {
	loadFixture(t)

	miss := modelcatalog.ResolveLimits("anthropic", "anthropic", "not-a-real-model")
	if miss.Source != modelcatalog.SourceFallback || miss.Context != modelcatalog.DefaultContext {
		t.Fatalf("miss = %+v, want fallback 128k", miss)
	}
	if miss.VisionKnown {
		t.Fatal("fallback must not claim VisionKnown")
	}

	custom := modelcatalog.ResolveLimits("openai-compatible", "local", "llama3")
	if custom.Source != modelcatalog.SourceFallback || custom.Context != modelcatalog.DefaultContext {
		t.Fatalf("custom = %+v, want fallback 128k", custom)
	}
	if custom.VisionKnown {
		t.Fatal("custom fallback must not claim VisionKnown")
	}

	ollama := modelcatalog.ResolveLimits("ollama", "ollama", "totally-local-unknown")
	if ollama.Source != modelcatalog.SourceFallback {
		t.Fatalf("ollama source = %q, want fallback", ollama.Source)
	}
	if ollama.VisionKnown {
		t.Fatal("ollama fallback must not claim VisionKnown")
	}
}

func TestResolveLimitsOpenAICompatibleScansCatalog(t *testing.T) {
	loadFixture(t)

	got := modelcatalog.ResolveLimits("openai-compatible", "company-gateway", "claude-opus-4-1")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	if got.Context != 200_000 {
		t.Fatalf("context = %d, want 200000", got.Context)
	}
	if !got.VisionKnown || !got.Vision {
		t.Fatalf("vision = known=%v value=%v", got.VisionKnown, got.Vision)
	}
	if len(got.InputModalities) < 2 || got.InputModalities[0] != "text" {
		t.Fatalf("modalities = %v", got.InputModalities)
	}

	prefixed := modelcatalog.ResolveLimits("openai-compatible", "gateway", "anthropic/claude-opus-4-1")
	if prefixed.Source != modelcatalog.SourceCatalog || prefixed.Context != 200_000 {
		t.Fatalf("prefixed = %+v, want catalog hit", prefixed)
	}
}

func TestResolveLimitsOpenAICompatibleGatewayAliases(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)
	payload := []byte(`{
		"anthropic": {
			"id": "anthropic",
			"models": {
				"claude-opus-4": {
					"id": "claude-opus-4",
					"modalities": {"input": ["text", "image", "pdf"]},
					"limit": {"context": 200000, "output": 32000}
				},
				"claude-sonnet-4": {
					"id": "claude-sonnet-4",
					"modalities": {"input": ["text", "image", "pdf"]},
					"limit": {"context": 1000000, "output": 64000}
				}
			}
		},
		"openrouter": {
			"id": "openrouter",
			"models": {
				"anthropic/claude-3-haiku": {
					"id": "anthropic/claude-3-haiku",
					"modalities": {"input": ["text", "image"]},
					"limit": {"context": 200000, "output": 4096}
				},
				"claude-3.5-haiku": {
					"id": "claude-3.5-haiku",
					"modalities": {"input": ["text", "image"]},
					"limit": {"context": 200000, "output": 8192}
				}
			}
		}
	}`)
	if err := modelcatalog.LoadFromJSONForTest(payload); err != nil {
		t.Fatal(err)
	}

	opus := modelcatalog.ResolveLimits("openai-compatible", "gateway", "claude-4-opus")
	if opus.Source != modelcatalog.SourceCatalog || opus.Context != 200_000 || !opus.Vision {
		t.Fatalf("claude-4-opus alias = %+v", opus)
	}

	sonnet := modelcatalog.ResolveLimits("openai-compatible", "gateway", "claude-4-sonnet")
	if sonnet.Source != modelcatalog.SourceCatalog || sonnet.Context != 1_000_000 || !sonnet.Vision {
		t.Fatalf("claude-4-sonnet alias = %+v", sonnet)
	}

	aws := modelcatalog.ResolveLimits("openai-compatible", "gateway", "claude-4-sonnet-aws")
	if aws.Source != modelcatalog.SourceCatalog || aws.Context != 1_000_000 {
		t.Fatalf("claude-4-sonnet-aws = %+v", aws)
	}

	haiku := modelcatalog.ResolveLimits("openai-compatible", "gateway", "claude-3-haiku")
	if haiku.Source != modelcatalog.SourceCatalog || haiku.Context != 200_000 || !haiku.Vision {
		t.Fatalf("claude-3-haiku org-suffix match = %+v", haiku)
	}

	exact := modelcatalog.ResolveLimits("openai-compatible", "gateway", "claude-3.5-haiku")
	if exact.Source != modelcatalog.SourceCatalog || exact.Context != 200_000 {
		t.Fatalf("claude-3.5-haiku = %+v", exact)
	}
}

func TestResolveLimitsOllamaMatchesCatalogVariants(t *testing.T) {
	loadFixture(t)

	tagged := modelcatalog.ResolveLimits("ollama", "ollama", "o3-mini:latest")
	if tagged.Source != modelcatalog.SourceCatalog {
		t.Fatalf("tagged source = %q, want catalog", tagged.Source)
	}
	if tagged.Context != 200_000 {
		t.Fatalf("tagged context = %d, want 200000", tagged.Context)
	}
	if !tagged.VisionKnown || tagged.Vision {
		t.Fatalf("o3-mini vision = known=%v value=%v", tagged.VisionKnown, tagged.Vision)
	}

	// Prefer the ollama provider bucket when the same id exists there.
	modelcatalog.ResetCacheForTest()
	payload := []byte(`{
		"openai": {
			"id": "openai",
			"models": {
				"llama3.2": {
					"id": "llama3.2",
					"modalities": {"input": ["text"]},
					"limit": {"context": 128000, "output": 8000}
				}
			}
		},
		"ollama": {
			"id": "ollama",
			"models": {
				"llama3.2": {
					"id": "llama3.2",
					"modalities": {"input": ["text", "image"]},
					"limit": {"context": 131072, "output": 8192}
				}
			}
		}
	}`)
	if err := modelcatalog.LoadFromJSONForTest(payload); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(modelcatalog.ResetCacheForTest)

	got := modelcatalog.ResolveLimits("ollama", "ollama", "llama3.2:latest")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	if got.Context != 131_072 {
		t.Fatalf("context = %d, want ollama bucket 131072", got.Context)
	}
	if !got.VisionKnown || !got.Vision {
		t.Fatalf("vision = known=%v value=%v, want ollama image support", got.VisionKnown, got.Vision)
	}
}

func TestResolveLimitsInputModalitiesNormalizeDocument(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)
	payload := []byte(`{
		"anthropic": {
			"id": "anthropic",
			"models": {
				"doc-model": {
					"id": "doc-model",
					"modalities": {"input": ["text", "document", "file"]},
					"limit": {"context": 100000, "output": 8000}
				}
			}
		}
	}`)
	if err := modelcatalog.LoadFromJSONForTest(payload); err != nil {
		t.Fatal(err)
	}
	got := modelcatalog.ResolveLimits("anthropic", "anthropic", "doc-model")
	if !got.VisionKnown {
		t.Fatal("expected VisionKnown")
	}
	if got.Vision {
		t.Fatal("document-only should not set vision")
	}
	if len(got.InputModalities) != 2 || got.InputModalities[0] != "text" || got.InputModalities[1] != "pdf" {
		t.Fatalf("modalities = %v, want [text pdf]", got.InputModalities)
	}
}

func TestResolveLimitsFetchesAndCaches(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "models-dev.json")
	modelcatalog.SetCachePathForTest(cachePath)

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		data, err := os.ReadFile(filepath.Join("testdata", "models-dev-snippet.json"))
		if err != nil {
			t.Errorf("fixture: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	modelcatalog.SetFetchURLForTest(srv.URL)
	t.Cleanup(func() {
		modelcatalog.SetFetchURLForTest(modelcatalog.APIURL)
		modelcatalog.ResetCacheForTest()
	})

	first := modelcatalog.ResolveLimits("anthropic", "anthropic", "claude-opus-4-1")
	if first.Source != modelcatalog.SourceCatalog {
		t.Fatalf("first = %+v", first)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected disk cache: %v", err)
	}

	modelcatalog.ResetCacheForTest()
	modelcatalog.SetCachePathForTest(cachePath)
	second := modelcatalog.ResolveLimits("anthropic", "anthropic", "claude-opus-4-1")
	if second.Source != modelcatalog.SourceCatalog {
		t.Fatalf("second = %+v", second)
	}
	if hits != 1 {
		t.Fatalf("hits after cache = %d, want 1", hits)
	}
	if !second.VisionKnown || !second.Vision {
		t.Fatalf("cached vision = known=%v value=%v", second.VisionKnown, second.Vision)
	}
	if len(second.InputModalities) < 2 || second.InputModalities[1] != "image" {
		t.Fatalf("cached modalities = %v, want image preserved", second.InputModalities)
	}
}

func TestResolveCostStaleCacheDoesNotRefetchOnFailure(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	dir := t.TempDir()
	modelcatalog.SetCachePathForTest(filepath.Join(dir, "models-dev.json"))
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	modelcatalog.SetNowForTest(func() time.Time { return now })
	t.Cleanup(func() {
		modelcatalog.SetNowForTest(nil)
		modelcatalog.SetFetchURLForTest(modelcatalog.APIURL)
		modelcatalog.ResetCacheForTest()
	})

	data, err := os.ReadFile(filepath.Join("testdata", "models-dev-snippet.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := modelcatalog.LoadFromJSONForTest(data); err != nil {
		t.Fatal(err)
	}

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	modelcatalog.SetFetchURLForTest(srv.URL)

	now = now.Add(modelcatalog.CacheTTL + time.Minute)
	if _, ok := modelcatalog.ResolveCost("anthropic", "claude-opus-4-1"); !ok {
		t.Fatal("expected stale catalog cost after failed refresh")
	}
	if _, ok := modelcatalog.ResolveCost("anthropic", "claude-opus-4-1"); !ok {
		t.Fatal("expected stale catalog cost on second lookup")
	}
	if hits != 1 {
		t.Fatalf("remote fetches=%d, want 1", hits)
	}
}

func TestReadDiskCacheRejectsLegacyWithoutModalities(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "models-dev.json")
	modelcatalog.SetCachePathForTest(cachePath)
	t.Cleanup(modelcatalog.ResetCacheForTest)

	// Legacy shape: limits only (what older CometMind wrote).
	legacy := []byte(`{
		"anthropic": {
			"id": "anthropic",
			"models": {
				"claude-opus-4-1": {
					"id": "claude-opus-4-1",
					"limit": {"context": 200000, "output": 32000}
				}
			}
		}
	}`)
	if err := os.WriteFile(cachePath, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		data, err := os.ReadFile(filepath.Join("testdata", "models-dev-snippet.json"))
		if err != nil {
			t.Errorf("fixture: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	modelcatalog.SetFetchURLForTest(srv.URL)
	t.Cleanup(func() { modelcatalog.SetFetchURLForTest(modelcatalog.APIURL) })

	got := modelcatalog.ResolveLimits("anthropic", "anthropic", "claude-opus-4-1")
	if got.Source != modelcatalog.SourceCatalog || !got.Vision {
		t.Fatalf("got = %+v, want refetched catalog vision", got)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1 (legacy cache must be ignored)", hits)
	}
}

func loadProtocolFixture(t *testing.T) {
	t.Helper()
	modelcatalog.ResetCacheForTest()
	data, err := os.ReadFile(filepath.Join("testdata", "models-dev-protocol-snippet.json"))
	if err != nil {
		t.Fatalf("read protocol fixture: %v", err)
	}
	if err := modelcatalog.LoadFromJSONForTest(data); err != nil {
		t.Fatalf("load protocol fixture: %v", err)
	}
	t.Cleanup(modelcatalog.ResetCacheForTest)
}

func TestResolveProviderMetadataModelOverrideWins(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveProviderMetadata("opencode-go", "opencode-go", "gpt-5.6-luna")
	if got.NPM != modelcatalog.NPMOpenAI {
		t.Fatalf("luna npm = %q, want %q", got.NPM, modelcatalog.NPMOpenAI)
	}
	if got.API != "https://opencode.ai/zen/go/v1" {
		t.Fatalf("luna api = %q, want provider-level api", got.API)
	}
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
}

func TestResolveProviderMetadataProviderLevelFallback(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveProviderMetadata("opencode-go", "opencode-go", "deepseek-v4-flash")
	if got.NPM != modelcatalog.DefaultProtocolNPM {
		t.Fatalf("deepseek npm = %q, want provider-level %q", got.NPM, modelcatalog.DefaultProtocolNPM)
	}
}

func TestRequiresEmptyReasoningContentReplayForDeepSeekFamily(t *testing.T) {
	for _, modelID := range []string{"deepseek-v4-flash", "deepseek-v4-pro", "deepseek-reasoner", "deepseek-r1"} {
		if !modelcatalog.RequiresEmptyReasoningContentReplay("opencode-go", "opencode-go", modelID) {
			t.Fatalf("%s should preserve empty reasoning_content", modelID)
		}
	}
	if modelcatalog.RequiresEmptyReasoningContentReplay("opencode-go", "opencode-go", "gpt-5.6-luna") {
		t.Fatal("gpt-5.6-luna should not preserve empty reasoning_content")
	}
	if modelcatalog.RequiresEmptyReasoningContentReplay("openai-compatible", "custom", "deepseek-v4-flash") {
		t.Fatal("unscoped custom gateway should not enable the DeepSeek policy")
	}
}

func TestResolveProviderMetadataAnthropicProtocol(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveProviderMetadata("opencode-go", "opencode-go", "qwen3.7-plus")
	if got.NPM != modelcatalog.NPMAnthropic {
		t.Fatalf("qwen3.7-plus npm = %q, want %q", got.NPM, modelcatalog.NPMAnthropic)
	}
}

func TestResolveProviderMetadataSiblingFallback(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveProviderMetadata("opencode-go", "opencode-go", "zen-only-model")
	if got.NPM != modelcatalog.NPMOpenAI {
		t.Fatalf("zen-only npm = %q, want %q via sibling bucket", got.NPM, modelcatalog.NPMOpenAI)
	}
	if got.API != "https://opencode.ai/zen/v1" {
		t.Fatalf("zen-only api = %q, want opencode bucket api", got.API)
	}
}

func TestResolveProviderMetadataMissFallsBackToDefault(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveProviderMetadata("opencode-go", "opencode-go", "not-a-real-model")
	if got.NPM != modelcatalog.DefaultProtocolNPM {
		t.Fatalf("miss npm = %q, want default", got.NPM)
	}
	if got.Source != modelcatalog.SourceFallback {
		t.Fatalf("miss source = %q, want fallback", got.Source)
	}

	empty := modelcatalog.ResolveProviderMetadata("opencode-go", "opencode-go", "  ")
	if empty.Source != modelcatalog.SourceFallback {
		t.Fatalf("empty source = %q, want fallback", empty.Source)
	}
}

func TestResolveProviderMetadataUnscopedAlwaysDefaults(t *testing.T) {
	loadProtocolFixture(t)

	// Custom gateways must never be switched to Responses by catalog metadata.
	got := modelcatalog.ResolveProviderMetadata("openai-compatible", "company-gateway", "gpt-5.6-luna")
	if got.NPM != modelcatalog.DefaultProtocolNPM {
		t.Fatalf("unscoped npm = %q, want default", got.NPM)
	}
	if got.Source != modelcatalog.SourceFallback {
		t.Fatalf("unscoped source = %q, want fallback", got.Source)
	}
}

func TestResolveReasoningOptionsEffort(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveReasoningOptions("opencode-go", "opencode-go", "gpt-5.6-luna")
	if got.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", got.Source)
	}
	want := []string{"none", "low", "medium", "high", "xhigh", "max"}
	if len(got.Effort) != len(want) {
		t.Fatalf("effort = %v, want %v", got.Effort, want)
	}
	for i := range want {
		if got.Effort[i] != want[i] {
			t.Fatalf("effort = %v, want %v", got.Effort, want)
		}
	}
	if got.Toggle {
		t.Fatal("luna must not advertise toggle")
	}
}

func TestResolveReasoningOptionsToggleAndBudget(t *testing.T) {
	loadProtocolFixture(t)

	got := modelcatalog.ResolveReasoningOptions("opencode-go", "opencode-go", "qwen3.7-plus")
	if got.Source != modelcatalog.SourceCatalog || !got.Toggle {
		t.Fatalf("qwen3.7-plus = %+v, want catalog toggle", got)
	}
	if got.BudgetMax == nil || *got.BudgetMax != 262144 {
		t.Fatalf("qwen3.7-plus budget max = %v, want 262144", got.BudgetMax)
	}
	if len(got.Effort) != 0 {
		t.Fatalf("qwen3.7-plus effort = %v, want none", got.Effort)
	}
}

func TestResolveReasoningOptionsMiss(t *testing.T) {
	loadProtocolFixture(t)

	miss := modelcatalog.ResolveReasoningOptions("opencode-go", "opencode-go", "not-a-real-model")
	if miss.Source != modelcatalog.SourceFallback || len(miss.Effort) != 0 || miss.Toggle {
		t.Fatalf("miss = %+v, want fallback", miss)
	}
}

func TestResolveReasoningOptionsCustomGatewayScansCatalog(t *testing.T) {
	loadProtocolFixture(t)

	unscoped := modelcatalog.ResolveReasoningOptions("openai-compatible", "company-gateway", "gpt-5.6-luna")
	if unscoped.Source != modelcatalog.SourceCatalog {
		t.Fatalf("source = %q, want catalog", unscoped.Source)
	}
	want := []string{"none", "low", "medium", "high", "xhigh", "max"}
	if !sameStrings(unscoped.Effort, want) {
		t.Fatalf("effort = %v, want %v", unscoped.Effort, want)
	}
}

func TestResolveReasoningOptionsCustomGatewayUsesCommonEffort(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)
	payload := []byte(`{
		"openai": {
			"id": "openai",
			"models": {
				"gpt-test": {
					"id": "gpt-test",
					"limit": {"context": 128000},
					"reasoning_options": [{"type": "effort", "values": ["none", "low", "medium", "high", "xhigh"]}]
				}
			}
		},
		"gateway": {
			"id": "gateway",
			"models": {
				"openai/gpt-test": {
					"id": "openai/gpt-test",
					"limit": {"context": 128000},
					"reasoning_options": [{"type": "effort", "values": ["low", "medium", "high"]}]
				}
			}
		},
		"toggle-only": {
			"id": "toggle-only",
			"models": {
				"gpt-test": {
					"id": "gpt-test",
					"limit": {"context": 128000},
					"reasoning_options": [{"type": "toggle"}]
				}
			}
		}
	}`)
	if err := modelcatalog.LoadFromJSONForTest(payload); err != nil {
		t.Fatal(err)
	}

	got := modelcatalog.ResolveReasoningOptions("openai-compatible", "company-proxy", "openai/gpt-test")
	want := []string{"low", "medium", "high"}
	if got.Source != modelcatalog.SourceCatalog || !sameStrings(got.Effort, want) {
		t.Fatalf("custom gateway = %+v, want effort %v", got, want)
	}
	if got.Toggle || got.BudgetMin != nil || got.BudgetMax != nil {
		t.Fatalf("custom gateway must only infer effort: %+v", got)
	}
}

func TestResolveReasoningOptionsCustomGatewayPrefersMatchingProviderID(t *testing.T) {
	modelcatalog.ResetCacheForTest()
	t.Cleanup(modelcatalog.ResetCacheForTest)
	payload := []byte(`{
		"openrouter": {
			"id": "openrouter",
			"models": {
				"openai/gpt-test": {
					"id": "openai/gpt-test",
					"limit": {"context": 128000},
					"reasoning_options": [{"type": "effort", "values": ["low", "high"]}]
				}
			}
		},
		"openai": {
			"id": "openai",
			"models": {
				"gpt-test": {
					"id": "gpt-test",
					"limit": {"context": 128000},
					"reasoning_options": [{"type": "effort", "values": ["low", "medium", "high"]}]
				}
			}
		}
	}`)
	if err := modelcatalog.LoadFromJSONForTest(payload); err != nil {
		t.Fatal(err)
	}

	got := modelcatalog.ResolveReasoningOptions("openai-compatible", "openrouter", "openai/gpt-test")
	want := []string{"low", "high"}
	if !sameStrings(got.Effort, want) {
		t.Fatalf("effort = %v, want provider-specific %v", got.Effort, want)
	}
}

func TestResolveCostFromCatalog(t *testing.T) {
	loadFixture(t)

	cost, ok := modelcatalog.ResolveCost("anthropic", "claude-opus-4-1")
	if !ok {
		t.Fatal("expected catalog cost")
	}
	if cost.Input != 15 || cost.Output != 75 || cost.CacheRead != 1.5 || cost.CacheWrite != 18.75 {
		t.Fatalf("cost = %+v", cost)
	}
	if _, ok := modelcatalog.ResolveCost("anthropic", "missing-model"); ok {
		t.Fatal("missing model should be unpriced")
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
