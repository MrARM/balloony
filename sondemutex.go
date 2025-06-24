package main

import "sync"

// sondeMutex is a map-based mutex for serials
var (
	sondeMutex   = make(map[string]struct{})
	sondeMutexMu sync.Mutex
)

func claimSonde(serial string) bool {
	sondeMutexMu.Lock()
	defer sondeMutexMu.Unlock()
	if _, exists := sondeMutex[serial]; exists {
		return false
	}
	sondeMutex[serial] = struct{}{}
	return true
}

func releaseSonde(serial string) {
	sondeMutexMu.Lock()
	defer sondeMutexMu.Unlock()
	delete(sondeMutex, serial)
}
