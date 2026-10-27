package smarttakeloss

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/capitalprotection"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/flatmarket"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/freefallprevention"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
)

const (
	// PauseWhenActiveDepthPriority switches the pause a cooldown depth priority hold row stamped
	// after the newest fill puts on the slow decline, flat market and free fall prevention gates
	// until the next fill (depthPriorityHeld); capital protection is never paused.
	PauseWhenActiveDepthPriority = false
	// SellLossStartAtDepth is the depth (filled entries, ladder.CountFilledEntries) a ladder in buy
	// must reach before the shared action activates its stopLoss, read before the ceiling and the loss;
	// under it the action waits with one held row per episode and gate. Zero or below is off.
	SellLossStartAtDepth = 1
	// SellLossIgnoreLastDepths is how many of the last depths (ladder.ConfiguredDepths) the shared
	// action ignores: filled into them, or with an unknown ceiling, it neither activates, trails nor
	// sells and waits with one held row per episode and gate. Zero or below is off.
	SellLossIgnoreLastDepths = 0
	// WaitAfterTrigger is how long the slow decline, flat market and free fall prevention gates wait
	// from their last trigger before the shared action judges it: no held row, no activation, no sale
	// meanwhile, the episode and the ladder running as they would. Each trigger restarts it (the flat
	// market's and free fall prevention's triggered row, the slow decline's pending row); a committed
	// sale trails whatever. Zero or below waits nothing, as does an engine that hands Apply no clock;
	// capital protection waits capitalprotection.WaitAfterTrigger.
	WaitAfterTrigger = 2 * time.Hour
	// ActivationConfirmation is how long after a reached row a reached print must come before the
	// shared action activates buy or sells a self-armed stopLoss: the first reached print writes the
	// reached row and holds the ladder, reached prints until then hold it with no row. Zero or below
	// acts on the first reached print, as does an engine that hands Apply no clock.
	ActivationConfirmation = 5 * time.Second
	// ActivationConfirmationWindow is how old a reached row may be and still be confirmed: a reached
	// print later than that, or after an entry fill newer than the row (its FilledMoment), writes a new
	// reached row, so a gate hovering at its trigger writes at most one per window and fill. Zero or
	// below is no window: a reached row is never stale by age.
	ActivationConfirmationWindow = 1 * time.Minute
	// SaleGapTolerances is how many of the held depth's tolerances under the anchor a print may gap
	// before the committed sale is held: a move under minus the tolerance but not under minus
	// SaleGapTolerances tolerances sells, one further under holds (the print may be a spike) and the
	// trail goes on from the same anchor. The first gapped print opens a gap episode with its gap row
	// (EventGapHeld), which SaleGapWait bounds. Zero or below sells whatever the gap.
	SaleGapTolerances = 5.0
	// SaleGapWait is how long a print under the gap floor must stand before the committed sale
	// executes at the print: gapped prints after the gap row hold until that long has passed since it,
	// then sell (the gap was a move, not a spike); a print back over the floor before that closes the
	// gap with its row (EventGapClosed), and a later gap opens a new one. Zero or below holds without
	// end, as does an engine that hands Apply no clock.
	SaleGapWait = 5 * time.Second
)

// SellLossTrigger is one rule of SellLossTriggers, read against the SMC trend dashboard the block
// serves, down for a long ladder and up for an inverse one: MinTrendDirection the fewest rows whose
// Trend Direction reads the ladder's way (zero or below asks nothing), MinReads the fewest reads of
// each row that must read that way, in dashboard order (1h, 4h, 1D; a missing or zero entry asks nothing of
// that row, a row the dashboard did not serve fails the rule), MinFreeFall the rows, in dashboard
// order, that must be in free fall the ladder's way (a resistance and no support left below within
// the S/R look-back for a long ladder, the mirror for an inverse one; an entry above zero asks it of
// that row, zero or a missing entry asks nothing, a row the dashboard did not serve fails the rule),
// Percentage the profit, in percent of the amount used (saleProfit), at or over which buy is
// activated or a self-armed stopLoss sold: negative takes a loss that large, zero sells at break
// even, positive waits for that profit.
type SellLossTrigger struct {
	MinTrendDirection int
	MinReads          []int
	MinFreeFall       []int
	Percentage        float64
}

// SellLossTriggers is the shared action's trigger table, read on every tick off the tick's block
// (sharedTriggerOf): of the rules that hold the strictest names the trigger — more MinTrendDirection
// asked, then a higher sum of MinReads, then more rows asked in free fall (MinFreeFall), then the
// one listed last — so the order of the table does not matter. No rule holding, or the dashboard
// not read, holds the activation and a self-armed sale; a committed sale trails whatever the trend
// or profit, and capital protection reads capitalprotection.MaxDrawdownPercentage as a loss in its
// place. A sale the profit holds logs its held row again whenever the trigger moves, and a take
// profit proposed in buy gives way to the activation once ActivationConfirmation confirms the trigger.
var SellLossTriggers = []SellLossTrigger{
	{MinTrendDirection: 2, MinReads: []int{1, 7, 8}, MinFreeFall: []int{0, 0, 0}, Percentage: 1},
	{MinTrendDirection: 2, MinReads: []int{1, 8, 8}, MinFreeFall: []int{0, 0, 0}, Percentage: 0.7},
	{MinTrendDirection: 2, MinReads: []int{1, 9, 8}, MinFreeFall: []int{0, 0, 0}, Percentage: 0.6},
	{MinTrendDirection: 2, MinReads: []int{1, 9, 9}, MinFreeFall: []int{0, 0, 0}, Percentage: 0.5},
	{MinTrendDirection: 2, MinReads: []int{2, 6, 6}, MinFreeFall: []int{0, 0, 0}, Percentage: 0.5},
	{MinTrendDirection: 2, MinReads: []int{2, 8, 9}, MinFreeFall: []int{0, 0, 0}, Percentage: 0.3},
	{MinTrendDirection: 2, MinReads: []int{6, 8, 4}, MinFreeFall: []int{0, 0, 1}, Percentage: 0.2},
	{MinTrendDirection: 2, MinReads: []int{1, 8, 8}, MinFreeFall: []int{0, 1, 1}, Percentage: -0.5},
	{MinTrendDirection: 2, MinReads: []int{1, 8, 8}, MinFreeFall: []int{1, 1, 1}, Percentage: -0.5},
	{MinTrendDirection: 3, MinReads: []int{5, 5, 5}, MinFreeFall: []int{0, 1, 1}, Percentage: 0.5},
	{MinTrendDirection: 3, MinReads: []int{8, 5, 8}, MinFreeFall: []int{0, 1, 1}, Percentage: 0.25},
	{MinTrendDirection: 3, MinReads: []int{7, 7, 5}, MinFreeFall: []int{0, 1, 1}, Percentage: 0.1},
	{MinTrendDirection: 3, MinReads: []int{8, 8, 8}, MinFreeFall: []int{1, 1, 1}, Percentage: -0.15},
	{MinTrendDirection: 3, MinReads: []int{7, 9, 7}, MinFreeFall: []int{1, 1, 1}, Percentage: -0.15},
	{MinTrendDirection: 3, MinReads: []int{9, 8, 8}, MinFreeFall: []int{1, 1, 1}, Percentage: -0.15},
	{MinTrendDirection: 3, MinReads: []int{9, 9, 9}, MinFreeFall: []int{1, 1, 1}, Percentage: -0.2},
}

// The switches and the bounds as the package reads them; variables only so the tests can move them.
var (
	quietDecline                       = quietdecline.Enabled
	swingDecline                       = swingdecline.Enabled
	earlyTakeProfit                    = earlytakeprofit.Enabled
	earlyTakeProfitQuietDecline        = earlytakeprofit.QuietDecline
	linearDecline                      = lineardecline.Enabled
	earlyTakeProfitSwingDecline        = earlytakeprofit.SwingDecline
	earlyTakeProfitLinearDecline       = earlytakeprofit.LinearDecline
	earlyTakeProfitFlatMarket          = earlytakeprofit.FlatMarket
	earlyTakeProfitFreeFallPrevention  = earlytakeprofit.FreeFallPrevention
	earlyTakeProfitDepthPriority       = earlytakeprofit.DepthPriority
	earlyTakeProfitDynamicParams       = earlytakeprofit.DynamicParams
	capitalProtectionExit              = capitalprotection.Enabled
	capitalProtectionMaxDrawdown       = capitalprotection.MaxDrawdownPercentage
	capitalProtectionWaitAfterTrigger  = capitalprotection.WaitAfterTrigger
	flatMarket                         = flatmarket.Enabled
	flatMarketHoldInitialDepth         = flatmarket.HoldInitialDepth
	flatMarketResetOnNewDepth          = flatmarket.ResetOnNewDepth
	freeFallPrevention                 = freefallprevention.Enabled
	freeFallPreventionHoldInitialDepth = freefallprevention.HoldInitialDepth
	freeFallPreventionResetOnNewDepth  = freefallprevention.ResetOnNewDepth
	swingDeclineResetOnNewDepth        = swingdecline.ResetOnNewDepth
	linearDeclineResetOnNewDepth       = lineardecline.ResetOnNewDepth
	quietDeclineNeedsRecentFill        = quietdecline.NeedsRecentFill
	pauseWhenActiveDepthPriority       = PauseWhenActiveDepthPriority
	sellLossStartAtDepth               = SellLossStartAtDepth
	sellLossIgnoreLastDepths           = SellLossIgnoreLastDepths
	waitAfterTrigger                   = WaitAfterTrigger
	activationConfirmation             = ActivationConfirmation
	activationConfirmationWindow       = ActivationConfirmationWindow
	saleGapTolerances                  = SaleGapTolerances
	saleGapWait                        = SaleGapWait
	sellLossTriggers                   = SellLossTriggers
)
