package smarttakeloss

import "time"

const (
	// ArmDepthOffset: the ladder arms at (Depths − offset) filled entries, so
	// it is counted in depths back from the ladder's own configured Depths and
	// never as an absolute fill number. A small offset means the smart take
	// loss watches only the ladder's last configured depths, where the support
	// line and the bounce targets decide the exit; that is also the cost of
	// arming late, because most of the wallet is already committed by the time
	// an exit rule is allowed to look. The user owns this number, with the
	// last-permitted-depth rule switched off.
	ArmDepthOffset = 1
	// minArmDepth floors the arm so short ladders (Depths <= offset+1) do not
	// arm on their first fills, where a grid is supposed to trade through
	// volatility.
	minArmDepth = 2
	// PermittedDepths is how many depths the ladder may still fill after the
	// activation while LastPermittedDepthExit is on. The rule exists to stop a
	// chain that is buying into the bottom of its window, and this is the room
	// the user left it to average the exit first. Once that many fills sit
	// after the activating one the trade is on its last permitted depth, where
	// the tolerance exit and the add refusal apply.
	PermittedDepths = 1
	// LastPermittedDepthExit switches that rule as a whole: PermittedDepths
	// more depths after the activation, then every add-side proposal becomes
	// the exit and one tolerance under the last fill sells once the window
	// has no bar left under the price — and the add refusal while the wait
	// after a fill runs.
	//
	// DEACTIVATED (switched off, not deleted): the support line is the
	// loss-side exit instead. While the price holds it the ladder keeps
	// trading normally, adds included, and it sells only on the break
	// (supportBreakReached) or on a bounce into the resistance line or the
	// band (sellTargetReached). Left on, the tolerance leg can sell a fill
	// minutes after it lands, barely under its own price and with the support
	// line still intact: an exit that banks nothing and ends the ladder.
	// Switch it back on to restore the rule; PermittedDepths and
	// MinAgeAfterLastFill keep their meaning for that.
	LastPermittedDepthExit = false
	// MinAgeAfterLastFill is how long a fill must stand before the smart take
	// loss may sell. With no wait the exit can price the same print the fill
	// did and bank the dip it has just paid for, seconds after paying for it.
	// A fill is a decision the trade has to live with for a while; only after
	// that does the exit read as an exit and not as a misfire. Editable, and
	// every leg waits — the line and the band as well as the tolerance,
	// because they are the same sell.
	//
	// The wait cuts both ways. On the window this rule reads, a fill and the
	// bounce that should sell it can land inside the same bar, so a wait
	// longer than a bar of that window sits out a whole leg of the move.
	//
	// It is measured on the fill's history stamp, which does not mean the
	// same instant on every engine (the divergence cooldown's depth spacing
	// documents): production stamps the fill's reconciliation, live-testing
	// the fill itself, and sisyphus backtesting the moment the entry order
	// was PLACED. A deeper entry is a limit resting at its price, so in a
	// replay an entry that rested longer than this already reads as old when
	// it fills — the two engines agree only on a cascade, where the limit
	// fills in seconds, which is the case the rule was chosen on. Any
	// calibration of this number carries that caveat.
	// The operator-facing copy of the value lives in
	// cp/constants/strategy-params.ts and has to move with it.
	//
	// Whatever it is set to, it separates the exit from the FILL, never from
	// the window.
	//
	// DEACTIVATED (set to zero, not deleted): every exit fires on the tick it
	// is reached, with no wait after a fill, and the add refusal that covered
	// the wait never triggers. On a flash crash the wait was exactly the time
	// in which the price fell from the tolerance line to the bottom, so the
	// exit it delayed sold the bottom. The unknown-clock guard in
	// minAgeReached stays: a fill without a stamp still holds the forced
	// exit. Set it back to a duration to restore the wait.
	MinAgeAfterLastFill = 0 * time.Hour
	// SupportBreakBars is how many of the newest closed bars of the window in
	// a row must have CLOSED under the support line (over the resistance line
	// on an inverse ladder) before the break sells; sophos counts them on its
	// own window (SupportBarsUnder / ResistanceBarsOver), the gate compares.
	// It is counted in bars of that window: a single close under the line is
	// a wick's worth of noise on this interval, several in a row are the
	// market declining to take the level back. The count alone does not sell
	// — the tick price plus one tolerance has to be under the line too
	// (supportBreakReached).
	SupportBreakBars = 3
	// SupportBounceBreakBars is the same count for a support the price had
	// ALREADY bounced from: the line's value where the price last touched it
	// and closed back over. A level that held once and then gives way is a
	// failed retest, so it needs fewer bars than a first break — keep it
	// under SupportBreakBars. Sophos serves the level and the count
	// (SupportBounceLevel / SupportBounceBarsUnder); the gate sells once the
	// count stands and the tick price is under the level — no tolerance on
	// this leg.
	SupportBounceBreakBars = 2
)
