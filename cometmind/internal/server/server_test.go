package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/runstate"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/sqlite"
	"github.com/gin-gonic/gin"
)

type fakeRunner func(context.Context, session.AgentTurn, chan<- event.Event) error

func (f fakeRunner) Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
	return f(ctx, turn, ch)
}

type flushObserverRecorder struct {
	*httptest.ResponseRecorder
	onFlush func(body string)
}

func (r *flushObserverRecorder) Flush() {
	r.ResponseRecorder.Flush()
	if r.onFlush != nil {
		r.onFlush(r.Body.String())
	}
}

func newTestEngine(t *testing.T, newRunner RunnerFactory) (*gin.Engine, *session.Service, func()) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "cometmind-test.db")
	sqlDB, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}

	svc := session.New(sqlDB)
	engine, err := New(Deps{
		Config: &config.Config{
			DefaultProviderID: "test-provider",
			DefaultModelID:    "test-model",
			MaxSteps:          8,
			Skills: config.SkillsConfig{
				Enabled:         true,
				IncludeOpenCode: false,
				IncludeClaude:   false,
			},
		},
		Sessions:  svc,
		NewRunner: newRunner,
		Runs:      NewRunManager(runstate.New(sqlDB)),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return engine, svc, func() {
		_ = sqlDB.Close()
	}
}

func decodeJSON(t *testing.T, raw []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", string(raw), err)
	}
}

func mustJSON(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func parseSSEDataFrames(body string) [][]byte {
	var frames [][]byte
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		frames = append(frames, []byte(payload))
	}
	return frames
}
