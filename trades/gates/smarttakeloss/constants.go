package smarttakeloss

import "time"

const (
	// ArmDepthOffset: the ladder arms at (Depths − offset) filled entries.
	// One since 2026-09-07 evening (user): fill 7 on an 8-deep ladder, fill
	// 8 on a 9-deep one — the smart take loss watches only the ladder's
	// last configured depth, where the support line and the bounce targets
	// decide the exit. It was four (fill 4 on 8 depths) for the earlier
	// reading of the same day, and three before that: run 84's HBAR chain
	// put 59% of the wallet into depths 6-8 before an exit rule armed at
	// Depths − 1 could look, which is the cost of arming this late — the
	// user's call, with the last-permitted-depth rule switched off.
	ArmDepthOffset = 1
	// minArmDepth floors the arm so short ladders (Depths <= offset+1) do not
	// arm on their first fills, where a grid is supposed to trade through
	// volatility.
	minArmDepth = 2
	// PermittedDepths is how many depths the ladder may still fill after the
	// activation while LastPermittedDepthExit is on. One: the rule exists to
	// stop a chain that is buying into the bottom of its window, and one
	// more depth is the room the user left it to average the exit. Once that
	// many fills sit after the activating one the trade is on its last
	// permitted depth, where the tolerance exit and the add refusal apply.
	PermittedDepths = 1
	// LastPermittedDepthExit switches that rule as a whole: PermittedDepths
	// more depths after the activation, then every add-side proposal becomes
	// the exit and one tolerance under the last fill sells once the window
	// has no bar left under the price — and the add refusal while the wait
	// after a fill runs.
	//
	// DEACTIVATED 2026-09-07 (switched off, not deleted): the support line
	// is the loss-side exit instead. While the price holds it the ladder
	// keeps trading normally, adds included, and it sells only on the break
	// (supportBreakReached) or on a bounce into the resistance line or the
	// band (sellTargetReached). SOL 56921 on backtest 139 showed the rule
	// otherwise: fill 5 at 193.62 activated on its own tick and 41 minutes
	// later the tolerance leg sold at 192.74 with the support line intact.
	// Switch it back on to restore the rule; PermittedDepths and
	// MinAgeAfterLastFill keep their meaning for that.
	LastPermittedDepthExit = false
	// MinAgeAfterLastFill is how long a fill must stand before the smart
	// take loss may sell. Backtest 128's HBAR trade 49490 took its permitted
	// depth at 13:09:30 and sold one tolerance under it at 13:09:31 — one
	// second, on a ladder that had just bought six times into the 19 May 2021
	// crash: the exit priced the same print the fill did and banked the dip
	// it had just paid for. A fill is a decision the trade has to live with
	// for a while; only after that does the exit read as an exit and not as a
	// misfire. Editable, and every leg waits — the line and the band as well
	// as the tolerance, because they are the same sell.
	//
	// One hour since 2026-09-07, down from three. On the 4h window a fill and
	// the bounce that should sell it can land inside the same bar, and three
	// hours was long enough to sit out a whole leg of it; one hour still
	// rules out trade 49490's same-second exit, which is what the wait exists
	// for.
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
	// Calibrated against the fund-blocked ladders of backtest 129, 24h scored
	// better than the 6h this shipped with on 2026-09-06; the difference sat
	// inside the noise of one window. It separates the exit from the FILL,
	// never from the window.
	//
	// DEACTIVATED 2026-09-07 (set to zero, not deleted): every exit fires on
	// the tick it is reached, with no wait after a fill, and the add refusal
	// that covered the wait never triggers. On the flash crashes the hour was
	// the time in which the price fell through the tolerance line to the
	// bottom (BTC 52813 sold at 47 253 after a 6h wait from 51 188). The
	// unknown-clock guard in minAgeReached stays: a fill without a stamp
	// still holds the forced exit. Set it back to a duration to restore the
	// wait.
	MinAgeAfterLastFill = 0 * time.Hour
	// SupportBreakBars is how many of the newest closed 4h bars in a row must
	// have CLOSED under the support line (over the resistance line on an
	// inverse ladder) before the break sells; sophos counts them on its own
	// window (SupportBarsUnder / ResistanceBarsOver), the gate compares.
	// Three, as asked on 2026-09-07 ("minim 3 bare ÎNCHISE rămân sub linia de
	// support"): a single close under the line is a wick's worth of noise on
	// this interval, three is twelve hours of the market not taking the
	// level back. The count alone does not sell — the tick price plus one
	// tolerance has to be under the line too (supportBreakReached).
	SupportBreakBars = 3
	// SupportBounceBreakBars is the same count for a support the price had
	// ALREADY bounced from: the line's value where the price last touched it
	// and closed back over. Two, as asked on 2026-09-07 ("pune 2 bars sub
	// acel support"): a level that held once and gives way is a failed
	// retest, and it needs one bar less than a first break. Sophos serves
	// the level and the count (SupportBounceLevel / SupportBounceBarsUnder);
	// the gate sells once the count stands and the tick price is under the
	// level — no tolerance on this leg, "sub support" was the ask.
	SupportBounceBreakBars = 2
)
