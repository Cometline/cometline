package contract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	loadOnce sync.Once
	cached   *openapi3.T
	loadErr  error
)

// OpenAPI loads and validates cometmind/openapi.yaml.
func OpenAPI() (*openapi3.T, error) {
	loadOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			loadErr = fmt.Errorf("runtime.Caller failed")
			return
		}
		specPath := filepath.Join(filepath.Dir(file), "..", "..", "openapi.yaml")
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		cached, loadErr = loader.LoadFromFile(specPath)
		if loadErr != nil {
			return
		}
		loadErr = cached.Validate(loader.Context)
	})
	return cached, loadErr
}

// ValidateStreamEventJSON checks one SSE frame body against StreamEvent.
func ValidateStreamEventJSON(raw []byte) error {
	doc, err := OpenAPI()
	if err != nil {
		return err
	}
	schemaRef := doc.Components.Schemas["StreamEvent"]
	if schemaRef == nil || schemaRef.Value == nil {
		return fmt.Errorf("StreamEvent schema missing from openapi.yaml")
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	return schemaRef.Value.VisitJSON(payload)
}
