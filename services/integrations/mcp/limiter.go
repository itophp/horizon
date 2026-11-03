package mcp

import (
	"golang.org/x/time/rate"
	"sync"
	"time"
)

type limitEntry struct {
	limiter *rate.Limiter
	used    time.Time
}
type requestLimits struct {
	mu      sync.Mutex
	entries map[string]limitEntry
}

func (l *requestLimits) allow(key string, perMinute int, burst int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[string]limitEntry{}
	}
	now := time.Now()
	entry, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= 4096 {
			for k, e := range l.entries {
				if now.Sub(e.used) > 5*time.Minute {
					delete(l.entries, k)
				}
			}
			if len(l.entries) >= 4096 {
				return false
			}
		}
		entry = limitEntry{limiter: rate.NewLimiter(rate.Limit(float64(perMinute)/60), burst)}
	}
	entry.used = now
	l.entries[key] = entry
	return entry.limiter.Allow()
}
