package dynamicparams

// Interval is the Binance interval the engines request sophos'
// GET /:symbol/smc-trend at for the reads. The dashboard's chart row is the
// read (aggragates.SophosSmcTrend.DynamicParams), so Interval names the
// timeframe the rule reads on: the last closed bar of that interval as of the
// tick. It must be an interval Binance serves: sophos refuses any other, and
// every engine reads a refused fetch as not read, the configured rows.
const Interval = "1d"

// BearPercentagePoints is what a ladder that opens while the increase names
// the percentage (IncreasePercentage, IncreaseBoth) adds to the percentage of
// every row: points added to the row's own percentage, never a multiple of
// it. Opening reads it once, when the ladder opens, and writes it into the
// opened row; the ladder then trades the amount its row carries
// (OpenedRaise), so a retune reaches the next ladder that opens and never one
// already open. It must stay at or above zero. The backtest's skip gates keep
// reading the stored rows, and they skip only prints the raised strategy does
// not act on while the opened row widens the ladder; an amount under zero
// would narrow it, and the replay would skip prints the raised strategy acts
// on.
const BearPercentagePoints float64 = 0.4

// BearDepths is what a ladder that opens while the increase names the depths
// (IncreaseDepths, IncreaseBoth) adds to the depths of every row, so its
// first entry is sized for that many more entries. Opening reads it at open
// only and writes it into the opened row, as it does BearPercentagePoints. It
// is a whole number of depths: ladder.CalculateInitialBid walks a row's
// depths down in hundredths and sizes only on a half depth, and a fraction
// added to a row can keep that walk off every half depth, which refuses every
// new ladder. It must stay at or above zero, for the reason
// BearPercentagePoints gives.
const BearDepths int = 1

// MixedIncrease is what a mixed tier raises when a ladder opens — exactly one
// of the two reads bearish: the percentage, the depths, both, or nothing
// (IncreaseNone).
const MixedIncrease Increase = IncreasePercentage
