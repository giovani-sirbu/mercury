package helpers

// UnixMillis normalizes a Unix timestamp given in seconds, milliseconds,
// microseconds or nanoseconds to milliseconds. Values below 1e9 (a seconds
// stamp before 2001) are treated as unset and return 0.
func UnixMillis(ts int64) int64 {
	switch {
	case ts >= 1e18:
		return ts / 1e6
	case ts >= 1e15:
		return ts / 1e3
	case ts >= 1e12:
		return ts
	case ts >= 1e9:
		return ts * 1e3
	default:
		return 0
	}
}
