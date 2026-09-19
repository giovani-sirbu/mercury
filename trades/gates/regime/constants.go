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

// Editable gate-policy knobs. Ungated, the shock hold lands almost entirely
// on shallow trades — the cheapest fills a grid gets during volatility —
// while the deep ones it was meant for are already the crash guard's. The
// shock veto therefore starts at a depth, and only off the trigger timeframe.
const (
	// ShockHoldMinDepth: a shock on ShockHoldTimeframe parks a rebuy only from
	// this many filled entries up. Shallow depths trade straight through
	// volatility spikes.
	ShockHoldMinDepth = 3
	// ShockHoldTimeframe: the one timeframe whose shock parks a rebuy. Higher
	// timeframes' shocks span far longer per bar and are the crash guard's
	// job. The crash guard reads the same timeframe for its capitulation
	// freeze.
	ShockHoldTimeframe = "15m"
	// profitHoldTimeframe: the timeframe whose persist label must move in the
	// trade's favor for the deferral to stand — uptrend for a long close,
	// downtrend for an inverse buyback. Any other label (flat, mixed, shock
	// in either direction) releases the close to the deterministic engine.
	profitHoldTimeframe = "15m"
	// ProfitHoldMinDepth: the profit hold only defers a close from this many
	// filled entries up. The deferral pays only on a deep ladder; shallower it
	// is a coin flip that ties up capital for nothing.
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

// addVetoTimeframes is the inverse add veto's either-of pair, evaluated here
// (HoldReason): the rule that keeps an inverse ladder from adding into a
// rally. longAddVetoTimeframes names the timeframes sophos folds into the
// long-side addAllowed (regime/set.go); mercury never evaluates that one, it
// exists so regimeDetail names the real blocker, and it changes together with
// sophos.
//
// The two pairs share their slow leg and differ in the fast one:
// longAddVetoTimeframes' second leg is a step slower than addVetoTimeframes'.
// That asymmetry is the rule, not an oversight. A long veto only has to keep
// a ladder out of a grind, which the slow leg already labels, so a faster
// second leg there blocked adds for long stretches while rarely saying
// anything the slow leg had not. The inverse veto also has to catch a
// vertical squeeze — the rally it exists for, arriving too fast for a slow
// label to carry it (ShockBlocks in HoldReason) — and that is what its faster
// leg buys.
var (
	addVetoTimeframes     = []string{"4h", "1h"}
	longAddVetoTimeframes = []string{"4h", "2h"}
)
