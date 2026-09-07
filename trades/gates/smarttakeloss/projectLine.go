package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// projectLine extends the line through its two anchors to the given time
// (Unix ms): y = To.Price + slope · (at − To.At). Sophos serves the anchors
// as (bar open time, wick price), so the projection is by time and a stale
// block does not move the line. No line — a zero anchor time or price, two
// anchors on the same bar, an unknown clock — reports false.
func projectLine(line aggragates.TrendLine, atMs int64) (float64, bool) {
	if atMs <= 0 || line.From.At <= 0 || line.To.At <= 0 || line.From.Price <= 0 || line.To.Price <= 0 {
		return 0, false
	}
	if line.To.At == line.From.At {
		return 0, false
	}
	slope := (line.To.Price - line.From.Price) / float64(line.To.At-line.From.At)
	return line.To.Price + slope*float64(atMs-line.To.At), true
}
