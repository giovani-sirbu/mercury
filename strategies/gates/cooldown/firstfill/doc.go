// Package firstfill is the Cooldown flag's first-fill gate: FirstFillHold holds a first entry on a
// local top, activated by the verdict sophos serves on /cooldown, released by price with the
// ladder's own levels (firstFillLevels) and capped by FirstFillMaxHold (firstFillExpired). Its
// state is rebuilt from the trade's first-fill events (firstFillState); FirstFillVerdictNeeded
// tells an engine whether to fetch the verdict, and NextDepthDoubled with WidenSettingsForNextDepth
// doubles the step of the depth after an entry released up through its reference. Its vocabulary
// is package cooldown's (GateFirstFill, the FirstFill* event kinds and row prefixes, FirstFillEvent).
//
// Vocabulary: first fill, depth, ladder, hold, verdict, event, row.
package firstfill
