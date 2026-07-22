package gateway

import "sync"

const maxSeenPlatformMessages = 4096

// seenPlatformMessages is a process-local LRU-ish set of platform message IDs
// already accepted by HandleInbound. It is defense-in-depth against Discord
// redelivery within one gateway process; cross-process duplicates still need
// ClaimExclusive / a single gateway instance.
type seenPlatformMessages struct {
	mu    sync.Mutex
	order []string
	set   map[string]struct{}
}

func newSeenPlatformMessages() *seenPlatformMessages {
	return &seenPlatformMessages{set: make(map[string]struct{})}
}

func (s *seenPlatformMessages) seenOrAdd(key string) bool {
	if s == nil || key == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.set[key]; ok {
		return true
	}
	s.set[key] = struct{}{}
	s.order = append(s.order, key)
	for len(s.order) > maxSeenPlatformMessages {
		old := s.order[0]
		s.order = s.order[1:]
		delete(s.set, old)
	}
	return false
}

var defaultSeenPlatformMessages = newSeenPlatformMessages()
