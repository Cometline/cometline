package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleListMCPCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &App{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/catalog", nil)
	app.handleListMCPCatalog(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Entries []struct {
			ID   string `json:"id"`
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range body.Entries {
		ids[e.ID] = true
	}
	for _, want := range []string{"atlassian", "heptabase", "linear", "sentry"} {
		if !ids[want] {
			t.Fatalf("missing catalog entry %q", want)
		}
	}
}
