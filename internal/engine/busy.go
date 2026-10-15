package engine

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// busyError is a 429 Too Many Requests or 503 Service Unavailable answer:
// the server asks the client to come back later, and may say when.
type busyError struct {
	status string
	wait   time.Duration // from Retry-After; 0 when the server did not say
}

func (e *busyError) Error() string { return "server busy: " + e.status }

// maxRetryAfter caps how long one Retry-After is honoured, so a server that
// asks for an hour does not keep a download waiting that long per attempt.
const maxRetryAfter = 2 * time.Minute

// busyResponse returns a *busyError for a 429 or 503 response, else nil.
func busyResponse(resp *http.Response, now time.Time) error {
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
		return nil
	}
	return &busyError{status: resp.Status, wait: retryAfter(resp.Header.Get("Retry-After"), now)}
}

// retryAfter reads a Retry-After value, which is either a number of seconds
// or an HTTP date. 0 means none, or one that is not in the future.
func retryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs > 0 {
			return time.Duration(secs) * time.Second
		}
		return 0
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// retryWait is how long to wait before the next attempt after err: what a
// busy server asked for (capped), else the usual backoff.
func retryWait(err error, backoff time.Duration) time.Duration {
	var busy *busyError
	if errors.As(err, &busy) && busy.wait > 0 {
		if busy.wait > maxRetryAfter {
			return maxRetryAfter
		}
		return busy.wait
	}
	return backoff
}

// inspect probes rawURL with the download's client. A busy answer (429,
// 503) is tried again, after what the server asked for or the usual
// backoff, up to MaxRetries times; other failures return at once, as before.
func (d *Download) inspect(ctx context.Context, rawURL string, headers map[string]string) (*Probe, error) {
	backoff := d.retryDelay()
	var err error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		var p *Probe
		p, err = inspectWith(ctx, d.httpClient(), rawURL, headers, d.authorizer())
		var busy *busyError
		if err == nil || !errors.As(err, &busy) {
			return p, err
		}
		if serr := sleepCtx(ctx, retryWait(err, backoff)); serr != nil {
			return nil, serr
		}
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
	return nil, err
}

// sleepCtx waits for d or until ctx is done, reporting ctx's error then.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
