// Package quietdecline holds the quiet decline's settings (Enabled, ArmDepth, NeedsRecentFill) and
// the pure reads of the sophos block that decide it, each on the stamps of a ladder's fills:
// ReadsForTheLadder (the last closed bar), ReadsRecently (the look-back), PendsRecently (the
// look-back marks a watched ladder pending), Confirms (a pending ladder's new fill holds) and
// RecentFill (the recent-fill rule). Stamps are compared in milliseconds and every read fails
// closed on a bar or window sophos did not serve. gates/smarttakeloss owns the watch, the pending
// and the judgement of a fill (Enabled), the rows and the first-fill hold.
//
// Vocabulary: depth, fill, bar, verdict, leg, look-back, watched, pending.
package quietdecline
