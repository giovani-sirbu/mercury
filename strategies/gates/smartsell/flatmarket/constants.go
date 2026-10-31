package flatmarket

import (
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
)

// The gate.
var (
	// Enabled switches the gate: its watch and the episode feeding the shared action (flatMarketTick);
	// the first-fill hold (HoldInitialDepth) and the latch on its readings (earlytakeprofit.FlatMarket)
	// read whatever this says.
	Enabled = false
	// HoldInitialDepth switches the first-fill hold on the reading (EntryHold), whatever Enabled says.
	HoldInitialDepth = true
	// ResetOnNewDepth switches the reset of an open episode at a new depth (flatMarketTick); off,
	// the episode stands through new fills until the sale.
	ResetOnNewDepth = true
	// ArmDepth is the fewest filled entries before the gate reads a ladder (flatMarketWatched).
	ArmDepth = 2
	// MinSmcTrend is the fewest Trend Direction rows reading the ladder's way (down for a long, up
	// for an inverse) for a reading to hold (series.Trend.Meets); zero or below switches it off.
	MinSmcTrend = 3
	// Timeframe is the exchange interval of the bars sophos serves (helpers.IntervalToMinutes).
	Timeframe = "1h"
	// WindowTimeframe is the span the bars cover; Window is its length in bars of Timeframe.
	WindowTimeframe = 36 * time.Hour
)

// The range reading (TriggerRange).
var (
	// RangeBars is how many of the newest bars of the window the range reads (Bars.Tail).
	RangeBars = 36
	// BodyShare is the share of the ladder's base percentage a bar's body stays under to read
	// quiet (RangeReading.BodyMaxPct).
	BodyShare float64 = 0.75
	// QuietShare is the share of the bars that must be quiet, strictly more (RangeReading.Quiet of RangeReading.Of).
	QuietShare float64 = 0.9
	// MaxMovePct bounds the move on the ladder's side, strictly under (RangeReading.MovePct), and
	// the stray of every inner close from the mean of the first and last closes, inclusive (RangeReading.StrayPct).
	MaxMovePct float64 = 2
	// MaxCloseRangePct bounds the span of all the closes, highest over lowest, inclusive
	// (RangeReading.CloseRangePct); zero or below switches the condition off.
	MaxCloseRangePct float64 = 3
)

// The line reading (TriggerLine).
var (
	// MaxDeviation is the share of the ladder's base percentage every close may stray from the
	// line and every body may span (LineReading.Bound), and the line may move before it reads as diagonal (flat).
	MaxDeviation float64 = 0.75
	// MaxSlopePercentage is the steepest the line may slope either way, in percent of its start per
	// hour (LineReading.SlopePct), however far it moves over the window; zero or below switches it off.
	MaxSlopePercentage float64 = 0.15
	// TolerancePercentage is the share of the bars that may count against the reading — a close
	// past the bound or a body over it, wherever its close lies (LineReading.Counted, LineReading.Allowed).
	TolerancePercentage float64 = 5
)

// Window is how many bars of Timeframe fit WindowTimeframe: the look-back sophos serves (Bars).
var Window = int(WindowTimeframe / (time.Duration(helpers.IntervalToMinutes(Timeframe)) * time.Minute))
