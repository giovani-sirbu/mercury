package freefallprevention

import (
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
)

// The gate.
var (
	// Enabled switches the gate: its watch and the episode feeding the shared action
	// (freeFallPreventionTick); the first-fill hold (HoldInitialDepth) and the latch on its reading
	// (earlytakeprofit.FreeFallPrevention) read whatever this says.
	Enabled = true
	// HoldInitialDepth switches the first-fill hold on the reading (EntryHold), whatever Enabled says.
	HoldInitialDepth = true
	// ResetOnNewDepth switches the reset of an open episode at a new depth (freeFallPreventionTick);
	// off, the episode stands through new fills until the sale.
	ResetOnNewDepth = true
	// ArmDepth is the fewest filled entries before the gate reads a ladder (freeFallPreventionWatched).
	ArmDepth = 2
	// MinShare is the share of the window's bars on which at least MinFreeFallTimeframes counted rows
	// must be free the ladder's way, strictly more (Reading.Count of Reading.Of).
	MinShare float64 = 0.8
	// Timeframe is the exchange interval of the bars sophos serves (helpers.IntervalToMinutes).
	Timeframe = "1h"
	// WindowTimeframe is the span the bars cover; Window is its length in bars of Timeframe.
	WindowTimeframe = 24 * time.Hour
)

// MinFreeFallTimeframes is how many rows in free fall the ladder's way (free rise for an inverse
// one) hold a reading, on the last closed bar or on a bar of the share (Trigger); at or below zero
// the gate reads nothing.
var MinFreeFallTimeframes = 2

// FreeFallTimeframes is the timeframe labels whose rows count toward MinFreeFallTimeframes,
// matched whatever their case; empty counts every row.
var FreeFallTimeframes = []string{"1h", "4h", "1d"}

// Window is how many bars of Timeframe fit WindowTimeframe: the look-back sophos serves (Series).
var Window = int(WindowTimeframe / (time.Duration(helpers.IntervalToMinutes(Timeframe)) * time.Minute))
