package xai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBorrowTokenReadsFreshSubscriptionSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XAI_HOME", home)

	token := testJWT(time.Now().Add(time.Hour))
	auth := authFile{
		AuthMode: "subscription",
		Tokens: authTokens{
			AccessToken:  token,
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		},
	}
	data, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "auth.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := BorrowToken(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatalf("BorrowToken() error = %v", err)
	}
	if got != token {
		t.Fatalf("token = %q, want %q", got, token)
	}
}

func TestBorrowTokenRejectsNonSubscriptionSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XAI_HOME", home)
	data := []byte(`{"auth_mode":"api","tokens":{"access_token":"token"}}`)
	if err := os.WriteFile(filepath.Join(home, "auth.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := BorrowToken(context.Background(), http.DefaultClient)
	if err == nil || !strings.Contains(err.Error(), "not a Grok subscription") {
		t.Fatalf("BorrowToken() error = %v, want subscription error", err)
	}
}

func TestTokenExpiresSoonUsesJWTExpiry(t *testing.T) {
	if !tokenExpiresSoon(authTokens{AccessToken: testJWT(time.Now().Add(30 * time.Second))}, time.Now().Add(2*time.Minute)) {
		t.Fatal("tokenExpiresSoon() = false, want true")
	}
	if tokenExpiresSoon(authTokens{AccessToken: testJWT(time.Now().Add(time.Hour))}, time.Now().Add(2*time.Minute)) {
		t.Fatal("tokenExpiresSoon() = true, want false")
	}
}

func testJWT(exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(exp.Unix(), 10) + `}`))
	return header + "." + payload + ".signature"
}
