// Package slowpattern reads the SHAPE of a ladder's decline between its fills:
// a smooth, stair-stepping fall that bought depth after depth and locked the
// wallet, which the quiet slow decline's market vote never read. It claims no
// prediction — it says the decline between two fills looks like a staircase —
// and it is pure: it imports the smart take loss block and time, and nothing
// of a trade, a row or an engine. gates/smarttakeloss owns the decision it
// feeds (the pending row, the sale, the latch); this package owns the reading.
//
// The input is the closed 1h bars sophos serves (Series), ladder-agnostic on
// purpose, and the ladder's fills (Fill) in the order it placed them. A fill's
// bar is the 1h bar its stamp falls in (BarOpen). A window runs from the bar
// of an earlier fill a to the bar of a later fill b, both inclusive, and is
// read on the bars' closes alone:
//
//   - A swing high is a bar whose close is the highest of the SlowPatternPivotBars
//     bars each side of it, a swing low the lowest, TIES COUNTING (swingPivots).
//     A pivot is known only once SlowPatternPivotBars bars closed after it, so
//     only the pivots known at the window's end bar exist, and the bars after
//     the end never change one that does. Consecutive pivots of one kind fold
//     into the more extreme of them, the later one on a tie (alternate).
//   - The legs start at the highest swing high at or after the window's start
//     and alternate down and up between the pivots, then one provisional leg
//     runs from the last pivot to the extreme close after it, up to the end
//     bar (legsFrom).
//   - The readings (Reading) are the decline's weight, the legs' fall less
//     their rise over the two together, held to SlowPatternMinWeight; the number
//     of down legs, held to SlowPatternMinDownLegs; the successive lower highs
//     of the down legs, held to SlowPatternMinLowerHighs; the largest down leg's
//     share of the fall, held to SlowPatternMaxLegShare; and the net move of
//     the window, held to SlowPatternMinFallPct. The window holds when every
//     reading meets its constant, and its Reasons — or its Breaks, naming what
//     missed — carry the frames the smart take loss writes on its rows.
//
// Trigger reads a ladder at its newest fill b: every window from an earlier
// fill a with b − a at least SlowPatternMinDepthsBetween, longest first, and
// the ladder triggers if ANY window holds. The two headline knobs of the rule
// are SlowPatternMinDepthsBetween and SlowPatternMinWeight; the other
// constants fix the shape the weight is read on. Fewer fills than one more than
// SlowPatternMinDepthsBetween are never read.
//
// The result is a pure function of the fills' bars and the closes up to the
// newest fill's bar, so it can be derived again on any later tick: a dropped
// row, a skipped print or a lock that failed never loses a reading. A window
// whose bars the series does not hold — older than its first bar, or newer
// than its last — is unreadable and never holds: the rule fails closed.
//
// Vocabulary: depth, fill, bar, window, swing, leg, weight, holds, reading.
package slowpattern
