package smarttakeloss

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/strategies"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// Two ladders of real fills, each met by the real bars of its market at the first tick the
// reading holds on it, replayed through the gate's entry points (Apply, ExitReached,
// ExitRow, Armed, EntryHold) with the engines' own writer for the rows and the state the
// chain leaves (engineStep). The trend the dashboard reads is served as met. The shared
// action is asked on every tick the reading stands and decides by the ladder's state, two
// ticks for a sale: the first activates the stopLoss from buy (the triggered row and the
// activation row, nothing sold), the next sells it from stopLoss, whatever the market reads
// by then. It reads two bounds, the depth bound first: the shallow ETH ladder is within it
// and has its stopLoss activated at the tape's print, within the cap, while at a print of
// the ladder's own over the cap the action waits; the deep LINK ladder is past it, and the
// action waits for its depth at the tape's print, within the cap, and at the print its next
// depth arms at, over it, so that the same ladder at its first fills, within the bound, is
// the one the cap holds. Nothing holds the next depth: the ladder buys it as its own logic
// lets it, and the fill resets the episode. They run at the numbers they were measured at
// (replayFlatMarket).

// replayArmings are the proposals that arm or re-arm a depth, as the engines hand them to
// Apply: the stopLoss of a ladder in buy that has gone against its next depth level and the
// update_stopLoss of one in stopLoss that keeps going. The action replaces them like any
// other proposal of the add side.
var replayArmings = []string{"stopLoss", "update_stopLoss"}

// engineBuyRow is the `buy` row of the engines' logic, as hermes and sisyphus carry it and as
// the strategies package's own tests do: a ladder in buy proposes stopLoss, the arming of its
// next depth, once its move against the newest fill, measured against the new price, is the
// pair's percentage plus its tolerance down; a resting add the print ran away from is cancelled
// first (update_buy), a branch these tests leave off (Position.RestingOrder false).
const engineBuyRow = "(restingOrder && percentage > tradePercentage+tolerance) ? 'update_buy' : (percentage <= -tradePercentage-tolerance ? 'stopLoss' : (profitPercentage >= tradePercentage+tolerance ? 'takeProfit' : ''))"

// activatedMessage and resetMessage are the texts of the rows the gate writes when the
// action activates the stopLoss (the loss it would take as the signed profit beside the
// trigger, with two decimals) and when a new depth resets the episode.
func activatedMessage(lossPct float64) string {
	return fmt.Sprintf("smartTakeLoss: flat market stopLoss activated, profit %.2f%% of the used amount at or over %.2f%%", 0-lossPct, -float64(replayPercentage))
}

const resetMessage = "smartTakeLoss: flat market reset at the new depth"

// lossOverTheCap fails unless a sale of the ladder at the print loses more than the cap, as
// the gate reads it and as the fills say, and answers that loss as a share of the used amount.
func lossOverTheCap(t *testing.T, trade aggragates.Trades, price float64) float64 {
	t.Helper()
	profitPct, standing := saleProfit(trade, price, -replayPercentage)
	lossPct := -profitPct
	if hand := handLossPct(trade, price); standing != short || math.Abs(lossPct-hand) > 1e-6 {
		t.Fatalf("fixture drifted: the print loses %v of the used amount over the cap, the gate reads %v (%v)", hand, lossPct, standing)
	}
	return lossPct
}

// nextDepthArming is the print and the proposal at which the engines' buy row arms the
// ladder's next depth, and what a sale there loses as a share of the used amount: at least
// 100*step/(100+step), more as the ladder's average entry stands over the newest fill. It
// fails the test when the row does not arm there and not a cent over it.
func nextDepthArming(t *testing.T, trade aggragates.Trades) (arming float64, proposal string, lossPct float64) {
	t.Helper()
	settings, newest := trade.StrategyPair.StrategySettings[0], trade.PositionPrice
	engine := strategies.Strategy{
		Position: strategies.Position{Type: "buy", Price: newest, AverageEntryPrice: ladder.AverageEntryPrice(trade)},
		Logic:    map[string]string{"buy": engineBuyRow},
		Settings: trade.StrategyPair.StrategySettings,
	}
	proposalAt := func(price float64) string {
		return engine.GetPosition(engine.GetPercentage(price), engine.GetProfitPercentage(price))
	}
	step := settings.Percentage + settings.Tolerance
	arming = helpers.ToFixed(newest/(1+step/100), int(trade.StrategyPair.TradeFilters.PriceFilter))
	if proposalAt(arming) != "stopLoss" || proposalAt(arming+0.01) == "stopLoss" {
		t.Fatalf("fixture drifted: the buy row must arm the next depth at %v and not a cent over it", arming)
	}
	lossPct = handLossPct(trade, arming)
	if floor := 100 * step / (100 + step); lossPct < floor {
		t.Fatalf("fixture drifted: at its arming print %v the ladder loses %v of the used amount, want at least %v", arming, lossPct, floor)
	}
	return arming, proposalAt(arming), lossPct
}

// assertActivatesThenSells fails unless the gate acts on the ladder at the print, which is
// within both bounds and books a real loss within the cap, in its two steps. On the first
// tick, whatever the ladder proposes (none, an arming, a re-arming), the episode opens and
// the stopLoss is activated: the triggered row and the activation row, both at the newest
// fill, nothing sold, ExitReached false, a take profit proposed in buy giving way to it. The chain
// leaves the ladder in stopLoss, the fold reads the activated episode, and from the second
// tick the committed ladder TRAILS from the print the activation anchored it at, whatever it
// proposes and whatever the market reads, with no row of its own: it holds at that print and
// is sold for the gate at a fall under the tolerance; the sold row, filed with the pair's
// decimals, closes the episode in the fold. A ladder that armed itself in stopLoss is sold at
// once, with the triggered row alone.
func assertActivatesThenSells(t *testing.T, trade aggragates.Trades, ai aggragates.AIIndicators, price float64, now time.Time, reasons []string) {
	t.Helper()
	newest := trade.PositionPrice
	profitPct, standing := saleProfit(trade, price, -replayPercentage)
	lossPct := -profitPct
	if hand := handLossPct(trade, price); standing != reached || lossPct <= 0 || math.Abs(lossPct-hand) > 1e-6 {
		t.Fatalf("fixture drifted: the print loses %v of the used amount within the cap, the gate reads %v (%v)", hand, lossPct, standing)
	}
	if ExitReached(trade, price, ai.SmartTakeLoss) {
		t.Error("ExitReached reads the sale alone: the activation the empty proposal finds on a ladder in buy is no exit")
	}
	if got := Apply(trade, "takeProfit", price, ai); got.Position != "stopLoss" || !got.Activated || got.Reason != "" || len(got.Rows) != 2 {
		t.Errorf("a take profit proposed in buy gives way to the activation, got %+v", got)
	}

	var activated aggragates.Trades
	for _, position := range append([]string{""}, replayArmings...) {
		next, got := engineStep(trade, position, price, now, ai)
		if got.Position != "stopLoss" || !got.Activated || got.Reason != "" || len(got.Rows) != 2 {
			t.Fatalf("%q: want the stopLoss activated with the triggered row and the activation row, got %+v", position, got)
		}
		assertEpisodeRow(t, got.Rows[0], EventTriggered, newest, triggeredMessage(reasons))
		if !reflect.DeepEqual(got.Rows[0].Reasons, reasons) {
			t.Errorf("reasons = %q, want %q", got.Rows[0].Reasons, reasons)
		}
		assertEpisodeRow(t, got.Rows[1], EventActivated, newest, activatedMessage(lossPct))
		activated = next
	}
	if activated.PositionType != "stopLoss" {
		t.Fatalf("the activation chain leaves the ladder in stopLoss, got %q", activated.PositionType)
	}
	if st := rebuildState(activated); !st.flatMarketWatched || !st.flatMarketActive || !st.flatMarketActivated || st.flatMarketSaleHeld || st.flatMarketFrom != newest {
		t.Errorf("the episode is open with its stopLoss activated, got %+v", st)
	}

	gone := withBlock(aggragates.SmartTakeLossIndicators{})
	falls := priceAtMove(activated, price, -(trailRow(activated).Tolerance + 1))
	for _, position := range append(slices.Clone(flatMarketOrdinaryProposals), replayArmings...) {
		for _, reading := range []aggragates.AIIndicators{ai, gone} {
			assertTrailHeld(t, Apply(activated, position, price, reading))
			assertSold(t, Apply(activated, position, falls, reading), reasonFlatMarket)
		}
	}
	if ExitReached(activated, price, ai.SmartTakeLoss) || !ExitReached(activated, falls, ai.SmartTakeLoss) {
		t.Error("ExitReached must read the sale of a ladder the gate activated at a fall under its tolerance, and not at the print it was anchored at")
	}
	sold := Apply(restingIn(trade, "stopLoss"), "stopLoss", price, ai)
	assertSoldWith(t, sold, reasonFlatMarket, triggeredBy(trade, ai))

	exit := ExitRow(activated, reasonFlatMarket, price)
	assertEpisodeRow(t, exit, EventSold, price, "smartTakeLoss: sell at flat market "+strconv.FormatFloat(price, 'f', int(trade.StrategyPair.TradeFilters.PriceFilter), 64))
	closed := withRow(activated, exit, now)
	if st := rebuildState(closed); st.flatMarketActive || st.flatMarketSaleHeld || st.flatMarketActivated {
		t.Errorf("the sold row closes the episode, got %+v", st)
	}
	if want := []string{eventAt(EventTriggered, newest), eventAt(EventActivated, newest), eventAt(EventSold, price)}; !slices.Equal(episodeEvents(t, closed), want) {
		t.Errorf("events = %q, want %q", episodeEvents(t, closed), want)
	}
}

// assertWaitsOverTheBound fails unless the gate waits at the print, for the reason the held
// row's message words (heldMessage: the loss over the cap or the depth past the bound,
// whichever the caller has checked the print and the ladder to hold it for). On any proposal
// it opens the episode and writes the held row beside the triggered row, both at the newest
// fill, activates and sells nothing and hands the proposal back as it came, the arming of the
// next depth and the force-trailing re-anchor included, the ladder resting in buy or in
// stopLoss; the same print again writes nothing. ExitReached reads false. It answers the
// trade carrying the two rows and their events.
func assertWaitsOverTheBound(t *testing.T, trade aggragates.Trades, ai aggragates.AIIndicators, price float64, now time.Time, reasons []string, heldMessage string) aggragates.Trades {
	t.Helper()
	newest := trade.PositionPrice
	if ExitReached(trade, price, ai.SmartTakeLoss) {
		t.Error("ExitReached must not reach a sale the bounds hold")
	}

	for _, state := range []string{"buy", "stopLoss"} {
		for _, position := range addSideProposals {
			_, got := engineStep(restingIn(trade, state), position, price, now, ai)
			assertNoSale(t, got, position)
			if len(got.Rows) != 2 {
				t.Fatalf("%s, %q: rows = %+v, want the triggered row and the held row", state, position, got.Rows)
			}
			assertEpisodeRow(t, got.Rows[0], EventTriggered, newest, triggeredMessage(reasons))
			assertEpisodeRow(t, got.Rows[1], EventSaleHeld, newest, heldMessage)
		}
	}

	held, got := engineStep(trade, "stopLoss", price, now, ai)
	if want := []string{eventAt(EventTriggered, newest), eventAt(EventSaleHeld, newest)}; !slices.Equal(episodeEvents(t, held), want) {
		t.Errorf("events = %q, want %q", episodeEvents(t, held), want)
	}
	if st := rebuildState(held); !st.flatMarketActive || !st.flatMarketSaleHeld || st.flatMarketActivated {
		t.Errorf("the episode is open with its action held, got %+v", st)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("rows = %+v, want the triggered row and the held row", got.Rows)
	}
	for _, position := range addSideProposals {
		_, again := engineStep(held, position, price, now.Add(time.Minute), ai)
		assertUntouched(t, again, position)
	}
	return held
}

// The ETH ladder of two fills on the bars of its market, at the tick after the first bar
// that reads a flat market market for it. At the close of that bar the ladder loses
// little, within the cap, and the gate acts on it in two steps: it opens its episode naming
// the quiet bodies it counted and activates the stopLoss, on whatever the ladder proposes,
// the ladder being shallow, well within the depth bound, and trails it from there to its sale. At a
// print twice the cap under its average entry the action waits with the held row naming the
// loss, and a later tick back within the cap activates the stopLoss with the activation row
// alone. The gate arms the ladder from its first fill, holds the first fill of a fresh parent
// on the same bars, and lets the first fill the ladder really opened on go through on the
// bars before it.
func TestFlatMarketAcceptanceEthereumJune2022ActivatesTheStopLossThenSellsWithinTheCapAndWaitsOverIt(t *testing.T) {
	replayFlatMarket(t)
	trade := ethereumLadder(ethereumOrders)
	window := func(lastBar string) aggragates.SmartTakeLossIndicators {
		return tapeWindowBlock(t, "ethusdt-1h-2022-06.json", utcAt(lastBar), flatmarket.MinSmcTrend, 0, dashboardRows)
	}
	block := window("2022-06-09 03:00:00")
	ai, reasons := withBlock(block), holdingReasons(t, block, trade.StrategyPair.StrategySettings[0].Percentage)
	price, now := block.FlatMarketCloses[sophosLookbackBars-1], utcAt("2022-06-09 04:00:02")
	if price != 1799.31 || reasons[0] != "1h bars 2022-06-08 04:00 to 2022-06-09 03:00 UTC" {
		t.Fatalf("fixture drifted: the tape's window is %q and closes at %v", reasons[0], price)
	}
	if filled := ladder.CountFilledEntries(trade); filled > lastDepthFills-replayIgnoreLastDepths {
		t.Fatalf("fixture drifted: the ETH ladder, %d deep, must be within the depth bound", filled)
	}

	assertActivatesThenSells(t, trade, ai, price, now, reasons)
	if !Armed(trade) {
		t.Error("the gate arms a spot ladder from its first fill")
	}
	withFlatMarket(t, false)
	if Armed(trade) {
		t.Error("the gate switched off arms nothing, the other rules being off")
	}
	withFlatMarket(t, true)

	under := helpers.ToFixed(ladder.AverageEntryPrice(trade)*(1-2*float64(replayPercentage)/100), int(trade.StrategyPair.TradeFilters.PriceFilter))
	held := assertWaitsOverTheBound(t, trade, ai, under, now, reasons, saleHeldMessage(lossOverTheCap(t, trade, under)))
	profitPct, _ := saleProfit(trade, price, -replayPercentage)
	lossPct := -profitPct
	for _, position := range append(slices.Clone(flatMarketOrdinaryProposals), replayArmings...) {
		bounced := Apply(held, position, price, ai)
		if bounced.Position != "stopLoss" || !bounced.Activated || bounced.Reason != "" || len(bounced.Rows) != 1 {
			t.Fatalf("%q: want the stopLoss activated with the activation row alone, got %+v", position, bounced)
		}
		assertEpisodeRow(t, bounced.Rows[0], EventActivated, trade.PositionPrice, activatedMessage(lossPct))
	}
	assertSold(t, Apply(restingIn(held, "stopLoss"), "stopLoss", price, ai), reasonFlatMarket)

	fresh := ethereumLadder(nil)
	want := gates.Hold{Reason: FlatMarketEntryHoldReason, Param: aggragates.StrategyParamSmartTakeLoss, Gate: GateFlatMarket, Data: EventData{Event: gates.EventHeld, Reasons: reasons}}
	if got := EntryHold(fresh, aggragates.SideLong, ai); !reflect.DeepEqual(got, want) {
		t.Errorf("a fresh parent's first fill: hold = %+v, want %+v", got, want)
	}
	if got := EntryHold(fresh, aggragates.SideLong, withBlock(window("2022-06-07 19:00:00"))); got.Held() {
		t.Errorf("the bars before the ladder's first fill read too few quiet bodies, got %+v", got)
	}
}

// The LINK ladder of six fills on the bars of its market, at the tick after the first bar
// that reads a flat market market for it. At the close of that bar the ladder loses little,
// within the cap, and yet the gate acts on it on no tick: the ladder has filled one depth
// more than the deepest depth the bound lets the action be made at, and the bound is read
// before the cap. The first tick writes the triggered row and the held row, which names the
// depth, whichever state the ladder rests in, and nothing else is held: every proposal
// comes back as it came, the arming of the next depth and the force-trailing re-anchor
// included, and so do the take profits. It is the ETH ladder's test turned round: the same
// print on a shallow ladder has its stopLoss activated.
func TestFlatMarketAcceptanceChainlinkMay2022WaitsAtItsDeepFillOnTheRealPrintByDepth(t *testing.T) {
	replayFlatMarket(t)
	trade := chainlinkLadder(chainlinkOrders)
	block := tapeWindowBlock(t, "linkusdt-1h-2022-05.json", utcAt("2022-05-06 15:00:00"), flatmarket.MinSmcTrend, 0, dashboardRows)
	ai := withBlock(block)
	reasons, price := holdingReasons(t, block, trade.StrategyPair.StrategySettings[0].Percentage), block.FlatMarketCloses[sophosLookbackBars-1]
	if price != 10.7 || reasons[0] != "1h bars 2022-05-05 16:00 to 2022-05-06 15:00 UTC" {
		t.Fatalf("fixture drifted: the tape's window is %q and closes at %v", reasons[0], price)
	}
	if profitPct, standing := saleProfit(trade, price, -replayPercentage); standing != reached || profitPct >= 0 {
		t.Fatalf("fixture drifted: the print loses %v of the used amount, which the trigger must take (%v)", -profitPct, standing)
	}

	held := assertWaitsOverTheBound(t, trade, ai, price, utcAt("2022-05-06 16:00:02"), reasons, saleHeldAtDepthMessage(len(chainlinkOrders), lastDepthFills))
	for _, state := range []string{"buy", "stopLoss"} {
		for _, position := range append(slices.Clone(addSideProposals), "takeProfit", "forceTrailingTakeProfit") {
			assertUntouched(t, Apply(restingIn(held, state), position, price, ai), position)
		}
	}
}

// The LINK ladder at the deepest depth the bound lets the action be made at, its first fills,
// at a print the ladder loses over the cap at: the gate opens its episode and waits for its
// loss, on any proposal and in either state, and nothing holds the arming of the next depth,
// the force-trailing re-anchor of it included. With the trend a row short nothing is written
// on any proposal. The first closed bar that no longer holds changes nothing in the episode:
// it STANDS, with no row and no second held row, whatever the proposal and whatever the
// ladder's print, and the action goes ahead inside it all the same, activating the stopLoss
// with the activation row alone on the first print within the cap, the market still gone.
func TestFlatMarketAcceptanceChainlinkMay2022WaitsOverTheCapAtItsShallowFillsAndTheEpisodeStandsThroughItsReading(t *testing.T) {
	replayFlatMarket(t)
	orders := chainlinkShallow()
	trade, newest := chainlinkLadder(orders), orders[len(orders)-1].price
	overTheCap := helpers.ToFixed(ladder.AverageEntryPrice(trade)*(1-2*float64(replayPercentage)/100), int(trade.StrategyPair.TradeFilters.PriceFilter))
	window := func(lastBar string, down int) aggragates.AIIndicators {
		return withBlock(tapeWindowBlock(t, "linkusdt-1h-2022-05.json", utcAt(lastBar), down, 0, dashboardRows))
	}
	ai := window("2022-05-06 15:00:00", flatmarket.MinSmcTrend)
	reasons := holdingReasons(t, ai.SmartTakeLoss, trade.StrategyPair.StrategySettings[0].Percentage)
	held := assertWaitsOverTheBound(t, trade, ai, overTheCap, utcAt("2022-05-06 16:00:02"), reasons, saleHeldMessage(lossOverTheCap(t, trade, overTheCap)))

	short := window("2022-05-06 15:00:00", flatmarket.MinSmcTrend-1)
	for _, position := range []string{"", "stopLoss"} {
		assertUntouched(t, Apply(trade, position, overTheCap, short), position)
	}

	later := window("2022-05-06 17:00:00", flatmarket.MinSmcTrend)
	closed := later.SmartTakeLoss.FlatMarketCloses[sophosLookbackBars-1]
	if reading := flatMarketReading(trade, later.SmartTakeLoss, later); reading.Holds {
		t.Fatalf("fixture drifted: the first closed bar after the reading must no longer hold, got %+v", reading)
	}
	lossOverTheCap(t, trade, closed)
	for _, position := range []string{"", "stopLoss", "update_stopLoss", "buy"} {
		assertUntouched(t, Apply(held, position, closed, later), position)
	}
	stood, _ := engineStep(held, "", closed, utcAt("2022-05-06 18:00:02"), later)
	for _, position := range []string{"", "stopLoss"} {
		_, again := engineStep(stood, position, overTheCap, utcAt("2022-05-06 19:00:02"), later)
		assertUntouched(t, again, position)
	}
	if st := rebuildState(stood); !st.flatMarketActive || !st.flatMarketSaleHeld || st.flatMarketActivated || st.flatMarketFrom != newest {
		t.Errorf("the episode stands as it was, got %+v", st)
	}

	within := helpers.ToFixed(ladder.AverageEntryPrice(trade)*(1-float64(replayPercentage)/200), int(trade.StrategyPair.TradeFilters.PriceFilter))
	profitPct, standing := saleProfit(trade, within, -replayPercentage)
	lossPct := -profitPct
	if standing != reached || lossPct <= 0 {
		t.Fatalf("fixture drifted: the print %v loses %v of the used amount, which must be a real loss within the cap (%v)", within, lossPct, standing)
	}
	activated, got := engineStep(stood, "", within, utcAt("2022-05-06 20:00:02"), later)
	assertActivated(t, got, FlatMarketStopLossActivatedRow(newest, -lossPct))
	if want := []string{eventAt(EventTriggered, newest), eventAt(EventSaleHeld, newest), eventAt(EventActivated, newest)}; !slices.Equal(episodeEvents(t, activated), want) {
		t.Errorf("events = %q, want %q", episodeEvents(t, activated), want)
	}
}

// The consequence of the rule, on the LINK ladder at its first fills, the deepest depth the
// bound lets the action be made at, met where its next depth arms. The engines' buy row
// proposes that arming once the price is a full step under the newest fill, the pair's
// percentage plus its tolerance, measured against the new price, and there the ladder loses
// at least 100*step/(100+step) percent of what it used, more as its average entry stands
// over the newest fill: over the cap as soon as the step is more than the cap and a little.
// So the action never goes ahead at a natural arming of a ladder like this one: the tick
// writes the triggered row and the held row, naming the loss, once, and the arming passes
// through, nothing holding it. The ladder buys its next depth, the new fill RESETS the
// episode with the reset row at the new fill, the reading is read afresh, and the new
// episode, the ladder now past the depth bound, waits for its depth with a held row of its
// own.
func TestFlatMarketAcceptanceChainlinkMay2022TheNextDepthArmsOverTheCapAndTheFillResetsTheEpisode(t *testing.T) {
	replayFlatMarket(t)
	shallow := chainlinkShallow()
	trade := chainlinkLadder(shallow)
	newest := trade.PositionPrice
	arming, proposal, lossPct := nextDepthArming(t, trade)
	if lossPct <= float64(replayPercentage) {
		t.Fatalf("fixture drifted: at its arming print %v the ladder loses %v of the used amount, want over the cap %v", arming, lossPct, float64(replayPercentage))
	}

	ai := withBlock(tapeWindowBlock(t, "linkusdt-1h-2022-05.json", utcAt("2022-05-06 15:00:00"), flatmarket.MinSmcTrend, 0, dashboardRows))
	reasons := holdingReasons(t, ai.SmartTakeLoss, trade.StrategyPair.StrategySettings[0].Percentage)
	held, got := engineStep(trade, proposal, arming, utcAt("2022-05-06 16:40:00"), ai)
	assertNoSale(t, got, "stopLoss")
	if len(got.Rows) != 2 {
		t.Fatalf("rows = %+v, want the triggered row and the held row on the arming tick", got.Rows)
	}
	assertEpisodeRow(t, got.Rows[0], EventTriggered, newest, triggeredMessage(reasons))
	assertEpisodeRow(t, got.Rows[1], EventSaleHeld, newest, saleHeldMessage(lossPct))
	for _, position := range []string{"stopLoss", "update_stopLoss", ""} {
		assertUntouched(t, Apply(held, position, arming, ai), position)
	}

	filledAt := chainlinkOrders[len(shallow)]
	deeper := withFill(held, filledAt.price, filledAt.at)
	got = Apply(deeper, "", arming, ai)
	assertNoSale(t, got, "")
	if len(got.Rows) != 3 {
		t.Fatalf("rows = %+v, want the reset row, the triggered row and the held row on the tick after the new fill", got.Rows)
	}
	assertEpisodeRow(t, got.Rows[0], EventReset, filledAt.price, resetMessage)
	assertEpisodeRow(t, got.Rows[1], EventTriggered, filledAt.price, triggeredMessage(reasons))
	assertEpisodeRow(t, got.Rows[2], EventSaleHeld, filledAt.price, saleHeldAtDepthMessage(len(shallow)+1, lastDepthFills))
	written, _ := engineStep(deeper, "", arming, utcAt("2022-05-06 16:50:00"), ai)
	for _, position := range []string{"", "stopLoss"} {
		assertUntouched(t, Apply(written, position, arming, ai), position)
	}
}

// The same consequence on the LINK ladder at all its fills, past the depth bound: where its
// next depth arms the action waits again, and the held row names the depth and not the loss,
// though the loss there is over the cap as well, since the bound is read first and the first
// that fails names the row. The arming passes through, nothing holding it, and the same
// print on any other proposal writes nothing more.
func TestFlatMarketAcceptanceChainlinkMay2022TheNextDepthArmsPastTheDepthBound(t *testing.T) {
	replayFlatMarket(t)
	trade := chainlinkLadder(chainlinkOrders)
	newest := trade.PositionPrice
	arming, proposal, lossPct := nextDepthArming(t, trade)
	if lossPct <= float64(replayPercentage) {
		t.Fatalf("fixture drifted: at its arming print %v the ladder loses %v of the used amount, want over the cap %v", arming, lossPct, float64(replayPercentage))
	}

	ai := withBlock(tapeWindowBlock(t, "linkusdt-1h-2022-05.json", utcAt("2022-05-06 15:00:00"), flatmarket.MinSmcTrend, 0, dashboardRows))
	reasons := holdingReasons(t, ai.SmartTakeLoss, trade.StrategyPair.StrategySettings[0].Percentage)
	held, got := engineStep(trade, proposal, arming, utcAt("2022-05-06 16:40:00"), ai)
	assertNoSale(t, got, "stopLoss")
	if len(got.Rows) != 2 {
		t.Fatalf("rows = %+v, want the triggered row and the held row on the arming tick", got.Rows)
	}
	assertEpisodeRow(t, got.Rows[0], EventTriggered, newest, triggeredMessage(reasons))
	assertEpisodeRow(t, got.Rows[1], EventSaleHeld, newest, saleHeldAtDepthMessage(len(chainlinkOrders), lastDepthFills))
	for _, position := range []string{"stopLoss", ""} {
		assertUntouched(t, Apply(held, position, arming, ai), position)
	}
}
