// Package smarttakeloss is the SmartTakeLoss flag: the forced-exit
// evaluation on a deep ladder (Evaluate, Position), the exit policy on the
// STL rails (ApplyExitPolicy), the stale-bag cut, the HTF add freeze
// (AddFreeze) and the ProtectiveTick overlay live and backtest both apply.
package smarttakeloss

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

const (
	// RiskThreshold is the continuation risk (down for a long, up for an
	// inverse) at or above which the HIGH test arms. Sophos serves the
	// score; the cutoff lives here so recalibration never needs a sophos
	// change.
	RiskThreshold = 70.0
	// MinReversalEvidence vetoes the forced exit: at or above this much
	// reversal evidence in the trade's favor, one more depth is judged
	// survivable and the normal flow keeps the trade.
	MinReversalEvidence = 60.0
	// MinBlockedDays is the projected recovery horizon that counts as a
	// "long" block. The projection divides the break-even distance by the
	// daily NATR (a RANGE measure), so it inflates exactly during a crash —
	// the harder the dump, the bigger the denominator, the shorter the
	// projected block. At the original 30 this made the whole loss-taking
	// branch dead code in a bear: run 82's six chains ground to −20..−37%
	// while the gate's projection never once cleared 12.6 days at a moment
	// where risk/reversal already passed (2,119 sophos-served 4h moments
	// measured). Recalibrated to 3 on that distribution: fires all six
	// doomed chains in late-Nov/mid-Jan for ~+21k USD saved vs holding to
	// the window end, and fires on NONE of the mid-Oct chains that later
	// recovered and closed in profit (their projection never reached 1.7
	// days). Risk 60 vs 70 is a wash here (±7 USD), so the risk gate stays
	// at its stricter 70 — estBlockedDays was the entire dead gate.
	MinBlockedDays = 3.0
	// TrailingExit switches the exit style for A/B replays: true rides
	// bounces through the smartTakeLoss trailing states and sells on the
	// first reversal from the best price seen; false closes NOW on the
	// sellLoss rails at the verdict-time price.
	TrailingExit = true
	// ArmDepthOffset: the depth-based arm starts at (Depths − offset) filled
	// entries. The original offset of 1 armed only at the second-to-last
	// rung — after the doubling multiplier had already committed the bulk
	// of the ladder's capital. Run 84's HBAR chain put 88.6k USDT (59% of
	// the wallet) into rungs 6-8 BEFORE the evaluation could even run, so
	// the well-timed cut still cost −14.2k; armed from depth 5 the same
	// signals (risk 85+ held for weeks) would have both cut at ~13k
	// committed and prevented rungs 6-8 from ever filling. The risk and
	// reversal gates stay as the actual decision — arming earlier only lets
	// them look.
	ArmDepthOffset = 3
	// minArmDepth floors the depth-based arm so short ladders
	// (Depths <= offset+1) do not arm on their first rungs, where a grid is
	// supposed to trade through volatility.
	minArmDepth = 2
)

// Evaluation carries the numbers behind a smart-take-loss decision so the
// transition log can say WHY a trade was armed or closed.
type Evaluation struct {
	Armed               bool     // ladder at its last depths, or already blocked
	FilledEntries       int      // depth at evaluation time
	MaxDepths           int      // the governing row's Depths sizing
	Risk                float64  // continuation risk against the trade's direction
	ReversalEvidence    float64  // reversal evidence in the trade's favor
	RequiredRecoveryPct float64  // price move to break even, 0 when profitable
	EstBlockedDays      float64  // RequiredRecoveryPct / daily NATR
	ProfitNow           bool     // fee-free close clears zero at this print
	Reasons             []string // sophos' continuation reasons, verbatim
	// StaleCut marks the 21-day stale-bag exit (staleBagDue), which reads
	// no continuation lens: the row it produces must say so, not pose as a
	// risk-driven trigger. AgeDays and EstProfit carry its numbers.
	StaleCut  bool
	AgeDays   float64
	EstProfit float64
}

// Evaluate answers whether the trade should force-exit NOW. Armed from
// (Depths − ArmDepthOffset) filled entries up — deep enough that the
// ladder's doubling has real capital committed, early enough that the worst
// rungs have not filled yet — or when already fund-blocked (initial-bid
// sizing can settle below the configured Depths, so the block event is the
// ground truth). A HIGH verdict needs the direction-matched continuation
// risk over the threshold, no favorable reversal evidence, and either a
// profitable close available right now or a long-enough projected block.
func Evaluate(trade aggragates.Trades, price float64, ai aggragates.AIIndicators) (Evaluation, bool) {
	var eval Evaluation
	if price <= 0 || trade.ParentID != 0 {
		return eval, false
	}
	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return eval, false
	}

	eval.FilledEntries = ladder.CountFilledEntries(trade)
	row := ladder.SettingsIndexOrBase(settings, eval.FilledEntries)
	eval.MaxDepths = int(math.Floor(settings[row].Depths))
	eval.Armed = tradeInZone(trade)
	if !eval.Armed || !ai.HasContinuationVerdict {
		return eval, false
	}

	eval.Risk = ai.DownContinuationRisk
	eval.ReversalEvidence = ai.ReversalUpEvidence
	if trade.Inverse {
		eval.Risk = ai.UpContinuationRisk
		eval.ReversalEvidence = ai.ReversalDownEvidence
	}
	eval.Reasons = ai.ContinuationReasons

	// A fund-blocked ladder that can close in profit banks it UNCONDITIONALLY:
	// the freed capital is the run's scarcest resource, and the risk gate
	// below reads the market's direction, not the wallet's. Run 79's trade
	// 14632 (BTC, depth 6/7, ~21k USDT captive) recovered to +2.2% on
	// 14 Jan in a RISING market — down-risk was naturally low, so the gated
	// profit branch never fired, the recovery slipped away, and the trade
	// stayed blocked for months. Only the loss-taking exits stay behind the
	// continuation-risk gate.
	if trade.Status == aggragates.Blocked {
		if pnl, invested := estimateCloseProfit(trade, price); invested > 0 && pnl > 0 {
			eval.ProfitNow = true
			return eval, true
		}
	}

	if eval.Risk < RiskThreshold ||
		eval.ReversalEvidence >= MinReversalEvidence {
		return eval, false
	}

	pnl, invested := estimateCloseProfit(trade, price)
	if invested <= 0 {
		return eval, false
	}
	if pnl > 0 {
		// Without this short-circuit the profitable exit would be the one
		// that never fires: near-zero required recovery means near-zero
		// projected blocked days, failing the month gate below.
		eval.ProfitNow = true
		return eval, true
	}
	if ai.DailyNatrPct <= 0 {
		return eval, false
	}

	eval.RequiredRecoveryPct = requiredRecoveryPct(trade, price)
	if eval.RequiredRecoveryPct <= 0 {
		return eval, false
	}
	eval.EstBlockedDays = eval.RequiredRecoveryPct / ai.DailyNatrPct
	return eval, eval.EstBlockedDays >= MinBlockedDays
}
