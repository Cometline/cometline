package wakeup

import "testing"

func TestSignalCoalescesAndDrainClears(t *testing.T) {
	ch := make(chan struct{}, 1)
	Signal(ch)
	Signal(ch)
	if got := len(ch); got != 1 {
		t.Fatalf("queued signals = %d, want 1", got)
	}

	Drain(ch)
	if got := len(ch); got != 0 {
		t.Fatalf("queued signals after drain = %d, want 0", got)
	}
}
