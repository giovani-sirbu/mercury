package cooldown

import (
	"fmt"
	"strconv"
	"time"

	"github.com/giovani-sirbu/mercury/events"
)

// Depth spacing is the Cooldown flag's second gate: a gate on the ladder
// cascading through every depth during one fast drop. It reads nothing but
// the trade's own fill history — no indicator, no sophos call, no persisted
// state — so it costs a fold over rows already in memory.
//
// It is therefore only as good as the clocks the calling engine supplies, and
// it fails OPEN when either is missing. sisyphus backtesting and hermes both
// supply them; sisyphus LIVE-TESTING supplies neither — its events literal
// carries no Timestamp and nothing stamps trade.UpdatedAt
// (handlers/testing/handleTrade.go:147), and the history rows it appends carry
// no CreatedAt (:218) — so on that surface this gate is inert and a paper run
// cascades where the replay of the same strategy would gate. Two literals in
// that file fix it, the same way an earlier fix stamped trade.CreatedAt there
// so the cooldown first-fill gate could expire (handlers/testing/manageTrades.go:17-23).
//
// The shape it answers to is backtest trade 25858 (HBAR/USDT): depths placed
// at 13:41:08, 13:48:33 (+7m25s), 13:55:45 (+7m12s), 14:25:08, 15:55:10,
// 16:09:00 (+13m50s), 16:39:22 — seven depths in three hours — after which
// the trade sat blocked for 54 days before it could sell. The budget was
// spent in an afternoon on a move that was not finished falling.
//
// TIMESTAMPS, AND THEY ARE NOT THE SAME IN EVERY ENGINE. This rule folds over
// TradesHistory.CreatedAt, and that column means two different things:
//
//   - sisyphus backtesting writes UnixToTime(order.Timestamp) explicitly
//     (handlers/backtesting/processOrder.go:292), and order.Timestamp is the
//     tick the order was CREATED and never advances — so a replay measures
//     PLACEMENT-to-placement spacing;
//   - production never records a placement at all. agora skips NEW orders
//     (jobs/processPendingOrder.go:80) and only reaches UpsertHistoryData for
//     FILLED / PARTIALLY_FILLED (handlers/orders/handlePendingOrder.go:107),
//     where the row carries no CreatedAt and gorm stamps it at insert — so
//     hermes measures FILL-to-fill spacing.
//
// Only the first entry is a market order, where placement == fill; every
// deeper one is a limit that rests below the market (buy.go:93-98), so the two
// clocks can differ by hours on a slow ladder. They converge on exactly the
// case this rule was built for: in a cascade the limit at the trailed low
// fills within seconds, so rest ≈ 0 and both engines see the same gaps.
// Calibrate on replays, but do not expect a slow ladder to gate identically
// live. Closing the gap means carrying the exchange's own order-creation time
// (mercury/exchange/aggregates/orderTypes.go:37 `Time`, already fetched and
// dropped) through agora onto the row — a change to the order pipeline, not
// to this rule.
//
// Whichever clock an engine uses, it uses it for every depth of the trade, so
// the comparison inside one fold is always self-consistent.

// Depth-spacing tunables. The two populations in trade 25858 separate
// cleanly: the gaps that emptied the budget were 7-14 minutes, the gaps that
// followed a real pullback were 30m and 90m. Both populations sit inside the
// settings below, which are deliberately wider than that trade needs — read
// the boundary the code computes, described on DepthSpacingBaseHold, not the
// individual numbers.
const (
	// DepthSpacingWindow is the grace the escalation allows PAST a hold:
	// a depth that lands within this of the previous hold's expiry is still
	// the same drop, and a depth that lands later resets the count. It is
	// therefore not by itself the line between a ladder and a cascade —
	// that line is the standing hold plus this window.
	DepthSpacingWindow = 30 * time.Minute
	// DepthSpacingBaseHold is what the first fast depth costs the ladder.
	// It is LARGER than the window, so the two thresholds are not the same
	// number and must not be read as one: the escalation compares a fill
	// against the previous hold's expiry, so what counts as "the same drop"
	// is base + window at step 1 (90m), then hold + window at every step
	// after it (150m, 270m). The window alone is only the tail of that.
	DepthSpacingBaseHold = 60 * time.Minute
	// depthSpacingFactor is the escalation. A drop that keeps filling depths
	// the instant the previous hold lifts is falling faster than the grid
	// was built for, so the gate has to grow faster than the drop consumes
	// depths: a linear backoff still let trade 25858's seven depths through
	// inside two hours, doubling does not. From the base hold the sequence
	// is 60m, 2h, 4h, and it is clamped from the third fast depth on.
	depthSpacingFactor = 2
	// depthSpacingMaxHold caps one hold. Past four hours the gate stops
	// being a gate and becomes an outage: the trade sits out the bottom of
	// the very move it was slowed down for, and that bottom depth is the one
	// that pays for the rest of the ladder. Four hours is also the slowest
	// timeframe any gate in this package reads, so nothing here reasons
	// about a horizon longer than that. With the base above, the third
	// consecutive fast depth already reaches it.
	depthSpacingMaxHold = 4 * time.Hour
)

// depthSpacingState is what the fold below leaves behind: when the next depth
// may arm, and the escalation that put it there.
type depthSpacingState struct {
	// eligibleFrom is the instant the next depth may arm. Zero means the
	// history could not be read and the gate stays open.
	eligibleFrom time.Time
	// step is k: how many depths in a row arrived before the previous hold
	// had been expired for a full window.
	step int
	// hold is the wait the last fast depth earned, already capped.
	hold time.Duration
}

// DepthSpacingHoldReason is the Cooldown flag's gate on an open position:
// the ladder gate. Empty means the chain may proceed. The caller owns the
// flag, exactly like crashguard.ApplyToHold and smarttakeloss.AddFreeze.
//
// stopLoss only. This is a "no new capital yet" gate, and a gate must
// never defer a profitable close: takeProfit is the capital the rest of the
// ladder is waiting for.
func DepthSpacingHoldReason(event events.Events, position string) string {
	if position != "stopLoss" {
		return ""
	}

	// The tick clock, simulated or wall, from gates.SaveHoldLog. Unknown clocks
	// never hold, the same fail-open posture as Expired.
	now := event.TickTime()
	if now.IsZero() {
		return ""
	}

	fills := depthFills(event.Trade)
	state := depthSpacingEligibleFrom(fills)
	if state.eligibleFrom.IsZero() || !now.UTC().Before(state.eligibleFrom) {
		return ""
	}

	// The clock says wait; the market may already have paid instead. See
	// depthSpacingPriceRelease.go — the hold is a price, not a duration.
	if depthSpacingPriceReleased(event.Trade, event.Trade.PositionPrice, fills[len(fills)-1].Price, state.step) {
		return ""
	}

	// The message must be stable for as long as one hold stands: gates.SaveHoldLog
	// deduplicates on the full string, so a countdown in here would write a
	// row on every tick. Depth and the earned wait are both frozen while the
	// add is held (a held add is precisely one that has not filled); the
	// remaining time is not, and is left out.
	//
	// The depth is len(fills), NOT step+1. `step` counts only the entries that
	// arrived fast, so on any ladder containing one real pause it lags the
	// trade's actual depth — trade 25858 would have logged "depth 6" while
	// holding seven entries. len(fills) is ladder.CountFilledEntries by
	// construction (depthFillTimes mirrors it row for row), which is the
	// number regimeHold, crash-guard and smart-take-loss print for the same
	// trade on the same tick.
	// The release price is in the row because it is the other half of the
	// decision: an operator reading "parked for 30m0s" alone cannot tell that
	// a 5.2% drop would have lifted it. It is derived from the last fill and
	// the settings row, both frozen while the hold stands, so the message
	// stays byte-identical tick to tick and SaveHoldLog still collapses it.
	release, ok := depthSpacingReleasePrice(event.Trade, fills[len(fills)-1].Price, state.step)
	if !ok {
		return fmt.Sprintf(
			"cooldown: depths too close (depth %d, step %d), next add parked for %s",
			len(fills), state.step, state.hold,
		)
	}
	return fmt.Sprintf(
		"cooldown: depths too close (depth %d, step %d), next add parked for %s or until %s",
		len(fills), state.step, state.hold, strconv.FormatFloat(release, 'f', -1, 64),
	)
}
