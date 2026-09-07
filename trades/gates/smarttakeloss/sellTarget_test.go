package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// anchor is a trend-line anchor on the bar that opened at clock.
func anchor(clock string, price float64) aggragates.TrendLineAnchor {
	return aggragates.TrendLineAnchor{At: testutil.At(clock).UnixMilli(), Price: price}
}

// The line SOL 45211 had at its activation: H1 193.25 at 06:30, H2 192.97 at
// 11:45, projected ~192.65 at 17:45 — over the last fill, so it is honoured.
func TestSellTargetReachedAtTheLine(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...) // last fill 179.78
	st := rebuildState(trade)
	block := solBlock()
	block.Resistance = aggragates.TrendLine{From: anchor("06:30:00", 193.25), To: anchor("11:45:00", 192.97)}
	now := testutil.At("17:45:00").UnixMilli()
	level, _ := projectLine(block.Resistance, now)
	if level <= trade.PositionPrice || level >= block.UpperBB {
		t.Fatalf("the fixture wants the line between the last fill and the band, got %v", level)
	}

	if reason, hit := sellTargetReached(trade, st, level, now, block); !hit || reason != "resistance line" {
		t.Fatalf("touching the line sells, got %q %v", reason, hit)
	}
	if reason, hit := sellTargetReached(trade, st, level+1, now, block); !hit || reason != "resistance line" {
		t.Fatalf("a print through the line sells, got %q %v", reason, hit)
	}
	if _, hit := sellTargetReached(trade, st, level-0.01, now, block); hit {
		t.Fatal("under the line and under the band nothing sells")
	}
}

func TestSellTargetReachedAtTheUpperBand(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	st := rebuildState(trade)
	block := solBlock() // no line: the band is the only target
	now := testutil.At("17:45:00").UnixMilli()

	if reason, hit := sellTargetReached(trade, st, block.UpperBB, now, block); !hit || reason != "upper bollinger band" {
		t.Fatalf("touching the band sells, got %q %v", reason, hit)
	}
	if reason, hit := sellTargetReached(trade, st, block.UpperBB+2, now, block); !hit || reason != "upper bollinger band" {
		t.Fatalf("a print through the band sells, got %q %v", reason, hit)
	}
	if _, hit := sellTargetReached(trade, st, block.UpperBB-0.01, now, block); hit {
		t.Fatal("under the band nothing sells")
	}
}

// The line re-anchored on SOL 45211 after the activation (192.97 at 11:45 →
// 183.99 at 20:15) projects to ~178.97 at 01:00 the next day, under the last
// fill of 179.78: without the guard the 179.41 bounce sells at the bottom,
// before the one permitted depth. With it the band stands alone.
func TestSellTargetIgnoresALineUnderTheLastFill(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	st := rebuildState(trade)
	block := solBlock()
	block.Resistance = aggragates.TrendLine{From: anchor("11:45:00", 192.97), To: anchor("20:15:00", 183.99)}
	bounce := testutil.At("20:15:00").Add(4*time.Hour + 45*time.Minute).UnixMilli()
	level, _ := projectLine(block.Resistance, bounce)
	if level >= trade.PositionPrice {
		t.Fatalf("the fixture wants the line under the last fill, got %v", level)
	}

	if reason, hit := sellTargetReached(trade, st, 179.41, bounce, block); hit {
		t.Fatalf("a line under the last fill must be ignored, got %q", reason)
	}
	// Earlier the same line still ran over the last fill and was honoured.
	earlier := testutil.At("17:45:00").UnixMilli()
	level, _ = projectLine(block.Resistance, earlier)
	if reason, hit := sellTargetReached(trade, st, level, earlier, block); !hit || reason != "resistance line" {
		t.Fatalf("a line over the last fill sells, got %q %v", reason, hit)
	}
}

func TestSellTargetInverseMirror(t *testing.T) {
	trade := testutil.LadderTrade(true, risingFills(5, "17:38:00")...) // last fill 108
	st := rebuildState(trade)
	block := risingBlock()
	now := testutil.At("17:45:00").UnixMilli()

	block.Support = aggragates.TrendLine{From: anchor("06:30:00", 90), To: anchor("11:45:00", 95)}
	level, _ := projectLine(block.Support, now)
	if level >= 108 || level <= block.LowerBB {
		t.Fatalf("the fixture wants the support between the band and the last fill, got %v", level)
	}
	if reason, hit := sellTargetReached(trade, st, level, now, block); !hit || reason != "support line" {
		t.Fatalf("touching the support sells an inverse ladder, got %q %v", reason, hit)
	}
	if _, hit := sellTargetReached(trade, st, level+0.01, now, block); hit {
		t.Fatal("over the support and over the band nothing sells")
	}

	block.Support = aggragates.TrendLine{}
	if reason, hit := sellTargetReached(trade, st, block.LowerBB, now, block); !hit || reason != "lower bollinger band" {
		t.Fatalf("touching the lower band sells, got %q %v", reason, hit)
	}
	if _, hit := sellTargetReached(trade, st, block.LowerBB+0.01, now, block); hit {
		t.Fatal("over the lower band nothing sells")
	}

	// A support projecting over the last fill is the mirror of the guard.
	block.Support = aggragates.TrendLine{From: anchor("06:30:00", 105), To: anchor("11:45:00", 110)}
	if reason, hit := sellTargetReached(trade, st, 109, now, block); hit {
		t.Fatalf("a support over the last fill must be ignored, got %q", reason)
	}
}

func TestSellTargetZeroBlockIsInert(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	now := testutil.At("17:45:00").UnixMilli()
	if reason, hit := sellTargetReached(trade, rebuildState(trade), 1e9, now, aggragates.SmartTakeLossIndicators{}); hit {
		t.Fatalf("no line and no band sell nothing, got %q", reason)
	}
}
