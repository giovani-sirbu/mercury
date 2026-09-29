package dynamicparams

// BearPercentagePoints is added to the percentage of every row while the
// increase names the percentage (IncreasePercentage, IncreaseBoth): points
// added to the row's own percentage, never a multiple of it. It must stay at
// or above zero. The backtest's skip gates keep reading the stored rows, and
// they skip only prints the raised strategy does not act on while the
// increase widens the ladder; an amount under zero would narrow it, and the
// replay would skip prints the raised strategy acts on.
const BearPercentagePoints float64 = 0.5

// BearDepths is added to the depths of every row while the increase names
// the depths (IncreaseDepths, IncreaseBoth), so the first entry of a ladder
// that opens while raised is sized for that many more entries. It is a whole
// number of depths: ladder.CalculateInitialBid walks a row's depths down in
// hundredths and sizes only on a half depth, and a fraction added to a row
// can keep that walk off every half depth, which refuses every new ladder. It
// must stay at or above zero, for the reason BearPercentagePoints gives.
const BearDepths int = 1

// MixedIncrease is what a mixed tier raises — exactly one of the two reads
// bearish: the percentage, the depths, both, or nothing (IncreaseNone).
const MixedIncrease Increase = IncreasePercentage
