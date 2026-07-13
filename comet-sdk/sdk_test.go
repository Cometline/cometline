package cometsdk

import "testing"

func TestDefaultProviderConfigMaxRetries(t *testing.T) {
	t.Parallel()

	if got := DefaultProviderConfig().MaxRetries; got != 5 {
		t.Fatalf("MaxRetries = %d, want 5", got)
	}
}
