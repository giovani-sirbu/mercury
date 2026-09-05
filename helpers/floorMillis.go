package helpers

import "time"

// FloorMillis floors a millisecond timestamp to the start of its N-minute
// window. A non-positive timestamp or window returns 0.
func FloorMillis(ms int64, minutes int64) int64 {
	size := minutes * int64(time.Minute/time.Millisecond)
	if ms <= 0 || size <= 0 {
		return 0
	}
	return ms / size * size
}
