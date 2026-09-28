package web

import (
	"net/http"
	"sync"
	"time"
)

// Public mode serves one project to anyone who opens the address, with no
// pairing (ADR 0029). It exists for a disposable demo whose data is reset
// from outside, so it bounds what an anonymous visitor can cost rather than
// who may write.
const (
	// publicWritesPerMinute bounds bd runs for the whole server. RemoteAddr
	// is the reverse proxy in front of a public server, so a per-visitor
	// limit belongs to that proxy.
	publicWritesPerMinute = 30
	// publicMaxStreams bounds open event streams, each of which holds a
	// goroutine and a store subscription for as long as the tab is open.
	publicMaxStreams = 256
)

// rateLimiter is a token bucket refilled continuously.
type rateLimiter struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	per    time.Duration
	last   time.Time
	now    func() time.Time
}

func newRateLimiter(max int, per time.Duration) *rateLimiter {
	return &rateLimiter{tokens: float64(max), max: float64(max), per: per, now: time.Now, last: time.Now()}
}

func (l *rateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.tokens += float64(now.Sub(l.last)) / float64(l.per) * l.max
	if l.tokens > l.max {
		l.tokens = l.max
	}
	l.last = now
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// limited refuses a request once the public write budget is spent. A server
// that is not public has no limiter and passes every request.
func (s *Server) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.writes != nil && !s.writes.allow() {
			w.Header().Set("Retry-After", "10")
			writeError(w, http.StatusTooManyRequests, "this public demo is busy: try again in a few seconds")
			return
		}
		next(w, r)
	}
}

// acquireStream reserves one event stream slot, or reports that none is free.
func (s *Server) acquireStream() (release func(), ok bool) {
	if s.opts.MaxStreams <= 0 {
		return func() {}, true
	}
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.streams >= s.opts.MaxStreams {
		return nil, false
	}
	s.streams++
	return func() {
		s.streamMu.Lock()
		s.streams--
		s.streamMu.Unlock()
	}, true
}
