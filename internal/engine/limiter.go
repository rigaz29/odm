package engine

import (
	"context"
	"sync"
	"time"
)

// Limiter is a shared token bucket throttling all connections of a download.
// A non-positive rate disables limiting entirely.
type Limiter struct {
	mu     sync.Mutex
	rate   float64
	tokens float64
	last   time.Time
}

// NewLimiter returns a limiter admitting bytesPerSecond bytes per second.
func NewLimiter(bytesPerSecond int64) *Limiter {
	return &Limiter{rate: float64(bytesPerSecond), last: time.Now()}
}

// Rate is the admitted rate in bytes per second, 0 when unlimited.
func (l *Limiter) Rate() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rate <= 0 {
		return 0
	}
	return int64(l.rate)
}

// SetRate changes the admitted rate (0 disables limiting) and refills the bucket.
func (l *Limiter) SetRate(bytesPerSecond int64) {
	l.mu.Lock()
	l.rate = float64(bytesPerSecond)
	l.tokens = 0
	l.last = time.Now()
	l.mu.Unlock()
}

// Wait blocks until n bytes may be consumed or ctx is done. The bytes are
// taken at once and the bucket may go below zero; the caller then waits
// until that debt is paid back. The bucket never holds more than one
// second of tokens, so waiting for a full balance would never end for a
// read larger than the rate (a 64 KiB read under a 32 KiB/s limit).
func (l *Limiter) Wait(ctx context.Context, n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.Lock()
	if l.rate <= 0 {
		l.mu.Unlock()
		return nil
	}
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	l.last = now
	if l.tokens > l.rate {
		l.tokens = l.rate
	}
	l.tokens -= float64(n)
	var wait time.Duration
	if l.tokens < 0 {
		wait = time.Duration(-l.tokens / l.rate * float64(time.Second))
	}
	l.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
