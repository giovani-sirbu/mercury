// FLAT MARKET. The flat market gate (GateFlatMarket) is a sibling of the slow decline gate under
// the same flag, its episode and sale switched by flatmarket.Enabled: it makes no ladder pending
// and judges no fill, and folds its events apart (foldFlatMarketEvents, the legacy low interest and
// non interest area events among them). Its reading (flatMarketReading, flatmarket.Trigger) is the
// range and the line read on the last closed bars against the ladder's base percentage and side
// under the Trend Direction row; it watches every spot ladder from flatmarket.ArmDepth, long or
// inverse. The first holding tick opens a sticky episode with the triggered row, closed by the sale
// and, with flatmarket.ResetOnNewDepth, by a new depth; the activation, trailing and sale go
// through the shared action (saleAt), its held row written again whenever the trigger moves and
// its activation taking the place of a take profit proposed in buy. Whatever Enabled says, its
// latch reading latches the early take profit (earlytakeprofit.FlatMarket) and its reading holds
// the first fill while flatmarket.HoldInitialDepth is on (EntryHold). A depth priority hold pauses
// the gate.
package smartsell
