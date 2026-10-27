package dynamicparams

// Interval is the Binance interval the engines request GET /:symbol/smc-trend at; the chart row of
// that interval is the read (aggragates.SophosSmcTrend.DynamicParams).
const Interval = "1h"

// BearPercentagePoints is the points added to every row's percentage when the increase names it,
// read once at open and written into the opened event (OpenedRaise); at or above zero.
const BearPercentagePoints float64 = 0.1

// BearDepths is the whole depths added to every row's depths when the increase names them, read
// once at open like BearPercentagePoints; at or above zero.
const BearDepths int = 1

// MixedIncrease is what a mixed tier (exactly one read bearish) raises: the percentage, the
// depths, both or nothing.
const MixedIncrease Increase = IncreaseBoth

// MinFreeFallTimeframes is how many rows in free fall open a ladder raised as both bearish do,
// whatever the two reads say (FreeFall); at or below zero the trigger is off.
var MinFreeFallTimeframes = 2

// FreeFallTimeframes is the timeframe labels whose rows count toward MinFreeFallTimeframes,
// matched whatever their case; empty counts every row.
var FreeFallTimeframes = []string{"1h", "4h", "1d"}
