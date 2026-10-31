package testutil

import "time"

// Trade25858 is the fill sequence depth spacing exists for: a ladder that put
// every depth on the book inside one afternoon and then sat blocked until the
// price came back.
func Trade25858() []time.Time {
	return []time.Time{
		At("13:41:08"), At("13:48:33"), At("13:55:45"), At("14:25:08"),
		At("15:55:10"), At("16:09:00"), At("16:39:22"),
	}
}
