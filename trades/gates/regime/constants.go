package regime

// Regime labels served by sophos' multi-timeframe regime set. The strings
// are the wire contract shared with sophos.
const (
	Shock       = "shock"
	ShockDown   = "shock-down"
	ShockUp     = "shock-up"
	DownPersist = "downtrend-persist"
	UpPersist   = "uptrend-persist"
)

// Editable gate-policy knobs, calibrated on run 73 (6 simulated months): of
// its 29 shock holds, 26 landed on trades 0-1 entries deep — the cheapest
// fills a grid gets during volatility — while the crash guard's own deep-hold
// fired once. The shock veto therefore starts at a depth, and only off the
// trigger timeframe.
const (
	// ShockHoldMinDepth: a 15m shock parks a rebuy only from this many filled
	// entries up. Shallow rungs trade straight through volatility spikes.
	ShockHoldMinDepth = 3
	// ShockHoldTimeframe: the one timeframe whose shock parks a rebuy. Higher
	// timeframes' shocks last hours per bar and are the crash guard's job.
	// The crash guard reads the same timeframe for its capitulation freeze.
	ShockHoldTimeframe = "15m"
	// profitHoldTimeframe: the timeframe whose persist label must move in the
	// trade's favor for the deferral to stand — uptrend for a long close,
	// downtrend for an inverse buyback. Any other label (flat, mixed, shock
	// in either direction) releases the close to the deterministic engine.
	profitHoldTimeframe = "15m"
	// ProfitHoldMinDepth: the profit hold only defers a close from this many
	// filled entries up. Measured on runs 90/94, the deferral's value is all
	// at depth 4+; shallower it was a negative coin flip that tied capital.
	ProfitHoldMinDepth = 4
)

// Hold-reason prefixes. The crash guard's capitulation override matches a
// regime hold by these prefixes (crashguard.capitulationEligibleHold), so
// they are the contract between the two packages: change the text here and
// nowhere else.
const (
	ShockHoldPrefix      = "regime: market in shock"
	AddVetoPrefix        = "regime: add not allowed"
	InverseAddVetoPrefix = "regime: inverse add not allowed"
)

// addVetoTimeframes is the inverse add veto's either-of pair, the rule that
// saved run 74's rally blow-ups. longAddVetoTimeframes names the timeframes
// sophos folds into the long-side addAllowed (regime/set.go): it exists so
// regimeDetail names the real blocker, and it changes together with sophos.
// The long pair moved from 4h+1h to 4h+2h: the 1h leg measured ~zero value
// on runs 88-94 while it kept trade 20237 out of adds for 20 days.
var (
	addVetoTimeframes     = []string{"4h", "1h"}
	longAddVetoTimeframes = []string{"4h", "2h"}
)
