package cmd

import (
	"testing"
)

func TestServeListenAddrPrefersEnv(t *testing.T) {
	t.Setenv("COMETMIND_BIND_ADDR", "0.0.0.0")
	if got := serveListenAddr("127.0.0.1"); got != "0.0.0.0" {
		t.Fatalf("serveListenAddr() = %q, want 0.0.0.0", got)
	}
}

func TestServeListenAddrUsesFlag(t *testing.T) {
	t.Setenv("COMETMIND_BIND_ADDR", "")
	if got := serveListenAddr("0.0.0.0"); got != "0.0.0.0" {
		t.Fatalf("serveListenAddr() = %q, want 0.0.0.0", got)
	}
}

func TestValidateServeBind(t *testing.T) {
	if err := validateServeBind("127.0.0.1"); err != nil {
		t.Fatalf("validateServeBind() error = %v", err)
	}
	if err := validateServeBind("not-an-ip"); err == nil {
		t.Fatal("validateServeBind() error = nil, want invalid address")
	}
}
