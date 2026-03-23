// Package ratelimit provides a simple in-memory fixed-window rate limiter.
// Each key (e.g. IP address or account ID) gets an independent counter that
// resets after every window duration.
package ratelimit

import (
	"sync"
	"time"
)

// entry tracks request count within the current window.
type entry struct {
	count     int
	windowEnd time.Time
}

// Limiter is a thread-safe fixed-window rate limiter.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	max     int           // requests allowed per window
	window  time.Duration // window length
}

// New creates a Limiter that allows at most max requests per window duration per key.
func New(max int, window time.Duration) *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
		max:     max,
		window:  window,
	}
	// Background goroutine to prune stale entries every 5 minutes.
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.prune()
		}
	}()
	return l
}

// Allow returns true if the key is within the rate limit, false if it is exceeded.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.entries[key]
	if !ok || now.After(e.windowEnd) {
		l.entries[key] = &entry{count: 1, windowEnd: now.Add(l.window)}
		return true
	}
	if e.count >= l.max {
		return false
	}
	e.count++
	return true
}

// prune removes entries whose windows have expired.
func (l *Limiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, e := range l.entries {
		if now.After(e.windowEnd) {
			delete(l.entries, k)
		}
	}
}
