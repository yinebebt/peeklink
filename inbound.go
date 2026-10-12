package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// inboundLimiters throttle HTTP clients so one visitor cannot starve others.
// Outbound platform limiters in main.go still cap origin quota.
type inboundLimiters struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter
}

func newInboundLimiters() *inboundLimiters {
	return &inboundLimiters{m: make(map[string]*rate.Limiter)}
}

func (l *inboundLimiters) allow(ip string) bool {
	l.mu.Lock()
	lim, ok := l.m[ip]
	if !ok {
		lim = rate.NewLimiter(rate.Every(2*time.Second), 8)
		l.m[ip] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

func (l *inboundLimiters) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(requestIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
