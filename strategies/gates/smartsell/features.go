package smartsell

// The features of the slow decline gate as an event names them (EventData.Features) and as a row
// names its findings (Finding.Feature).
const (
	// FeatureEarlyTakeProfit is the early take profit's latch; an event carrying it latched the ladder.
	FeatureEarlyTakeProfit = "earlyTakeProfit"
	// FeatureIndecision is sophos' indecision reading the latch landed on, a row label only; events
	// written before the latch was named carry it as the latch itself (foldGateEvent).
	FeatureIndecision = "indecision"
	// FeatureQuietDecline is the quiet decline, which makes a ladder pending.
	FeatureQuietDecline = "quietDecline"
	// FeatureSwingDecline is the swing decline, which makes a ladder pending.
	FeatureSwingDecline = "swingDecline"
	// FeatureLinearDecline is the linear decline, which makes a ladder pending with
	// lineardecline.Enabled on; its latch alone is a row label whose event carries FeatureEarlyTakeProfit.
	FeatureLinearDecline = "linearDecline"
	// FeatureFlatMarket is the flat market's latch, a row label only, like FeatureLinearDecline.
	FeatureFlatMarket = "flatMarket"
	// FeatureFreeFallPrevention is the free fall prevention's latch, a row label only, like FeatureFlatMarket.
	FeatureFreeFallPrevention = "freeFallPrevention"
	// FeatureDepthPriority is the latch on a depth priority hold, a row label only.
	FeatureDepthPriority = "depthPriority"
	// FeatureDynamicParams is the latch on a ladder that opened raised, a row label only.
	FeatureDynamicParams = "dynamicParams"
)

// declineFeature is whether a feature is a decline the slow decline gate goes pending on.
func declineFeature(feature string) bool {
	return feature == FeatureQuietDecline || feature == FeatureSwingDecline || feature == FeatureLinearDecline
}
