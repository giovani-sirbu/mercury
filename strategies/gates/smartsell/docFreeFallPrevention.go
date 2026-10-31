// FREE FALL PREVENTION. The free fall prevention gate (GateFreeFallPrevention) is a sibling of the
// slow decline and flat market gates under the same flag, its episode and sale switched by
// freefallprevention.Enabled: it makes no ladder pending and judges no fill, and folds its events
// apart (foldFreeFallPreventionEvents). Its reading (freeFallPreventionReading,
// freefallprevention.Trigger) is at least freefallprevention.MinFreeFallTimeframes of the SMC trend
// rows freefallprevention.FreeFallTimeframes counts in free fall the ladder's way (free rise for an
// inverse one), on the last closed bar or on more than freefallprevention.MinShare of the window's
// bars; it watches every spot ladder from freefallprevention.ArmDepth, long or inverse. The first
// holding tick opens a sticky episode with the triggered row unless the slow decline or flat market
// gate's is open (one at a time), closed by the sale and, with freefallprevention.ResetOnNewDepth,
// by a new depth; the activation, trailing and sale go through the shared action (saleAt), its
// held row written again whenever the trigger moves and its activation taking the place of a take
// profit proposed in buy. Whatever Enabled says, its reading latches the early take profit
// (earlytakeprofit.FreeFallPrevention) and holds the first fill while
// freefallprevention.HoldInitialDepth is on (EntryHold). A depth priority hold pauses the gate.
package smartsell
