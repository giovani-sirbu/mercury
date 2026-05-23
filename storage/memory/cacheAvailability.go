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
