// Package flatmarket holds the flat market gate's settings (Enabled, HoldInitialDepth, ArmDepth)
// and its two pure readings of the last closed bars of Timeframe sophos serves over
// WindowTimeframe (Bars), against a ladder's base percentage and entry side under the Trend
// Direction row (series.Trend): the range (TriggerRange, the newest RangeBars bars: quiet bodies
// under BodyShare, the move on the ladder's side under MaxMovePct, every inner close near the
// mean, all closes within MaxCloseRangePct) and the line (TriggerLine, the whole window: a
// least-squares line through the closes, flat within MaxDeviation or moving the ladder's way,
// no steeper than MaxSlopePercentage, the bars within the bound but for TolerancePercentage),
// each under MinSmcTrend rows of the trend. Trigger holds when either reading holds, the range
// first; LatchTrigger is the latch's reading. Bars not served fail closed, and a miss formats
// nothing (Breaks). gates/smartsell owns the hold, the latch, the episode and the sale.
//
// Vocabulary: bar, body, move, line, bound, range, hold, reading.
package flatmarket
