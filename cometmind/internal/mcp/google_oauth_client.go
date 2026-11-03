package mcp

import "os"

// Build-time values come from -ldflags. A local build can still supply the
// same names through its environment when the binary was not stamped.
var (
	googleOAuthClientID     = ""
	googleOAuthClientSecret = ""
)

func init() {
	if googleOAuthClientID == "" {
		googleOAuthClientID = os.Getenv("COMETLINE_GOOGLE_OAUTH_CLIENT_ID")
	}
	if googleOAuthClientSecret == "" {
		googleOAuthClientSecret = os.Getenv("COMETLINE_GOOGLE_OAUTH_CLIENT_SECRET")
	}
}
