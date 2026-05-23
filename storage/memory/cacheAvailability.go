package memory

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

var ErrRemoteUnavailable = errors.New("cache remote unavailable")

const defaultRemoteRetryInterval = 30 * time.Second

func (m *Memory) remoteUnavailable() error {
	m.remoteMu.RLock()
	until := m.remoteUnavailableUntil
	cause := m.remoteUnavailableErr
	m.remoteMu.RUnlock()

	if time.Now().Before(until) {
		if cause != nil {
			return fmt.Errorf("%w: %v", ErrRemoteUnavailable, cause)
		}
		return ErrRemoteUnavailable
	}
	return nil
}

// Available reports whether the remote (Redis / Dragonfly) is currently
// reachable based on the most recent operation. Cheap — does NOT perform a
// Redis call, just reads the in-process availability sentinel maintained by
// recordRemoteError / recordRemoteSuccess. Returns false during the retry
// window after a recent failure, true otherwise.
//
// Use this to fast-fail expensive code paths that will ultimately exit at a
// fail-closed boundary (e.g., hermes ManageTrade exits if TryLockTrade
// cannot acquire the lock). Checking Available() at the top of those paths
// avoids the wasted HTTP cascade to agora that would otherwise fire on
// every tick during a Redis outage.
//
// Cost of a false positive (returns true while remote is actually down):
// one cache op fails and sets the sentinel; the next call returns false.
// One wasted tick after the outage starts, then steady state.
func (m *Memory) Available() bool {
	return m.remoteUnavailable() == nil
}

func (m *Memory) recordRemoteError(err error) {
	if !isRemoteAvailabilityError(err) {
		return
	}

	m.remoteMu.Lock()
	m.remoteUnavailableUntil = time.Now().Add(m.retryInterval())
	m.remoteUnavailableErr = err
	m.remoteMu.Unlock()
}

func (m *Memory) recordRemoteSuccess() {
	m.remoteMu.Lock()
	m.remoteUnavailableUntil = time.Time{}
	m.remoteUnavailableErr = nil
	m.remoteMu.Unlock()
}

func (m *Memory) retryInterval() time.Duration {
	if m.remoteRetryInterval > 0 {
		return m.remoteRetryInterval
	}
	return defaultRemoteRetryInterval
}

func isRemoteAvailabilityError(err error) bool {
	if err == nil ||
		errors.Is(err, cache.ErrCacheMiss) ||
		errors.Is(err, redis.Nil) ||
		errors.Is(err, ErrRemoteUnavailable) {
		return false
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection refused") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "no such host") ||
		strings.Contains(message, "i/o timeout")
}
