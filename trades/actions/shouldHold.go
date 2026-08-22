package actions

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

const (
	ActionHold  = "HOLD"
	ActionLong  = "LONG"
	ActionShort = "SHORT"
)

// Regime labels and profit-exit verdicts served by sophos' multi-timeframe
// regime set. The strings are the wire contract shared with sophos.
const (
	RegimeShock       = "shock"
	RegimeDownPersist = "downtrend-persist"
	RegimeUpPersist   = "uptrend-persist"
)

// Editable gate-policy knobs, calibrated on run 73 (6 simulated months): of
// its 29 shock holds, 26 landed on trades 0-1 entries deep — the cheapest
// fills a grid gets during volatility — while the crash guard's own deep-hold
// fired once. The shock veto therefore starts at a depth, and only off the
// trigger timeframe.
const (
	// ShockHoldMinDepth: a 15m shock parks a rebuy only from this many filled
	// entries up. Shallow rungs trade straight through volatility spikes.
	ShockHoldMinDepth = 3
	// shockHoldTimeframe: the one timeframe whose shock parks a rebuy. Higher
	// timeframes' shocks last hours per bar and are the crash guard's job.
	shockHoldTimeframe = "15m"
)

// entryVetoTimeframes / addVetoTimeframes: which timeframes' opposing trend
// vetoes an INVERSE entry or add (the long side reads EnterAllowed/AddAllowed
// computed by sophos). Entries mirror the long policy — 4h only; adds keep
// the stricter either-of pair that saved run 74's rally blow-ups.
var (
	entryVetoTimeframes = []string{"4h"}
	addVetoTimeframes   = []string{"4h", "1h"}
)

// ShouldHold blocks the action chain when the AI (or the classic cooldown
// fallback) advises against acting on the current position. Holds are
// recorded by saveHoldLog as INFO trade-log entries.
//
// With a regime verdict present, each transition consults its own answer:
// entries ask EnterAllowed (judged on 4h+1h) and rebuys ask AddAllowed (1h
// with a 4h veto). A profitable close is never gated — only new capital is.
// Without a verdict the legacy bullish/bearish logic applies unchanged.
func ShouldHold(event events.Events) (events.Events, error) {
	cool := event.Params.CoolDownIndicators
	ai := event.Params.AIIndicators

	// A trade that has not entered yet is gated only on a clearly opposing
	// signal.
	if event.Params.OldPosition == "new" {
		if ai.UseAI {
			if ai.HasRegimeVerdict {
				if blocked, reason := regimeBlocksCapital(ai, event.Trade.Inverse); blocked {
					return saveHoldLog(event, "entry", reason)
				}
				return event, nil
			}
			isBearishSignal := ai.AIMarketBearish || ai.AIAction == ActionShort
			isBullishSignal := ai.AIMarketBullish || ai.AIAction == ActionLong

			if !event.Trade.Inverse && isBearishSignal {
				return saveHoldLog(event, "entry", "AI market is bearish")
			}
			if event.Trade.Inverse && isBullishSignal {
				return saveHoldLog(event, "entry", "AI market is bullish")
			}
		}
		return event, nil
	}

	holdReason := ""

	if ai.UseAI && ai.HasRegimeVerdict {
		holdReason = regimeHoldReason(event, ai)
	} else if ai.UseAI {
		if event.Trade.Inverse {
			if event.Trade.PositionType == "stopLoss" && ai.AIMarketBullish {
				holdReason = "AI market is bullish"
			}
			if event.Trade.PositionType == "takeProfit" && ai.AIMarketBearish {
				holdReason = "AI market is bearish"
			}
		} else {
			if event.Trade.PositionType == "stopLoss" && ai.AIMarketBearish {
				holdReason = "AI market is bearish"
			}
			if event.Trade.PositionType == "takeProfit" && ai.AIMarketBullish {
				holdReason = "AI market is bullish"
			}
		}

		// Explicit HOLD means "no direction", so it gates only positions that
		// add risk (rebuys/averaging). A profitable close reduces exposure and
		// is allowed to execute — same rule for inverse trades, where
		// takeProfit is still the profitable close.
		if holdReason == "" && ai.AIAction == ActionHold && event.Trade.PositionType != "takeProfit" {
			holdReason = "AI recommends HOLD"
		}
	} else {
		if event.Trade.Inverse {
			if event.Trade.PositionType == "stopLoss" && cool.MarketBullish {
				holdReason = "cooldown market is bullish"
			}
			if event.Trade.PositionType == "takeProfit" && cool.MarketBearish {
				holdReason = "cooldown market is bearish"
			}
		} else {
			if event.Trade.PositionType == "stopLoss" && cool.MarketBearish {
				holdReason = "cooldown market is bearish"
			}
			if event.Trade.PositionType == "takeProfit" && cool.MarketBullish {
				holdReason = "cooldown market is bullish"
			}
		}
	}

	if holdReason != "" {
		return saveHoldLog(event, event.Trade.PositionType, holdReason)
	}

	return event, nil
}

// regimeBlocksCapital answers whether the regime verdict refuses new capital
// for this trade's direction. The verdict is computed long-only, so a normal
// trade reads EnterAllowed while an inverse trade mirrors it from the regime
// labels: what blocks a long entry is a persistent slide, what blocks an
// inverse entry is a persistent rise — shock blocks both.
func regimeBlocksCapital(ai aggragates.AIIndicators, inverse bool) (bool, string) {
	if !inverse {
		// EnterAllowed already carries the whole entry policy, the 4h shock
		// included — no blanket shock veto on top: a fresh depth-1 entry
		// during a 15m spike is the fill the grid wants.
		if !ai.EnterAllowed {
			return true, fmt.Sprintf("regime: enter not allowed (%s)", regimeDetail(ai))
		}
		return false, ""
	}
	for _, timeframe := range entryVetoTimeframes {
		if ai.Regimes[timeframe] == RegimeUpPersist || ai.Regimes[timeframe] == RegimeShock {
			return true, fmt.Sprintf("regime: inverse enter not allowed (%s %s)", timeframe, ai.Regimes[timeframe])
		}
	}
	return false, ""
}

// regimeHoldReason maps the per-transition verdict onto an existing trade's
// position. Empty means the chain may proceed.
func regimeHoldReason(event events.Events, ai aggragates.AIIndicators) string {
	inverse := event.Trade.Inverse

	// A profitable close is never deferred. The regime used to hold it while
	// its 15m view still read as rising ("keep trailing"), but a 6-week A/B
	// settled it: the rule fired on 49 of 93 holds, cost 15 closed trades and
	// 67 USD of extra unrealised exposure, and on the one stretch built for it
	// — the recovery after the 10 Oct 2025 flush — it earned 6% on a single
	// trade. The engine already trails on its own
	// (`percentage > trailingTakeProfit ? 'update_takeProfit'`), so the regime
	// was duplicating that with a cruder trigger.
	switch event.Trade.PositionType {
	case "stopLoss":
		// A stopLoss position is a pending rebuy — new capital, judged like
		// an add. A deep trade adds nothing while the market flushes: the
		// crash guard either de-risks it (engines with sellLoss rails) or
		// parks it here.
		filledEntries := trades.CountFilledEntries(event.Trade)
		if ai.CrashActive && filledEntries >= CrashDeRiskMinDepth {
			return "crash-guard: deep trade, no new capital during a flush"
		}
		// The shock veto is depth-aware and reads only the trigger timeframe:
		// shallow rungs are cheap and a volatility spike is exactly when the
		// grid gets its best fills, so only an already-committed ladder parks.
		if filledEntries >= ShockHoldMinDepth && ai.Regimes[shockHoldTimeframe] == RegimeShock {
			return fmt.Sprintf("regime: market in shock (%s, depth %d)", shockHoldTimeframe, filledEntries)
		}
		if !inverse && !ai.AddAllowed {
			return fmt.Sprintf("regime: add not allowed (%s)", regimeDetail(ai))
		}
		if inverse {
			for _, timeframe := range addVetoTimeframes {
				if ai.Regimes[timeframe] == RegimeUpPersist {
					return fmt.Sprintf("regime: inverse add not allowed (%s %s)", timeframe, RegimeUpPersist)
				}
			}
		}
	}

	return ""
}

// regimeDetail names the timeframe that carried the veto, for the trade log.
func regimeDetail(ai aggragates.AIIndicators) string {
	for _, timeframe := range []string{"4h", "1h", "15m"} {
		regime := ai.Regimes[timeframe]
		if regime == RegimeDownPersist || regime == RegimeShock {
			return fmt.Sprintf("%s %s", timeframe, regime)
		}
	}
	return ai.Regime
}
