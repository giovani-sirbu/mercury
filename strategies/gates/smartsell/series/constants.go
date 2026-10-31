package series

// BarMs is one bar in ms, the 1h interval sophos serves the series in (its SwingDeclineInterval
// must equal it); BarOpen floors a stamp to it.
const BarMs = int64(3_600_000)
