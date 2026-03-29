package main

import (
	"sync"
	"time"
)

const (
	staleSeenTTL         = time.Hour
	staleCleanupInterval = time.Hour
)

// serialTracker keeps a lightweight in-memory record of recently seen serials.
type serialTracker struct {
	mu       sync.RWMutex
	serials  map[string]struct{}
	lastSeen map[string]time.Time
}

func newSerialTracker() *serialTracker {
	return &serialTracker{
		serials:  make(map[string]struct{}),
		lastSeen: make(map[string]time.Time),
	}
}

// wasSeenRecently reports whether a serial exists in memory.
func (t *serialTracker) wasSeenRecently(serial string) bool {
	t.mu.RLock()
	_, ok := t.serials[serial]
	t.mu.RUnlock()
	return ok
}

// markSeen updates the serial timestamp to "now".
func (t *serialTracker) markSeen(serial string, now time.Time) {
	t.mu.Lock()
	t.serials[serial] = struct{}{}
	t.lastSeen[serial] = now
	t.mu.Unlock()
}

// removeStale drops serials that have not been seen within maxAge.
func (t *serialTracker) removeStale(now time.Time, maxAge time.Duration) {
	cutoff := now.Add(-maxAge)
	t.mu.Lock()
	for serial, seenAt := range t.lastSeen {
		if seenAt.Before(cutoff) {
			delete(t.lastSeen, serial)
			delete(t.serials, serial)
		}
	}
	t.mu.Unlock()
}

func startStaleSeenSerialsCleanup(tracker *serialTracker) {
	go func() {
		ticker := time.NewTicker(staleCleanupInterval)
		defer ticker.Stop()

		for now := range ticker.C {
			tracker.removeStale(now, staleSeenTTL)
		}
	}()
}
