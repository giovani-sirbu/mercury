package depthpriority

const (
	// Enabled switches the wallet reserve (DepthPriorityHold); off, every ladder of the wallet
	// competes for the funds on its own.
	Enabled = true

	// DepthOffset is how many of a ladder's last depths the reserve leaves out: at 0 the wallet is
	// kept through the ceiling (a ladder of 9 depths reserves through depth 9), at 1 through depth
	// 8, at 2 through depth 7. A ladder at or past that costed ceiling is full for the gate and
	// keeps nothing, so its last depths compete for the wallet like any other entry.
	DepthOffset = 0
)

// The constants' in-force mirror; the suites pin it per test.
var depthOffset = DepthOffset
