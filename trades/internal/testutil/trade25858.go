package testutil

import "time"

// Trade25858 is the case depth spacing exists for: HBAR/USDT put seven
// depths on the book in three hours and then sat blocked for 54 days.
func Trade25858() []time.Time {
	return []time.Time{
		At("13:41:08"), At("13:48:33"), At("13:55:45"), At("14:25:08"),
		At("15:55:10"), At("16:09:00"), At("16:39:22"),
	}
}
