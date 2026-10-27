package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/lineardecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The fixture is a ladder on one line, watched by the early take profit alone,
// and its line holds under a trend that just meets the need. The arm depth follows
// the fewest fills the rule reads, so the gate never asks for a ladder the rule
// cannot read.
func TestTheLinearDeclineFixtureSitsOnOneLineAndIsWatched(t *testing.T) {
	onlyTheLinearDecline(t)
	trade, block := linearScene(linearTrendMet)
	st := rebuildState(trade)
	if len(st.fills) != linearDepth || !st.earlyTakeProfitWatched || st.earlyTakeProfit || st.slowDeclineWatched || st.depthPriorityHeld {
		t.Fatalf("fixture drifted: a ladder watched by the early take profit alone, not latched, not held, got %+v", st)
	}
	linearReading(t, trade, block)
	if !Armed(trade) {
		t.Fatal("fixture drifted: the ladder must be armed through the indecision watch")
	}
	arm, least := lineardecline.ArmDepth, lineardecline.MinFills
	if arm != least {
		t.Fatalf("the arm depth %d must follow the fewest fills the rule reads, %d", arm, least)
	}
}

// A trend short of the need, one that was not read and a count with no row behind it
// leave a line that holds unlatched.
func TestTheLinearDeclineReadsNothingUnderATrendShortOfTheNeed(t *testing.T) {
	onlyTheLinearDecline(t)
	needsTheLinearTrend(t)
	trade, block := linearScene(linearTrendMet)
	assertRow(t, Apply(trade, "", trade.PositionPrice, withBlock(block)), linearRow(t, trade, block))
	for name, trend := range map[string][2]int{
		"a row short of the need":  {linearTrendShort, linearTrendValid},
		"no row read":              {0, 0},
		"a count with no row read": {linearTrendMet, 0},
	} {
		served := withSmcTrend(block, trend[0], trend[1])
		if got := Apply(trade, "", trade.PositionPrice, withBlock(served)); got.Position != "" || got.Reason != "" || len(got.Rows) != 0 {
			t.Errorf("%s: want nothing read, got %+v", name, got)
		}
	}
}

// The ladder at the arm depth — the fewest fills the rule reads — is read, and one
// fill short of it is not, whatever line its fills sit on. At linearDepth — the arm
// depth, or the depth the early take profit watches from when that is deeper —
// the gate latches the ladder, and it latches nothing for one fill fewer.
func TestTheLinearDeclineReadsFromTheArmDepth(t *testing.T) {
	onlyTheLinearDecline(t)
	armed := max(lineardecline.ArmDepth, 3)
	slope := linearSlope
	trade, block := linearLadder(slope, armed), linearBlock(slope, armed, linearTail, linearTrendMet)
	if _, reads := linearDeclineReads(rebuildState(trade), block, tick{}); !reads {
		t.Fatalf("a ladder at the arm depth, %d fills, is read", armed)
	}
	prefix := linearLadder(slope, armed-1)
	if _, reads := linearDeclineReads(rebuildState(prefix), block, tick{}); reads {
		t.Fatalf("a ladder one fill short of the arm depth, %d fills, is not read", armed-1)
	}

	shallow := linearDepth - 1
	shallowSlope := linearSlope
	short, shortBlock := linearLadder(shallowSlope, shallow), linearBlock(shallowSlope, shallow, linearTail, linearTrendMet)
	assertUntouched(t, Apply(short, "", short.PositionPrice, withBlock(shortBlock)), "")
	deep, deepBlock := linearScene(linearTrendMet)
	assertRow(t, Apply(deep, "", deep.PositionPrice, withBlock(deepBlock)), linearRow(t, deep, deepBlock))
}

// The line is read for a ladder the rule reads and nothing else: not with the latch on
// the line switched off, nor with the early take profit off, which watches no
// ladder then; not for a child, a strategy without the flag, an inverse ladder or a
// futures one, nor for a newest fill with no stamp, a series that is not served or
// one that does not hold the newest fill's bar; and not for a ladder whose closes
// leave the line or whose fall is short of the least. Each is the control's ladder
// with one thing changed, and the control latches.
func TestTheLinearDeclineReadsNothingForALadderItDoesNotRead(t *testing.T) {
	onlyTheLinearDecline(t)
	slope := linearSlope
	control, controlBlock := linearScene(linearTrendMet)
	assertRow(t, Apply(control, "", control.PositionPrice, withBlock(controlBlock)), linearRow(t, control, controlBlock))

	type scene struct {
		trade aggragates.Trades
		block aggragates.SmartTakeLossIndicators
	}
	fresh := func() scene {
		trade, block := linearScene(linearTrendMet)
		return scene{trade, block}
	}
	scenes := map[string]func(*scene){
		"a futures ladder": func(s *scene) {
			s.trade.Strategy.TradeType = aggragates.Futures
		},
		"an inverse ladder": func(s *scene) {
			s.trade = testutil.LadderTrade(true, linearFills(slope, linearDepth)...)
		},
		"a child": func(s *scene) {
			s.trade.ParentID = 7
		},
		"a strategy without the flag": func(s *scene) {
			s.trade.Strategy.Params.SmartTakeLoss = false
		},
		"a newest fill with no stamp": func(s *scene) {
			s.trade.History[len(s.trade.History)-1].CreatedAt = time.Time{}
		},
		"no series": func(s *scene) {
			s.block.SeriesOpens, s.block.SeriesCloses = nil, nil
		},
		"arrays of different lengths": func(s *scene) {
			s.block.SeriesCloses = s.block.SeriesCloses[1:]
		},
		"a series that stops a bar short of the newest fill's": func(s *scene) {
			s.block = linearBlock(slope, linearDepth-1, linearGap-1, linearTrendMet)
		},
		"a close off the line": func(s *scene) {
			s.block.SeriesCloses[linearLead+linearGap/2] *= 1 + linearOver/100
		},
	}
	if lineardecline.MinFallPct < 0 {
		scenes["a fall short of the least"] = func(s *scene) {
			shortOf := -0.5 * lineardecline.MinFallPct
			flat := -shortOf / float64((lineardecline.Window-1)*linearGap)
			s.trade, s.block = linearLadder(flat, linearDepth), linearBlock(flat, linearDepth, linearTail, linearTrendMet)
		}
	}
	for name, change := range scenes {
		one := fresh()
		change(&one)
		if got := Apply(one.trade, "", control.PositionPrice, withBlock(one.block)); got.Position != "" || got.Reason != "" || len(got.Rows) != 0 {
			t.Errorf("%s: want nothing read, got %+v", name, got)
		}
	}

	withEarlyTakeProfitLinearDecline(t, false)
	assertUntouched(t, Apply(control, "", control.PositionPrice, withBlock(controlBlock)), "")
	withEarlyTakeProfitLinearDecline(t, true)
	withEarlyTakeProfit(t, false)
	assertUntouched(t, Apply(control, "", control.PositionPrice, withBlock(controlBlock)), "")
}

// The part of the watch the fills do not decide is the linear decline's own: a futures ladder
// and an inverse one are never eligible nor read for the latch, the control is both; switched
// off, no ladder is eligible and the latch still reads the control.
func TestTheLinearDeclineEligibilityIsItsOwn(t *testing.T) {
	onlyTheLinearDecline(t)
	withLinearDecline(t, true)
	slope := linearSlope
	control, block := linearScene(linearTrendMet)
	futures := control
	futures.Strategy.TradeType = aggragates.Futures
	inverse := testutil.LadderTrade(true, linearFills(slope, linearDepth)...)
	if _, reads := linearDeclineReads(rebuildState(control), block, tick{}); !reads || !linearDeclineEligible(control) {
		t.Fatal("control: the ladder on one line is eligible and read")
	}
	for name, trade := range map[string]aggragates.Trades{"a futures ladder": futures, "an inverse ladder": inverse} {
		if linearDeclineEligible(trade) {
			t.Errorf("%s is eligible", name)
		}
		if _, reads := linearDeclineReads(rebuildState(trade), block, tick{}); reads {
			t.Errorf("%s is read", name)
		}
	}
	withLinearDecline(t, false)
	if linearDeclineEligible(control) {
		t.Error("switched off, no ladder is eligible")
	}
	if _, reads := linearDeclineReads(rebuildState(control), block, tick{}); !reads {
		t.Error("switched off, the latch still reads the control")
	}
}
