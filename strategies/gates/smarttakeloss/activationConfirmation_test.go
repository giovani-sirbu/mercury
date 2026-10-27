package smarttakeloss

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ActivationConfirmation holds a reached trigger back: the first reached print writes the gate's
// reached row and holds the ladder; the activation (or the sale of a self-armed stopLoss) comes on a
// reached print found between ActivationConfirmation and ActivationConfirmationWindow after the row,
// one found later starts over with a new row. A print short of the trigger writes its held row by
// the old rules only and leaves the stamp standing. The knobs are pinned off for the package (the
// window at its shipped value); each test switches them on (withConfirmation), and every expectation
// is stated by hand off the fixtures.

// confirmFor and confirmWindow are the confirmation and its window the tests switch on, confirmStart
// the clock of the first reached print, after the newest fill of every fixture ladder.
const (
	confirmFor    = 5 * time.Second
	confirmWindow = time.Minute
)

var confirmStart = testutil.At("23:00:00")

// withConfirmation runs the test with the confirmation at confirmFor and its window at confirmWindow.
func withConfirmation(t *testing.T) {
	t.Helper()
	withActivationConfirmation(t, confirmFor)
	withActivationConfirmationWindow(t, confirmWindow)
}

// assertConfirming fails unless Apply held the ladder on a trigger awaiting its confirmation:
// nothing proposed, sold or activated, exactly these rows.
func assertConfirming(t *testing.T, got Result, rows ...Row) {
	t.Helper()
	assertTrailHeld(t, got, rows...)
}

// confirmation is one way the shared action meets a confirmed trigger: the ladder in buy activated, a
// self-armed stopLoss sold; opening is what the first tick writes ahead of the reached row.
type confirmation struct {
	name    string
	start   aggragates.Trades
	opening []Row
	acts    func(t *testing.T, got Result, price float64, before ...Row)
}

// confirmations are the two ways to act on a fixture's confirmed trigger.
func confirmations(c saleFixture) []confirmation {
	return []confirmation{
		{"buy activates", c.trade, c.opening, func(t *testing.T, got Result, price float64, before ...Row) {
			t.Helper()
			assertActivated(t, got, append(slices.Clone(before), c.activated(price))...)
		}},
		{"a self-armed stopLoss sells", restingIn(c.opened(), "stopLoss"), nil, func(t *testing.T, got Result, price float64, before ...Row) {
			t.Helper()
			assertSoldWith(t, got, c.reason, before...)
		}},
	}
}

// On every sale gate, in buy and from a self-armed stopLoss: the first reached print writes the
// reached row and drops the ladder's proposal; prints reached again inside the confirmation write
// nothing and hold; a print from the confirmation to the window after the row acts (judged on its own
// profit); one later than the window finds the stamp stale and writes a new reached row; an engine
// with no clock, or the knob off, acts on the first print.
func TestAReachedTriggerIsConfirmedBeforeTheSaleGateActs(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			reached := c.reached(c.within)
			for _, way := range confirmations(c) {
				t.Run(way.name, func(t *testing.T) {
					firstRows := append(slices.Clone(way.opening), reached)
					for _, position := range addSideProposals {
						assertConfirming(t, ApplyAt(way.start, position, c.within, c.ai, confirmStart), firstRows...)
					}

					first, got := engineStepAt(way.start, "", c.within, confirmStart, c.ai)
					assertConfirming(t, got, firstRows...)
					if stamp := memoryOf(rebuildState(first), c.gate).reachedAt; !stamp.Equal(confirmStart) {
						t.Fatalf("the reached event must stamp the gate at the first reached print, got %v", stamp)
					}

					for _, wait := range []time.Duration{time.Nanosecond, confirmFor / 2, confirmFor - time.Nanosecond} {
						assertConfirming(t, ApplyAt(first, "", c.within, c.ai, confirmStart.Add(wait)))
					}
					for _, wait := range []time.Duration{confirmFor, 3 * confirmFor, confirmWindow} {
						way.acts(t, ApplyAt(first, "", c.within, c.ai, confirmStart.Add(wait)), c.within)
					}
					way.acts(t, ApplyAt(first, "", c.within+0.01, c.ai, confirmStart.Add(confirmFor)), c.within+0.01)

					staleAt := confirmStart.Add(confirmWindow + time.Nanosecond)
					assertConfirming(t, ApplyAt(first, "", c.within, c.ai, staleAt), reached)
					renewed, _ := engineStepAt(first, "", c.within, staleAt, c.ai)
					if stamp := memoryOf(rebuildState(renewed), c.gate).reachedAt; !stamp.Equal(staleAt) {
						t.Fatalf("a stale stamp is replaced by the new reached row's, got %v", stamp)
					}
					assertConfirming(t, ApplyAt(renewed, "", c.within, c.ai, staleAt.Add(confirmFor/2)))
					way.acts(t, ApplyAt(renewed, "", c.within, c.ai, staleAt.Add(confirmFor)), c.within)

					way.acts(t, Apply(way.start, "", c.within, c.ai), c.within, way.opening...)
					withActivationConfirmation(t, 0)
					way.acts(t, ApplyAt(way.start, "", c.within, c.ai, confirmStart), c.within, way.opening...)
				})
			}
		})
	}
}

// A print short of the trigger after a reached one writes no row on account of the stamp and does not
// cancel it: with the episode already held under the trigger it answers nothing, and the next reached
// print, a confirmation after the row, still acts. A held row written for its own reason (the first
// of the episode) drops the stamp as the fold always did, and the next reached print starts over.
func TestAShortPrintLeavesTheReachedPrintStanding(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			reached := c.reached(c.within)
			for _, way := range confirmations(c) {
				t.Run(way.name, func(t *testing.T) {
					held, got := engineStepAt(way.start, "", c.over, confirmStart.Add(-time.Second), c.ai)
					assertWaits(t, got, "", append(slices.Clone(way.opening), c.held(c.over))...)
					stamped, got := engineStepAt(held, "", c.within, confirmStart, c.ai)
					assertConfirming(t, got, reached)

					for _, wait := range []time.Duration{time.Millisecond, confirmFor / 2, confirmFor, confirmWindow, 2 * confirmWindow} {
						assertUntouched(t, ApplyAt(stamped, "", c.over, c.ai, confirmStart.Add(wait)), "")
					}
					back, got := engineStepAt(stamped, "", c.over, confirmStart.Add(confirmFor/2), c.ai)
					assertUntouched(t, got, "")
					if stamp := memoryOf(rebuildState(back), c.gate).reachedAt; !stamp.Equal(confirmStart) {
						t.Fatalf("a short print must leave the stamp standing, got %v", stamp)
					}
					assertConfirming(t, ApplyAt(back, "", c.within, c.ai, confirmStart.Add(confirmFor-time.Nanosecond)))
					way.acts(t, ApplyAt(back, "", c.within, c.ai, confirmStart.Add(confirmFor)), c.within)
					assertConfirming(t, ApplyAt(back, "", c.within, c.ai, confirmStart.Add(confirmWindow+time.Nanosecond)), reached)

					unheld, _ := engineStepAt(way.start, "", c.within, confirmStart, c.ai)
					dropped, got := engineStepAt(unheld, "", c.over, confirmStart.Add(confirmFor/2), c.ai)
					assertWaits(t, got, "", c.held(c.over))
					if stamp := memoryOf(rebuildState(dropped), c.gate).reachedAt; !stamp.IsZero() {
						t.Fatalf("a held row written for its own reason must drop the stamp, got %v", stamp)
					}
					assertConfirming(t, ApplyAt(dropped, "", c.within, c.ai, confirmStart.Add(confirmFor)), reached)
				})
			}
		})
	}
}

// A bounce across the trigger, prints alternating reached and short every 200 ms from an episode held
// under the trigger, writes exactly one reached row and nothing else, and acts on the first reached
// print at or after the confirmation (the 26th print, 5.2 s in: the 25th is a short one).
func TestABounceAroundTheTriggerWritesOneReachedRow(t *testing.T) {
	const step = 200 * time.Millisecond
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			for _, way := range confirmations(c) {
				t.Run(way.name, func(t *testing.T) {
					trade, _ := engineStepAt(way.start, "", c.over, confirmStart.Add(-time.Second), c.ai)
					reachedRows, acted := 0, -1
					for index := 0; index <= int(30*time.Second/step); index++ {
						price := c.within
						if index%2 == 1 {
							price = c.over
						}
						at := confirmStart.Add(time.Duration(index) * step)
						next, got := engineStepAt(trade, "", price, at, c.ai)
						if got.Activated || got.Reason != "" {
							way.acts(t, got, price)
							acted = index
							break
						}
						for _, row := range got.Rows {
							if row.Event != EventReached {
								t.Fatalf("print %d: only the reached row may be written on the bounce, got %+v", index, row)
							}
							reachedRows++
						}
						if got.Position != "" {
							t.Fatalf("print %d: the ladder must stay held, got %+v", index, got)
						}
						trade = next
					}
					if reachedRows != 1 || acted != 26 {
						t.Errorf("want one reached row and the action on the 26th print, got %d rows and the action on print %d", reachedRows, acted)
					}
				})
			}
		})
	}
}

// A window at zero or below is no window: the stamp never goes stale by age, so a reached print hours
// after the row still confirms on it instead of writing a new one.
func TestAWindowAtZeroOrBelowNeverMakesTheReachedRowStale(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withActivationConfirmation(t, confirmFor)
			reached := c.reached(c.within)
			for _, window := range []time.Duration{0, -time.Second} {
				withActivationConfirmationWindow(t, window)
				for _, way := range confirmations(c) {
					first, got := engineStepAt(way.start, "", c.within, confirmStart, c.ai)
					assertConfirming(t, got, append(slices.Clone(way.opening), reached)...)
					assertConfirming(t, ApplyAt(first, "", c.within, c.ai, confirmStart.Add(confirmFor/2)))
					for _, wait := range []time.Duration{confirmFor, time.Hour, 3 * time.Hour} {
						way.acts(t, ApplyAt(first, "", c.within, c.ai, confirmStart.Add(wait)), c.within)
					}
				}
			}
		})
	}
}

// A reached row stamped before the newest fill is stale, whatever its age: with the episode standing
// through a new fill, a reached print inside the window of the old row writes a new reached row and does
// not act, and the print a confirmation after that one does. The fill is read at the newest entry's
// FilledMoment: its FilledAt when stamped, else its CreatedAt, so a resting add placed before the row
// and filled after it makes the row stale, and one filled (or only placed) before it does not. Only the
// gates that keep an episode across a fill are told apart this way (the others' episodes reset or
// re-confirm at the fill, which zeroes the stamp).
func TestAReachedRowStampedBeforeTheNewestFillIsStale(t *testing.T) {
	stamped := func(at time.Time) *time.Time { return &at }
	for _, gate := range saleGates {
		if gate.name != "flat market" && gate.name != "free fall prevention" {
			continue
		}
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			withFlatMarketResetOnNewDepth(t, false)
			withFreeFallPreventionResetOnNewDepth(t, false)

			first, got := engineStepAt(c.trade, "", c.within, confirmStart, c.ai)
			assertConfirming(t, got, append(slices.Clone(c.opening), c.reached(c.within))...)

			for _, entry := range []struct {
				name      string
				createdAt time.Time
				filledAt  *time.Time
				stale     bool
			}{
				{"a fill with only its CreatedAt, after the row", confirmStart.Add(30 * time.Second), nil, true},
				{"a resting add placed before the row and filled after it", confirmStart.Add(-10 * time.Second), stamped(confirmStart.Add(30 * time.Second)), true},
				{"a resting add placed and filled before the row", confirmStart.Add(-10 * time.Second), stamped(confirmStart.Add(-5 * time.Second)), false},
				{"an entry placed before the row with no FilledAt", confirmStart.Add(-10 * time.Second), nil, false},
			} {
				t.Run(entry.name, func(t *testing.T) {
					deeper := first
					deeper.History = append(slices.Clone(first.History), aggragates.TradesHistory{
						Type: "BUY", Quantity: exactUnits, Price: 100, OrderId: int64(len(first.History) + 1),
						CreatedAt: entry.createdAt, FilledAt: entry.filledAt,
					})
					deeper.PositionPrice = 100
					within, _ := flatMarketPrices(t, deeper)
					reached := TriggerReachedRow(c.gate, 100, -handLossPct(deeper, within), FullTrendTrigger())
					activation := stopLossActivatedRow(c.gate, 100, -handLossPct(deeper, within), FullTrendTrigger(), nil)

					printAt := confirmStart.Add(31 * time.Second)
					if age := printAt.Sub(confirmStart); age < confirmFor || age > confirmWindow {
						t.Fatalf("fixture drifted: the print must sit inside the old row's window and past its confirmation, got %v", age)
					}
					if !entry.stale {
						assertActivated(t, ApplyAt(deeper, "", within, c.ai, printAt), activation)
						return
					}
					assertConfirming(t, ApplyAt(deeper, "", within, c.ai, printAt), reached)
					renewed, _ := engineStepAt(deeper, "", within, printAt, c.ai)
					assertConfirming(t, ApplyAt(renewed, "", within, c.ai, printAt.Add(confirmFor/2)))
					assertActivated(t, ApplyAt(renewed, "", within, c.ai, printAt.Add(confirmFor)), activation)
				})
			}
			assertActivated(t, ApplyAt(first, "", c.within, c.ai, confirmStart.Add(31*time.Second)), c.activated(c.within))
		})
	}
}

// A take profit the ladder proposes in buy does not pass on the first reached print (it would block
// the activation behind the protected position for the rest of the episode): the print answers
// nothing, the confirming print activates in its place; a print short of the trigger leaves it standing
// with the held row, and no trigger standing leaves it untouched.
func TestATakeProfitDoesNotPassOnTheFirstReachedPrint(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			reached := c.reached(c.within)
			for _, position := range takeProfitProposals {
				assertConfirming(t, ApplyAt(c.trade, position, c.within, c.ai, confirmStart), append(slices.Clone(c.opening), reached)...)

				first, _ := engineStepAt(c.trade, position, c.within, confirmStart, c.ai)
				assertConfirming(t, ApplyAt(first, position, c.within, c.ai, confirmStart.Add(confirmFor/2)))
				assertActivated(t, ApplyAt(first, position, c.within, c.ai, confirmStart.Add(confirmFor)), c.activated(c.within))

				next, _ := engineStepAt(first, position, c.within, confirmStart.Add(confirmFor), c.ai)
				if next.PositionType != "stopLoss" || next.PositionPrice != c.within {
					t.Fatalf("%s: the confirming tick must leave the stopLoss anchored at the print, got %q at %v", position, next.PositionType, next.PositionPrice)
				}
				assertTrailHeld(t, ApplyAt(next, "", c.within, c.ai, confirmStart.Add(confirmFor+time.Second)))

				assertWaits(t, ApplyAt(first, position, c.over, c.ai, confirmStart.Add(time.Second)), position, c.held(c.over))
				goneTrade, goneAI := c.gone()
				assertUntouched(t, ApplyAt(goneTrade, position, c.within, goneAI, confirmStart), position)
			}
		})
	}
}

// A bound or the trend refusing a trigger that holds a reached stamp writes its held row by the old
// rules only (none once the episode is held) and cancels nothing: the stamp stands, and the reached
// print a confirmation after the row acts once the bound or the trend lets the trigger through.
func TestARefusalLeavesTheReachedPrintStanding(t *testing.T) {
	for _, gate := range saleGates {
		if !gate.bounded {
			continue
		}
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			held, _ := engineStepAt(c.trade, "", c.over, confirmStart.Add(-time.Second), c.ai)
			stamped, got := engineStepAt(held, "", c.within, confirmStart, c.ai)
			assertConfirming(t, got, c.reached(c.within))

			unread := c.reads([]int{0, 0, 0})
			filled := len(rebuildState(c.opened()).fills)
			for _, refusal := range []struct {
				name    string
				ai      aggragates.AIIndicators
				bound   func(t *testing.T)
				release func(t *testing.T)
			}{
				{"the trend", unread, func(*testing.T) {}, func(*testing.T) {}},
				{"the activation floor", c.ai, func(t *testing.T) { withSellLossStartAtDepth(t, filled+1) }, func(t *testing.T) { withSellLossStartAtDepth(t, 0) }},
			} {
				t.Run(refusal.name, func(t *testing.T) {
					refusal.bound(t)
					refused, got := engineStepAt(stamped, "", c.within, confirmStart.Add(confirmFor/2), refusal.ai)
					assertUntouched(t, got, "")
					if stamp := memoryOf(rebuildState(refused), c.gate).reachedAt; !stamp.Equal(confirmStart) {
						t.Fatalf("a refusal must leave the stamp standing, got %v", stamp)
					}
					assertUntouched(t, ApplyAt(refused, "", c.within, refusal.ai, confirmStart.Add(confirmFor)), "")
					refusal.release(t)
					assertActivated(t, ApplyAt(refused, "", c.within, c.ai, confirmStart.Add(confirmFor)), c.activated(c.within))
				})
			}
		})
	}
}

// profitStep judges every trigger on its own stamp and its own trigger: a hit acts alone (the others'
// reached rows are not written), fresh triggers write their reached rows after the due held rows,
// pending ones nothing, and a short trigger holding a stamp writes its held row though it is not due.
func TestEveryTriggerIsJudgedOnItsOwnReachedStamp(t *testing.T) {
	withSellLossIgnoreLastDepths(t, 0)
	withAllowedLoss(t, shippedLoss)
	farOverTheCap(t, shippedLoss)
	withConfirmation(t)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	p := roundLadderPrices(t, trade)
	now := confirmStart.Add(confirmFor)
	aged := func(trigger saleTrigger, age time.Duration) saleTrigger {
		trigger.reachedAt = now.Add(-age)
		return trigger
	}
	capital := func(due bool) saleTrigger {
		return saleTrigger{reason: reasonCapitalProtection, gate: GateCapitalProtection, terms: capitalProtectionTerms, rowDue: due,
			heldUnder: heldTrigger{pct: 0 - capitalProtectionMaxDrawdown, named: !due}}
	}
	within, over := lossPctOf(trade, p.within), lossPctOf(trade, p.over)
	reachedRow := func(trigger saleTrigger, loss, pct float64) Row {
		return TriggerReachedRow(trigger.gate, fill, 0-loss, pct, trigger.features...)
	}
	overTheCap := func(trigger saleTrigger) Row { return heldByLossRowOf(trigger, fill, over, shippedLoss) }
	slow, low := slowTrigger(false), lowTrigger(false)
	confirmed, half := confirmFor, confirmFor/2

	for _, c := range []struct {
		name     string
		state    string
		price    float64
		now      time.Time
		triggers []saleTrigger
		step     saleStep
		reason   string
		rows     []Row
	}{
		{"both fresh", "buy", p.within, now, []saleTrigger{slow, low}, stepHold, "",
			[]Row{reachedRow(slow, within, FullTrendTrigger()), reachedRow(low, within, FullTrendTrigger())}},
		{"one confirmed, one fresh: the hit acts alone", "buy", p.within, now, []saleTrigger{aged(slow, confirmed), low}, stepActivate, "",
			[]Row{activationRowOf(slow, fill, activatedText(within, shippedLoss))}},
		{"one fresh, one confirmed, in trigger order", "buy", p.within, now, []saleTrigger{slow, aged(low, confirmed)}, stepActivate, "",
			[]Row{activationRowOf(low, fill, activatedText(within, shippedLoss))}},
		{"both confirmed", "buy", p.within, now, []saleTrigger{aged(slow, confirmed), aged(low, 2*confirmed)}, stepActivate, "",
			[]Row{activationRowOf(slow, fill, activatedText(within, shippedLoss)), activationRowOf(low, fill, activatedText(within, shippedLoss))}},
		{"one pending, one fresh", "buy", p.within, now, []saleTrigger{aged(slow, half), low}, stepHold, "",
			[]Row{reachedRow(low, within, FullTrendTrigger())}},
		{"both pending", "buy", p.within, now, []saleTrigger{aged(slow, half), aged(low, confirmed-time.Nanosecond)}, stepHold, "", nil},
		{"no clock acts on a fresh trigger", "buy", p.within, time.Time{}, []saleTrigger{slow}, stepActivate, "",
			[]Row{activationRowOf(slow, fill, activatedText(within, shippedLoss))}},
		{"a short trigger with no stamp and its row written writes nothing", "buy", p.over, now, []saleTrigger{slow, low}, stepNone, "", nil},
		{"a short trigger holding a stamp writes nothing on account of it", "buy", p.over, now, []saleTrigger{aged(slow, half), low}, stepNone, "", nil},
		{"a short trigger holding a stamp still writes the row it is due", "buy", p.over, now, []saleTrigger{aged(slowTrigger(true), half), low}, stepNone, "",
			[]Row{overTheCap(slowTrigger(true))}},
		{"a stamp exactly the confirmation old confirms", "buy", p.within, now, []saleTrigger{aged(slow, confirmed)}, stepActivate, "",
			[]Row{activationRowOf(slow, fill, activatedText(within, shippedLoss))}},
		{"a stamp exactly the window old confirms", "buy", p.within, now, []saleTrigger{aged(slow, confirmWindow)}, stepActivate, "",
			[]Row{activationRowOf(slow, fill, activatedText(within, shippedLoss))}},
		{"a stamp older than the window is fresh again", "buy", p.within, now, []saleTrigger{aged(slow, confirmWindow+time.Nanosecond)}, stepHold, "",
			[]Row{reachedRow(slow, within, FullTrendTrigger())}},
		{"one stale, one confirmed: the hit acts alone", "buy", p.within, now, []saleTrigger{aged(slow, 2*confirmWindow), aged(low, confirmed)}, stepActivate, "",
			[]Row{activationRowOf(low, fill, activatedText(within, shippedLoss))}},
		{"a self-armed stopLoss: a stale trigger holds with a new row", "stopLoss", p.within, now, []saleTrigger{aged(slow, 2*confirmWindow)}, stepHold, "",
			[]Row{reachedRow(slow, within, FullTrendTrigger())}},
		{"a hit of another trigger leaves the short ones' rows unwritten", "buy", p.over, now, []saleTrigger{slowTrigger(true), aged(capital(false), confirmed)}, stepActivate, "",
			[]Row{{Message: "smartTakeLoss: capital protection stopLoss activated, " + activatedText(over, capitalProtectionMaxDrawdown), Price: fill, Gate: GateCapitalProtection, Event: EventActivated}}},
		{"a short trigger's due row first, a fresh one's reached row after", "buy", p.over, now, []saleTrigger{slowTrigger(true), capital(false)}, stepHold, "",
			[]Row{overTheCap(slowTrigger(true)), reachedRow(capital(false), over, 0-capitalProtectionMaxDrawdown)}},
		{"a pending trigger beside a short one with nothing due", "buy", p.over, now, []saleTrigger{slow, aged(capital(false), half)}, stepHold, "", nil},
		{"a self-armed stopLoss: a fresh trigger holds", "stopLoss", p.within, now, []saleTrigger{slow}, stepHold, "",
			[]Row{reachedRow(slow, within, FullTrendTrigger())}},
		{"a self-armed stopLoss: a pending trigger holds", "stopLoss", p.within, now, []saleTrigger{aged(slow, half)}, stepHold, "", nil},
		{"a self-armed stopLoss: a confirmed trigger sells under its reason", "stopLoss", p.within, now, []saleTrigger{slow, aged(low, confirmed)}, stepSell, lowReason, nil},
		{"a self-armed stopLoss: no clock sells at once", "stopLoss", p.within, time.Time{}, []saleTrigger{slow}, stepSell, slowReason, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			resting := restingIn(trade, c.state)
			rows, step, reason := sellLossAt(resting, rebuildState(resting), c.price, c.triggers, everyRowShared(), c.now)
			if step != c.step || reason != c.reason {
				t.Errorf("step = %s, reason = %q, want %s, %q", stepNames[step], reason, stepNames[c.step], c.reason)
			}
			if len(rows) != len(c.rows) || (len(c.rows) > 0 && !reflect.DeepEqual(rows, c.rows)) {
				t.Errorf("rows = %+v, want exactly %+v", rows, c.rows)
			}
		})
	}

	t.Run("a stamp before the newest fill is stale though no window runs", func(t *testing.T) {
		withActivationConfirmationWindow(t, 0)
		newest := testutil.At("12:00:00")
		resting := restingIn(trade, "buy")
		for _, c := range []struct {
			name  string
			stamp time.Time
			step  saleStep
			rows  []Row
		}{
			{"a hair before the fill", newest.Add(-time.Nanosecond), stepHold, []Row{reachedRow(slow, within, FullTrendTrigger())}},
			{"at the fill", newest, stepActivate, []Row{activationRowOf(slow, fill, activatedText(within, shippedLoss))}},
			{"hours after the fill, with no window", newest.Add(10 * time.Hour), stepActivate, []Row{activationRowOf(slow, fill, activatedText(within, shippedLoss))}},
		} {
			trigger := slow
			trigger.reachedAt = c.stamp
			rows, step, _ := sellLossAt(resting, rebuildState(resting), p.within, []saleTrigger{trigger}, everyRowShared(), now)
			if step != c.step || !reflect.DeepEqual(rows, c.rows) {
				t.Errorf("%s: step = %s with rows %+v, want %s with %+v", c.name, stepNames[step], rows, stepNames[c.step], c.rows)
			}
		}
	})

	t.Run("a committed sale trails whatever stamp it holds", func(t *testing.T) {
		committed := restingAt(trade, "stopLoss", 100)
		trigger := aged(activatedOf(lowTrigger(false)), half)
		for price, want := range map[float64]saleStep{99.9: stepHold, 99.7: stepSell, 101.2: stepReanchor} {
			rows, step, _ := sellLossAt(committed, rebuildState(committed), price, []saleTrigger{trigger}, everyRowShared(), now)
			if step != want || len(rows) != 0 {
				t.Errorf("at %v: step = %s with rows %+v, want %s and none", price, stepNames[step], rows, stepNames[want])
			}
		}
	})

	t.Run("a trade that used nothing keeps its answer", func(t *testing.T) {
		unused := testutil.LadderTrade(false)
		for state, want := range map[string]saleStep{"buy": stepNone, "stopLoss": stepSell} {
			resting := restingIn(unused, state)
			rows, step, _ := sellLossAt(resting, rebuildState(resting), 100, []saleTrigger{slow}, everyRowShared(), now)
			if step != want || len(rows) != 0 {
				t.Errorf("%s: step = %s with rows %+v, want %s and none", state, stepNames[step], rows, stepNames[want])
			}
		}
	})
}

// The fold reads a reached event into its gate's stamp, at the event's own stamp, and every event
// that moves the sale on (held by the profit or a bound, activated, sold, reset, the episode's
// opening, the slow decline's new pending or cancelled, the legacy ended) zeroes it. A reached
// event under another gate, with no price, or the slow decline's without features, stamps nothing.
func TestRebuildStateFoldsTheReachedStampOfEachSaleGate(t *testing.T) {
	for _, gate := range saleGates {
		t.Run(gate.name, func(t *testing.T) {
			c := gate.build(t)
			withConfirmation(t)
			stampOf := func(trade aggragates.Trades) time.Time { return memoryOf(rebuildState(trade), c.gate).reachedAt }
			stamped := Append(c.opened(), c.reached(c.within), confirmStart)
			if got := stampOf(stamped); !got.Equal(confirmStart) {
				t.Fatalf("a reached event must stamp the gate at its own CreatedAt, got %v", got)
			}
			if got := stampOf(Append(stamped, c.reached(c.within), confirmStart.Add(time.Second))); !got.Equal(confirmStart.Add(time.Second)) {
				t.Errorf("a newer reached event moves the stamp, got %v", got)
			}

			opens := quietPending(c.fill)
			if len(c.opening) > 0 {
				opens = c.opening[0]
			}
			resets := map[string]Row{
				GateFlatMarket:         FlatMarketResetRow(c.fill),
				GateFreeFallPrevention: FreeFallPreventionResetRow(c.fill),
				GateCapitalProtection:  CapitalProtectionResetRow(c.fill),
				GateSlowDecline:        ResetRow(c.fill, quietFinding(nil)),
			}
			clears := map[string]Row{
				"held by the profit": c.held(c.over),
				"held by a bound":    saleHeldRow(c.gate, c.fill, heldUnderDepth(1, 2), c.features),
				"activated":          c.activated(c.within),
				"sold":               ExitRow(c.trade, c.reason, c.within),
				"reset":              resets[c.gate],
				"the opening event":  opens,
			}
			if c.gate == GateSlowDecline {
				clears["cancelled"] = CancelledRow(c.fill, quietFinding(nil))
			}
			for name, row := range clears {
				if got := stampOf(Append(stamped, row, confirmStart.Add(time.Minute))); !got.IsZero() {
					t.Errorf("%s must zero the stamp, got %v", name, got)
				}
			}
			if c.gate == GateFlatMarket {
				ended := eventOf(gateLowInterest, legacyDocument(EventEnded, c.fill))
				if got := stampOf(withEvents(stamped, ended)); !got.IsZero() {
					t.Errorf("the legacy ended event must zero the stamp, got %v", got)
				}
			}

			other := GateFlatMarket
			if c.gate == other {
				other = GateCapitalProtection
			}
			if got := stampOf(Append(c.opened(), TriggerReachedRow(other, c.fill, -0.1, -0.15), confirmStart)); !got.IsZero() {
				t.Errorf("a reached event of %s must not stamp %s, got %v", other, c.gate, got)
			}
			noPrice := eventOf(c.gate, `{"event":"reached","trigger":-0.15}`)
			noPrice.CreatedAt = confirmStart
			if got := stampOf(withEvents(c.opened(), noPrice)); !got.IsZero() {
				t.Errorf("a reached event with no price must stamp nothing, got %v", got)
			}
			if c.gate == GateSlowDecline {
				bare := eventOf(GateSlowDecline, `{"event":"reached","price":`+jsonNumber(c.fill)+`,"trigger":-0.15}`)
				bare.CreatedAt = confirmStart
				if got := stampOf(withEvents(c.opened(), bare)); !got.IsZero() {
					t.Errorf("a slow decline reached event without features is never folded, got %v", got)
				}
			}
		})
	}
}

// jsonNumber is a number as a JSON document writes it.
func jsonNumber(value float64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// The reached row is "smartTakeLoss: <gate> trigger reached, profit <p>% of the used amount at or
// over <trigger>%, the activation confirms after <ActivationConfirmation>" at the newest fill under
// its gate, its event reached carrying the trigger (zero included) and the fill, features cloned.
func TestTheTriggerReachedRowIsTheGrammarOfEachGate(t *testing.T) {
	withConfirmation(t)
	features := []string{FeatureQuietDecline, FeatureSwingDecline}
	for gate, label := range map[string]string{
		GateSlowDecline: "slow decline", GateFlatMarket: "flat market",
		GateFreeFallPrevention: "free fall prevention", GateCapitalProtection: "capital protection",
	} {
		trigger := -0.15
		row := TriggerReachedRow(gate, 12.5, -0.1, trigger, features...)
		want := Row{
			Message:  "smartTakeLoss: " + label + " trigger reached, profit -0.10% of the used amount at or over -0.15%, the activation confirms after 5s",
			Price:    12.5,
			Gate:     gate,
			Event:    EventReached,
			Features: []string{FeatureQuietDecline, FeatureSwingDecline},
			Trigger:  &trigger,
		}
		if !reflect.DeepEqual(row, want) {
			t.Errorf("%s: row = %+v, want %+v", gate, row, want)
		}
	}

	withActivationConfirmation(t, 90*time.Second)
	if got := TriggerReachedRow(GateFlatMarket, 12.5, 0.1, 0.25).Message; !strings.HasSuffix(got, "profit 0.10% of the used amount at or over 0.25%, the activation confirms after 1m30s") {
		t.Errorf("the row must name the confirmation in force, got %q", got)
	}

	features[0] = "changed"
	if row := TriggerReachedRow(GateSlowDecline, 12.5, -0.1, -0.15, features...); !slices.Equal(row.Features, []string{"changed", FeatureSwingDecline}) {
		t.Fatalf("fixture drifted: the row must carry the features it is given, got %+v", row.Features)
	}
	row := TriggerReachedRow(GateSlowDecline, 12.5, -0.1, -0.15, features...)
	features[1] = "changed too"
	if row.Features[1] != FeatureSwingDecline {
		t.Errorf("the row must clone its features, got %v", row.Features)
	}
	if got := TriggerReachedRow(GateFlatMarket, 12.5, -0.1, -0.15); got.Features != nil {
		t.Errorf("a row with no features carries none, got %#v", got.Features)
	}

	zero := TriggerReachedRow(GateFlatMarket, 12.5, 0.1, 0)
	_, event := Rows(flatMarketLadder(false), zero, confirmStart)
	data, ok := readEvent(event)
	if !ok || data.Event != EventReached || data.Price != 12.5 || data.Trigger == nil || *data.Trigger != 0 || !strings.Contains(string(event.Data), `"trigger":0`) {
		t.Errorf("the reached event must carry the fill as price and a zero trigger, got %+v (%s)", data, event.Data)
	}
}
