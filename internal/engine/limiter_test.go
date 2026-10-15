package engine

import (
	"context"
	"testing"
	"time"
)

// A read larger than one second of the rate used to wait for a balance the
// bucket can never hold, so a limit below the read size stopped the
// download for good.
func TestLimiterAdmitsReadLargerThanRate(t *testing.T) {
	l := NewLimiter(32 << 10)
	l.SetRate(32 << 10) // start from an empty bucket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if err := l.Wait(ctx, 48<<10); err != nil {
		t.Fatalf("48 KiB at 32 KiB/s: %v", err)
	}
	if took := time.Since(start); took < time.Second || took > 3*time.Second {
		t.Fatalf("48 KiB at 32 KiB/s took %v, want about 1.5s", took)
	}
}

func TestLimiterKeepsTheRateAcrossCalls(t *testing.T) {
	l := NewLimiter(256 << 10)
	l.SetRate(256 << 10)
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 8; i++ { // 512 KiB at 256 KiB/s
		if err := l.Wait(ctx, 64<<10); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(start); took < 1500*time.Millisecond || took > 3*time.Second {
		t.Fatalf("512 KiB at 256 KiB/s took %v, want about 2s", took)
	}
}

func TestLimiterOffAndCancel(t *testing.T) {
	var nilLimiter *Limiter
	if err := nilLimiter.Wait(context.Background(), 1<<20); err != nil {
		t.Fatalf("nil limiter: %v", err)
	}
	if err := NewLimiter(0).Wait(context.Background(), 1<<20); err != nil {
		t.Fatalf("rate 0: %v", err)
	}
	l := NewLimiter(1 << 10)
	l.SetRate(1 << 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx, 64<<10); err == nil {
		t.Fatal("a cancelled wait returned nil")
	}
}
