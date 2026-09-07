package smarttakeloss

import "time"

const (
	// ArmDepthOffset: the ladder arms at (Depths − offset) filled entries,
	// fill 5 on an 8-deep ladder. Arming three depths before the last one
	// leaves the doubling multiplier's costliest fills still ahead: run 84's
	// HBAR chain put 59% of the wallet into depths 6-8 before an exit rule
	// armed at Depths − 1 could look.
	ArmDepthOffset = 3
	// minArmDepth floors the arm so short ladders (Depths <= offset+1) do not
	// arm on their first fills, where a grid is supposed to trade through
	// volatility.
	minArmDepth = 2
	// PermittedDepths is how many depths the ladder may still fill after the
	// activation. One: the rule exists to stop a chain that is buying into
	// the bottom of two hundred days, and one more depth is the room the user
	// left it to average the exit. Once that many fills sit after the activating one the
	// trade is on its last permitted depth, where the tolerance exit and the
	// add refusal apply.
	PermittedDepths = 1
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
	// inside the noise of one window, and the value has been 3h since
	// 2026-09-07. On the 1d window the rule now reads that is an eighth of a
	// bar: it separates the exit from the FILL, never from the window, whose
	// levels cannot move inside the wait at all.
	MinAgeAfterLastFill = 3 * time.Hour
)
