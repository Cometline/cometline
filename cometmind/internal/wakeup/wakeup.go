// Package wakeup provides coalesced signals for background worker config changes.
package wakeup

// Signal wakes a listener without blocking or queueing duplicate wake-ups.
func Signal(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Drain removes one pending wake-up before a worker enters its wait loop.
func Drain(ch <-chan struct{}) {
	select {
	case <-ch:
	default:
	}
}
