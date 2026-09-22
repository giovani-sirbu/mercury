package actions

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// A wallet on a taller grid than the other files use, with the deepest ladder
// and its closest sibling one depth apart, run end to end through ShouldHold.
//
// What it is here for is the arithmetic between the two halves of the rule.
// The gate subtracts one helper's answer (what this entry spends) from the
// balance and compares the rest against another's (what the reserved ladder
// still needs); the two are written apart, in separate files, and a ladder
// that is priced forward by one of them and backward by the other would leave
// the boundary silently in the wrong place. So every number below is read off
// the ladders through those helpers rather than written out, and the reserve
// itself is checked against the entries it is supposed to be the sum of.

// reserveGridDepths is the grid every pair of this wallet is configured for.
// Taller than the wallet the other files describe, so the deepest ladder has
// more than one entry left and its remainder is a sum rather than a single
// price.
const reserveGridDepths = 9

// reserveGridAsset is what every long ladder of this wallet spends.
const reserveGridAsset = "USDT"

// reserveGridShape is the wallet: the deepest ladder, the sibling one depth
// behind it, and three shallower ones. Ordered deepest first, so an
// expectation that names a ladder can be read against the shape it is
// written for.
var reserveGridShape = []struct {
	id     uint
	symbol string
	depth  int
}{
	{14, "LINK/USDT", 7},
	{13, "SOL/USDT", 6},
	{11, "BTC/USDT", 5},
	{12, "ETH/USDT", 3},
	{15, "HBAR/USDT", 1},
}

// reserveGridTrade is one ladder of that wallet as a trade.
func reserveGridTrade(id uint, symbol string, depth int) aggragates.Trades {
	return testutil.LadderDepthTrade(id, symbol, depth, reserveGridDepths)
}

// reserveGridTrades builds the whole wallet.
func reserveGridTrades() []aggragates.Trades {
	trades := make([]aggragates.Trades, 0, len(reserveGridShape))
	for _, entry := range reserveGridShape {
		trades = append(trades, reserveGridTrade(entry.id, entry.symbol, entry.depth))
	}
	return trades
}

// reserveGridView is the view an engine hands the tick, mapped through the
// helper all four surfaces build their view with.
func reserveGridView(trades []aggragates.Trades) []aggragates.LadderDepth {
	view := make([]aggragates.LadderDepth, 0, len(trades))
	for _, trade := range trades {
		view = append(view, ladder.DepthOf(trade))
	}
	return view
}

// reserveGridWithout is the view an engine builds once a ladder blocks on its
// next entry: the trade goes on ticking, it is only out of the wallet the
// gate reads.
func reserveGridWithout(trades []aggragates.Trades, symbol string) []aggragates.LadderDepth {
	view := make([]aggragates.LadderDepth, 0, len(trades))
	for _, trade := range trades {
		if trade.Symbol == symbol {
			continue
		}
		view = append(view, ladder.DepthOf(trade))
	}
	return view
}

// reserveGridTradeOf is one named ladder of the wallet, read back from the
// fixture so an expectation cannot drift from the shape it describes.
func reserveGridTradeOf(t *testing.T, trades []aggragates.Trades, symbol string) aggragates.Trades {
	t.Helper()

	for _, trade := range trades {
		if trade.Symbol == symbol {
			return trade
		}
	}

	t.Fatalf("%s is not a ladder of this wallet", symbol)
	return aggragates.Trades{}
}

// reserveGridKeeper is the ladder the view keeps the wallet for: the deepest
// one still short of its ceiling, with the amount it still needs.
func reserveGridKeeper(t *testing.T, view []aggragates.LadderDepth) aggragates.LadderDepth {
	t.Helper()

	var keeper aggragates.LadderDepth
	found := false

	for _, candidate := range view {
		if candidate.Asset != reserveGridAsset || candidate.MaxDepth <= 0 {
			continue
		}
		if candidate.Depth < 1 || candidate.Depth >= candidate.MaxDepth {
			continue
		}
		if !found || candidate.Depth > keeper.Depth {
			keeper, found = candidate, true
		}
	}

	if !found {
		t.Fatal("this wallet must have a ladder with entries left to fill")
	}
	if keeper.RemainingCost <= 0 {
		t.Fatalf("%s must name what its remaining depths cost", keeper.Symbol)
	}

	return keeper
}

// reserveGridRow is the row a waiting ladder of this wallet carries.
func reserveGridRow(position, keeper string, keeperDepth, ownDepth int) string {
	return fmt.Sprintf(
		"Hold %s: cooldown: depth priority, %s at depth %d of %d keeps the wallet for its remaining depths, this ladder waits at depth %d of %d",
		position, keeper, keeperDepth, reserveGridDepths, ownDepth, reserveGridDepths,
	)
}

// reserveGridClosingRow is the row a ladder carries while it waits behind one
// that has nothing left to fill: no further entry of that ladder will free the
// funds, so the row names the close instead of a remainder.
func reserveGridClosingRow(position, keeper string, keeperDepth, ownDepth int) string {
	return fmt.Sprintf(
		"Hold %s: cooldown: depth priority, %s at depth %d of %d holds the wallet until it closes, this ladder waits at depth %d of %d",
		position, keeper, keeperDepth, reserveGridDepths, ownDepth, reserveGridDepths,
	)
}

// reserveGridSettling is the same ladder caught mid-settlement: the exchange
// has given its deepest entry part of the quantity that entry is for, and the
// engine has written that part onto the entry's OWN row — the row it goes on
// raising until the order completes. One row per entry, so the ladder is at
// the same depth throughout; only the amount on the deepest row moves.
func reserveGridSettling(trade aggragates.Trades, executed float64) aggragates.Trades {
	settling := trade
	settling.History = append([]aggragates.TradesHistory(nil), trade.History...)

	last := len(settling.History) - 1
	settling.History[last].Quantity = executed
	settling.History[last].Status = "PARTIALLY_FILLED"

	return settling
}

// reserveGridInverse is a ladder of the same wallet on the other side: an
// inverse trade enters by SELLING base, so its entries are SELL rows and
// everything it spends is counted in the base asset.
func reserveGridInverse(id uint, symbol string, depth int) aggragates.Trades {
	trade := reserveGridTrade(id, symbol, depth)
	trade.Inverse = true

	for index := range trade.History {
		trade.History[index].Type = "SELL"
	}

	return trade
}

// What is kept for the deepest ladder is the entries it has left AT THE
// PRICES ITS GRID WAS PLANNED AGAINST: each remaining entry multiplies the
// quantity up by its row's multiplier and steps the price down by that row's
// percentage, which is the arithmetic the initial bid was sized with.
//
// Two readings of the same tail, because the two are what the gate subtracts
// from each other. The first walks the plan directly. The second asks
// NextEntryCost — the helper that prices the managed trade's own entry — what
// each of those entries would cost at the price the grid planned it for, and
// the sums have to meet: the reserve is what those entries cost, not a
// separate amount computed a second way.
//
// The step down is the whole correction this file was realigned for. Priced
// at TODAY's price the tail compounds against a level the ladder has already
// left, and one ladder's remainder can exceed the entire wallet from its
// first depth — a reserve nobody can ever satisfy, which holds every sibling
// out of its first fill instead of keeping money for a ladder that will spend
// it.
func TestReserveIsTheEntriesTheLadderHasLeftAtThePricesTheGridPlanned(t *testing.T) {
	keeper := reserveGridShape[0]
	trade := reserveGridTrade(keeper.id, keeper.symbol, keeper.depth)

	_, reserve := ladder.RemainingCost(trade)
	if reserve <= 0 {
		t.Fatalf("reserve = %f, want the cost of the depths this ladder has left", reserve)
	}

	row := trade.StrategyPair.StrategySettings[0]
	step := 1 - row.Percentage/100
	if step <= 0 || step >= 1 {
		t.Fatalf("the fixture grid must step the price down between depths, got a factor of %v", step)
	}

	// The plan itself: quantity up by the multiplier, price down by the
	// percentage, one depth at a time.
	history := append([]aggragates.TradesHistory(nil), trade.History...)
	quantity := ladder.GetLatestQuantityByHistory(history, "BUY")
	price := trade.PositionPrice
	planned := 0.0

	for depth := keeper.depth; depth < reserveGridDepths; depth++ {
		quantity *= row.Multiplier
		price *= step
		planned += quantity * price
	}

	testutil.AssertFloatEqual(t, reserve, planned, 1e-9, "the reserve against the budget the grid was planned with")

	// The same entries through the helper that prices the OTHER side of the
	// gate's subtraction, each asked at the price its depth was planned for.
	entries := 0.0
	stepped := trade.PositionPrice

	for depth := keeper.depth; depth < reserveGridDepths; depth++ {
		stepped *= step

		planPrice := reserveGridTrade(keeper.id, keeper.symbol, depth)
		planPrice.PositionPrice = stepped

		_, entry := ladder.NextEntryCost(planPrice, 0)
		if entry <= 0 {
			t.Fatalf("depth %d of %d: the next entry must be priced, got %f", depth, reserveGridDepths, entry)
		}
		entries += entry
	}

	// NextEntryCost fits a long entry to the pair's lot size and the walk
	// above does not, so the two may part by half a lot step per entry —
	// which is the fit itself, not a disagreement about the tail.
	lotStep := math.Pow(10, -float64(trade.StrategyPair.TradeFilters.LotSize))
	tolerance := lotStep * float64(reserveGridDepths-keeper.depth)

	testutil.AssertFloatEqual(t, entries, reserve, tolerance, "the reserve against the entries it is the sum of")
}

// The boundary the rule names, on the sibling one depth behind the deepest
// ladder: it buys while the wallet still covers the reserve after its own
// entry, and waits the moment it would not. One unit of quote either side of
// the line, far above float noise and far below any entry of this ladder.
func TestReserveLetsTheSiblingThroughUntilItsEntryWouldBreakIn(t *testing.T) {
	trades := reserveGridTrades()
	view := reserveGridView(trades)

	keeper := reserveGridKeeper(t, view)
	if keeper.Symbol != reserveGridShape[0].symbol {
		t.Fatalf("the wallet is kept for %s, want %s", keeper.Symbol, reserveGridShape[0].symbol)
	}

	sibling := reserveGridTradeOf(t, trades, reserveGridShape[1].symbol)
	_, ownCost := ladder.NextEntryCost(sibling, keeper.RemainingCost)
	if ownCost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", sibling.Symbol)
	}

	level := keeper.RemainingCost + ownCost

	released, err := ShouldHold(priorityEvent(sibling, "buy", view, level))
	if err != nil {
		t.Fatalf("a wallet level with the reserve must let the entry through, got %v", err)
	}
	if len(released.Trade.Logs) != 0 {
		t.Fatalf("no row may be written on a released ladder, got %v", messages(released.Trade.Logs))
	}

	held, err := ShouldHold(priorityEvent(sibling, "buy", view, level-1))
	if err == nil {
		t.Fatal("a wallet one unit under the reserve must hold the entry")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	want := reserveGridRow("stopLoss", keeper.Symbol, keeper.Depth, reserveGridShape[1].depth)
	if got := held.Trade.Logs[0].Message; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}
	if held.Trade.Logs[0].Type != aggragates.LOG_INFO {
		t.Errorf("row type = %q, want %q", held.Trade.Logs[0].Type, aggragates.LOG_INFO)
	}
}

// The ladder the wallet is kept for takes its own entry on any balance the
// gate is handed, including an empty one: the funds being kept are its own,
// and the gate it would be held by is its own reservation. What stops it is
// the funds gate further down the chain, which is the one that knows whether
// the money is really there.
func TestReserveNeverHoldsTheLadderItIsKeptFor(t *testing.T) {
	trades := reserveGridTrades()
	view := reserveGridView(trades)

	keeper := reserveGridKeeper(t, view)
	own := reserveGridTradeOf(t, trades, keeper.Symbol)
	_, ownCost := ladder.NextEntryCost(own, keeper.RemainingCost)

	for _, free := range []float64{0, 1, keeper.RemainingCost - 1, keeper.RemainingCost, keeper.RemainingCost + ownCost} {
		released, err := ShouldHold(priorityEvent(own, "buy", view, free))
		if err != nil {
			t.Fatalf("%s must take its own entry on a wallet of %f, got %v", own.Symbol, free, err)
		}
		if len(released.Trade.Logs) != 0 {
			t.Fatalf("wallet of %f: no row may be written on the reserved ladder, got %v", free, messages(released.Trade.Logs))
		}
	}
}

// A trade opened into this wallet has no fill to multiply, so what it would
// spend is the ladder's initial bid off the very balance it is being sized
// against — and the gate weighs that bid, not a depth. Both sides are pinned:
// the wallet that cannot carry the bid and the reserve together writes the
// entry row, and the wallet that can lets the first fill through.
func TestReservePricesATradeOpenedIntoTheWalletAtItsInitialBid(t *testing.T) {
	view := reserveGridView(reserveGridTrades())
	keeper := reserveGridKeeper(t, view)

	newcomer := reserveGridTrade(21, "ADA/USDT", 0)
	newcomer.PositionType = "buy"

	if _, bid := ladder.NextEntryCost(newcomer, keeper.RemainingCost); bid <= 0 {
		t.Fatalf("a first fill must be priced, got %f", bid)
	}

	held, err := ShouldHold(priorityEvent(newcomer, "new", view, keeper.RemainingCost))
	if err == nil {
		t.Fatal("a first fill the reserved wallet cannot spare must wait")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}

	want := reserveGridRow("entry", keeper.Symbol, keeper.Depth, 0)
	if got := held.Trade.Logs[0].Message; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}

	// The wallet the same first fill goes through on. The bid grows with the
	// balance it is sized against, so the balance that carries both is found
	// by feeding each answer back in; what the assertion rests on is the
	// invariant checked right after, never the search.
	free := keeper.RemainingCost
	for attempt := 0; attempt < 16; attempt++ {
		_, bid := ladder.NextEntryCost(newcomer, free)
		free = (keeper.RemainingCost + bid) * 1.01
	}

	_, bid := ladder.NextEntryCost(newcomer, free)
	if free-bid < keeper.RemainingCost {
		t.Fatalf("a wallet of %f leaves %f after a bid of %f, want the reserve of %f covered", free, free-bid, bid, keeper.RemainingCost)
	}

	opened, err := ShouldHold(priorityEvent(newcomer, "new", view, free))
	if err != nil {
		t.Fatalf("a wallet that carries the bid and the reserve must let a first fill through, got %v", err)
	}
	if len(opened.Trade.Logs) != 0 {
		t.Fatalf("no row may be written on a released first fill, got %v", messages(opened.Trade.Logs))
	}
}

// The ladder the wallet was kept for blocks on the entry it was being kept
// for, the engines drop it from the view, and the ladder that was waiting
// arms on the next tick. What it is then weighed against is the NEXT deepest
// ladder's OWN remainder, which is a different amount — never the one the
// ladder that left needed.
//
// Only that it differs is asserted, not which way. The remainder trades a
// quantity that grows per depth against a price that falls per depth, so
// whether handing the reservation on raises or lowers it depends on the
// multiplier and the percentage a pair is tuned with; pinning a direction
// here would pin somebody's tuning.
func TestReserveHandsOnToTheNextDeepestLadderWhenTheKeeperLeaves(t *testing.T) {
	trades := reserveGridTrades()
	view := reserveGridView(trades)

	keeper := reserveGridKeeper(t, view)
	waiting := reserveGridTradeOf(t, trades, reserveGridShape[2].symbol)
	_, ownCost := ladder.NextEntryCost(waiting, keeper.RemainingCost)

	held, err := ShouldHold(priorityEvent(waiting, "buy", view, keeper.RemainingCost+ownCost-1))
	if err == nil {
		t.Fatalf("%s must wait while %s is in the view", waiting.Symbol, keeper.Symbol)
	}
	want := reserveGridRow("stopLoss", keeper.Symbol, keeper.Depth, reserveGridShape[2].depth)
	if got := held.Trade.Logs[0].Message; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}

	released := reserveGridWithout(trades, keeper.Symbol)
	next := reserveGridKeeper(t, released)
	if next.Symbol != reserveGridShape[1].symbol {
		t.Fatalf("the wallet is now kept for %s, want %s", next.Symbol, reserveGridShape[1].symbol)
	}
	if next.RemainingCost == keeper.RemainingCost {
		t.Fatalf(
			"handing the reservation on left %f to keep, the same amount the ladder that left needed: the reserve must be the new keeper's own remainder",
			next.RemainingCost,
		)
	}

	// The tick after the hold: the ladder arms the same depth again, and the
	// row it is already carrying is the only one it has.
	standing := held.Trade
	standing.PositionType = "stopLoss"

	armed, err := ShouldHold(priorityEvent(standing, "buy", released, next.RemainingCost+ownCost))
	if err != nil {
		t.Fatalf("%s must arm its next entry once the blocked ladder is out of the wallet, got %v", waiting.Symbol, err)
	}
	if len(armed.Trade.Logs) != 1 {
		t.Fatalf("a released ladder writes no further row, got %v", messages(armed.Trade.Logs))
	}

	// One unit under the new reservation it waits again, for the ladder that
	// is actually there.
	stillShort, err := ShouldHold(priorityEvent(waiting, "buy", released, next.RemainingCost+ownCost-1))
	if err == nil {
		t.Fatalf("%s must wait for %s once it keeps the wallet", waiting.Symbol, next.Symbol)
	}
	want = reserveGridRow("stopLoss", next.Symbol, next.Depth, reserveGridShape[2].depth)
	if got := stillShort.Trade.Logs[0].Message; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}
}

// A reservation that stands for many ticks is one row, not one per tick. The
// balance moves on every tick and the row carries none of it, which is the
// property gates.SaveHoldLog deduplicates on — so a ladder held across a
// drifting wallet writes the row once and an operator reads a reservation
// instead of a wall of near-identical lines.
func TestReserveWritesOneRowWhileTheHoldStands(t *testing.T) {
	trades := reserveGridTrades()
	view := reserveGridView(trades)

	keeper := reserveGridKeeper(t, view)
	waiting := reserveGridTradeOf(t, trades, reserveGridShape[1].symbol)
	_, ownCost := ladder.NextEntryCost(waiting, keeper.RemainingCost)

	short := keeper.RemainingCost + ownCost - 1
	standing := waiting
	first := ""

	// Three ticks of the same standing hold, the balance drifting either way
	// underneath it the way a live one does.
	for tick, free := range []float64{short, short - 1, short - 0.5} {
		standing.PositionType = "stopLoss"

		held, err := ShouldHold(priorityEvent(standing, "buy", view, free))
		if err == nil {
			t.Fatalf("tick %d: the ladder must still be held", tick)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("tick %d: a standing hold must not write a row per tick, got %v", tick, messages(held.Trade.Logs))
		}
		if tick == 0 {
			first = held.Trade.Logs[0].Message
		}
		if got := held.Trade.Logs[0].Message; got != first {
			t.Fatalf("tick %d: row = %q, want the row from the first tick %q", tick, got, first)
		}

		standing = held.Trade
	}
}

// The reservation does not shrink while the reserved ladder's deepest entry
// is still settling, and the siblings it holds stay held for the whole of
// that window.
//
// An entry arrives in parts and the engine raises the quantity on that
// entry's own row as they land, so for the seconds between the first part and
// the last one the deepest row carries LESS than the row above it. Sized off
// the largest quantity that has executed, the ladder reads one depth
// shallower than it is and the wallet is kept for roughly one depth less —
// and a sibling arming inside that window passes a gate that should have held
// it. What it then spends is precisely what the reserved ladder is about to
// need for its own last depth, which is the failure the reserve exists to
// prevent, opened by the reserve itself.
//
// Both ends are covered: a part under the entry above it, and a part under
// the ladder's very FIRST entry, where the settling row is the smallest
// quantity in the whole history. The planned size is the same either way,
// because the plan is the first entry taken through the multipliers and never
// a reading of what has executed.
func TestReserveKeepsItsSizeWhileTheDeepestEntryIsStillSettling(t *testing.T) {
	keeper := reserveGridShape[0]
	full := reserveGridTrade(keeper.id, keeper.symbol, keeper.depth)

	settled := ladder.DepthOf(full)
	if settled.RemainingCost <= 0 {
		t.Fatalf("the reserved ladder must name what its remaining depths cost, got %f", settled.RemainingCost)
	}

	planned := full.History[len(full.History)-1].Quantity
	above := full.History[len(full.History)-2].Quantity
	first := full.History[0].Quantity

	sibling := reserveGridTrade(reserveGridShape[1].id, reserveGridShape[1].symbol, reserveGridShape[1].depth)
	_, ownCost := ladder.NextEntryCost(sibling, settled.RemainingCost)
	if ownCost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", sibling.Symbol)
	}

	for name, executed := range map[string]float64{
		"part of its own quantity, under the entry above it": above / 2,
		"less than the ladder's very first entry":            first / 4,
	} {
		if executed >= planned {
			t.Fatalf("%s: the fixture must leave the deepest row under its planned %v, got %v", name, planned, executed)
		}

		settling := ladder.DepthOf(reserveGridSettling(full, executed))

		if settling.Depth != settled.Depth || settling.MaxDepth != settled.MaxDepth {
			t.Fatalf("%s: a settling entry is still one depth, got %+v want %+v", name, settling, settled)
		}
		if settling.RemainingCost != settled.RemainingCost {
			t.Fatalf(
				"%s: a settling ladder keeps %v of the wallet, want the %v it keeps once that entry completes",
				name, settling.RemainingCost, settled.RemainingCost,
			)
		}

		// And through the gate, where the difference would have been spent:
		// the sibling held against the completed ladder is held against the
		// settling one, and the sibling free against the completed ladder is
		// free against the settling one.
		wallet := []aggragates.LadderDepth{settling}

		held, err := ShouldHold(priorityEvent(sibling, "buy", wallet, settling.RemainingCost+ownCost-1))
		if err == nil {
			t.Fatalf("%s: %s must wait while the reserved ladder settles its deepest entry", name, sibling.Symbol)
		}
		want := reserveGridRow("stopLoss", keeper.symbol, keeper.depth, reserveGridShape[1].depth)
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s: row = %q, want %q", name, got, want)
		}

		released, err := ShouldHold(priorityEvent(sibling, "buy", wallet, settling.RemainingCost+ownCost))
		if err != nil {
			t.Fatalf("%s: a wallet that covers the reservation must release the entry, got %v", name, err)
		}
		if len(released.Trade.Logs) != 0 {
			t.Fatalf("%s: no row may be written on a released ladder, got %v", name, messages(released.Trade.Logs))
		}
	}
}

// A ladder that has filled its last depth stays in front of the wallet until
// it CLOSES, and the siblings it stands in front of wait rather than spend.
//
// It reserves nothing — there is no entry left to keep funds for — so the
// only question left is whether the wallet can pay for the entry being
// weighed: it can, the sibling buys, and that is the release at the ceiling.
// It cannot, and the sibling is HELD.
//
// Held, not blocked, is the whole of it. A sibling released into a wallet the
// full ladder has just spent down reaches the funds gate instead, and a
// funds-blocked ladder leaves the view the engines build — so when the close
// finally refills the wallet, the deepest siblings are no longer in it and
// the shallowest ladder still active inherits the wallet over them. Holding
// keeps each sibling active, in the view, at its own rank, and lets the close
// hand the wallet on in the order the depths say.
func TestReserveHoldsSiblingsBehindAFullLadderUntilItCloses(t *testing.T) {
	const fullSymbol = "LINK/USDT"

	full := reserveGridTrade(14, fullSymbol, reserveGridDepths)
	fullView := ladder.DepthOf(full)
	if fullView.Depth != fullView.MaxDepth {
		t.Fatalf("the fixture must fill its last depth, got %+v", fullView)
	}
	if fullView.RemainingCost != 0 {
		t.Fatalf("a ladder at its ceiling reserves %v, want nothing left to keep", fullView.RemainingCost)
	}

	// Two siblings level at a shallower depth — so the handover after the
	// close is split on the trade id — and one shallower still, so the
	// wallet can afford one entry without affording the others.
	deeper := reserveGridTrade(13, "SOL/USDT", 5)
	level := reserveGridTrade(16, "ETH/USDT", 5)
	shallow := reserveGridTrade(15, "HBAR/USDT", 1)

	siblings := []aggragates.Trades{deeper, level, shallow}
	depthOf := map[string]int{deeper.Symbol: 5, level.Symbol: 5, shallow.Symbol: 1}

	costs := map[string]float64{}
	cheapest := 0.0
	for _, sibling := range siblings {
		_, cost := ladder.NextEntryCost(sibling, 0)
		if cost <= 0 {
			t.Fatalf("%s must cost something to place its next entry", sibling.Symbol)
		}
		costs[sibling.Symbol] = cost
		if cheapest == 0 || cost < cheapest {
			cheapest = cost
		}
	}

	view := []aggragates.LadderDepth{fullView, ladder.DepthOf(deeper), ladder.DepthOf(level), ladder.DepthOf(shallow)}

	// A wallet under the cheapest entry of the wallet: every sibling waits,
	// and waits on the row that names the close.
	for _, sibling := range siblings {
		held, err := ShouldHold(priorityEvent(sibling, "buy", view, cheapest-1))
		if err == nil {
			t.Fatalf("%s must wait while %s holds the wallet", sibling.Symbol, fullSymbol)
		}
		if !errors.Is(err, events.ErrTradeHeld) {
			t.Fatalf("%s: err = %v, want the ladder HELD rather than stopped any other way", sibling.Symbol, err)
		}
		if len(held.Trade.Logs) != 1 {
			t.Fatalf("%s: expected one row, got %v", sibling.Symbol, messages(held.Trade.Logs))
		}

		want := reserveGridClosingRow("stopLoss", fullSymbol, reserveGridDepths, depthOf[sibling.Symbol])
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", sibling.Symbol, got, want)
		}
	}

	// Raised to exactly what the cheapest entry costs, that one ladder buys
	// while the others go on waiting: the full ladder keeps nothing back, so
	// what decides is only whether the wallet covers the entry.
	for _, sibling := range siblings {
		released, err := ShouldHold(priorityEvent(sibling, "buy", view, cheapest))

		if costs[sibling.Symbol] > cheapest {
			if err == nil {
				t.Fatalf("%s costs more than the wallet holds and must still wait", sibling.Symbol)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s costs what the wallet holds and must buy, got %v", sibling.Symbol, err)
		}
		if len(released.Trade.Logs) != 0 {
			t.Fatalf("%s: no row may be written on a released ladder, got %v", sibling.Symbol, messages(released.Trade.Logs))
		}
	}

	// The close: the full ladder leaves the view and the wallet comes back.
	// The wallet is now in front of the deepest ladder LEFT — split from its
	// level twin on the trade id — and that one waits for nobody while the
	// others wait behind it, on the row that names a remainder again.
	closed := []aggragates.LadderDepth{ladder.DepthOf(deeper), ladder.DepthOf(level), ladder.DepthOf(shallow)}

	next := reserveGridKeeper(t, closed)
	if next.Symbol != deeper.Symbol {
		t.Fatalf("the wallet is now in front of %s, want the lower trade id of the level pair (%s)", next.Symbol, deeper.Symbol)
	}

	assertFreeToArm(t, deeper, closed, next.RemainingCost+costs[deeper.Symbol]-1)

	for _, sibling := range []aggragates.Trades{level, shallow} {
		held, err := ShouldHold(priorityEvent(sibling, "buy", closed, next.RemainingCost+costs[sibling.Symbol]-1))
		if err == nil {
			t.Fatalf("%s must wait behind %s once the full ladder closes", sibling.Symbol, next.Symbol)
		}

		want := reserveGridRow("stopLoss", next.Symbol, next.Depth, depthOf[sibling.Symbol])
		if got := held.Trade.Logs[0].Message; got != want {
			t.Fatalf("%s row = %q, want %q", sibling.Symbol, got, want)
		}
	}
}

// The managed trade is ranked against the view by the same ordering as every
// ladder in it, so a trade that is DEEPER than anything the view holds waits
// for nobody — and it does not have to be in the view to say so.
//
// That is the case a re-admitted ladder lands in. Blocked on its next entry
// it is out of the view the engines build, and on the tick the wallet can
// afford that entry again it is judged against a view it is still missing
// from. Read from the view alone it would find a shallower ladder in front of
// it and wait there, which inverts the very ranking the gate exists to keep.
func TestReserveRanksTheManagedTradeAgainstTheViewItIsMissingFrom(t *testing.T) {
	shallower := reserveGridTrade(11, "BTC/USDT", 4)
	view := []aggragates.LadderDepth{ladder.DepthOf(shallower)}

	// Deeper than anything in the view, and absent from it.
	deepest := reserveGridTrade(14, "LINK/USDT", 5)
	for _, free := range []float64{0, 1} {
		assertFreeToArm(t, deepest, view, free)
	}

	// Level with the view's ladder instead, and holding the higher id: the
	// split goes the other way and it waits.
	levelBehind := reserveGridTrade(14, "LINK/USDT", 4)
	_, ownCost := ladder.NextEntryCost(levelBehind, 0)

	keeper := reserveGridKeeper(t, view)
	held, err := ShouldHold(priorityEvent(levelBehind, "buy", view, keeper.RemainingCost+ownCost-1))
	if err == nil {
		t.Fatalf("a level ladder with the higher id must wait for %s", shallower.Symbol)
	}

	want := reserveGridRow("stopLoss", shallower.Symbol, keeper.Depth, 4)
	if got := held.Trade.Logs[0].Message; got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}
}

// Ladders funded from different currencies never take money from each other.
// An inverse ladder spends the BASE side of its pair, so however deep it is
// it keeps a wallet the long ladders of the same exchange never spend from —
// and the long ladders are weighed against the deepest LONG one instead. A
// gate that ranked the view by depth alone would keep a quote wallet for a
// ladder that spends base.
func TestReserveNeverKeepsAQuoteWalletForAnInverseLadder(t *testing.T) {
	deepInverse := reserveGridInverse(31, "BTC/USDT", 8)
	inverseView := ladder.DepthOf(deepInverse)
	if inverseView.Asset != "BTC" {
		t.Fatalf("an inverse ladder spends %q, want the base side of its pair", inverseView.Asset)
	}
	if inverseView.RemainingCost <= 0 {
		t.Fatal("the inverse fixture must name what its remaining depths cost")
	}

	longKeeper := reserveGridTrade(13, reserveGridShape[1].symbol, reserveGridShape[1].depth)
	view := []aggragates.LadderDepth{inverseView, ladder.DepthOf(longKeeper)}

	keeper := reserveGridKeeper(t, view)
	if keeper.Symbol != reserveGridShape[1].symbol {
		t.Fatalf("the quote wallet is kept for %s, want the deepest LONG ladder %s", keeper.Symbol, reserveGridShape[1].symbol)
	}

	own := reserveGridTrade(12, reserveGridShape[3].symbol, reserveGridShape[3].depth)
	_, ownCost := ladder.NextEntryCost(own, keeper.RemainingCost)

	held, err := ShouldHold(priorityEvent(own, "buy", view, keeper.RemainingCost+ownCost-1))
	if err == nil {
		t.Fatalf("%s must wait for the deepest long ladder", own.Symbol)
	}
	row := held.Trade.Logs[0].Message
	if !strings.Contains(row, fmt.Sprintf("%s at depth %d of %d", keeper.Symbol, keeper.Depth, reserveGridDepths)) {
		t.Fatalf("row = %q, want the deepest long ladder named", row)
	}
	if strings.Contains(row, deepInverse.Symbol+" at depth") {
		t.Fatalf("row = %q, want no ladder spending another asset to keep this wallet", row)
	}

	// And the quote balance that covers the long reservation releases the
	// entry, however much base the inverse ladder still needs.
	released, err := ShouldHold(priorityEvent(own, "buy", view, keeper.RemainingCost+ownCost))
	if err != nil {
		t.Fatalf("a quote wallet that covers the long reservation must release the entry, got %v", err)
	}
	if len(released.Trade.Logs) != 0 {
		t.Fatalf("no row may be written on a released ladder, got %v", messages(released.Trade.Logs))
	}
}
