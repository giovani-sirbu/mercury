// Package depthpriority is the Cooldown flag's depth-priority gate: DepthPriorityHold refuses an
// entry while placing it would leave the wallet under what the ladder in front still needs for its
// remaining depths, costed DepthOffset depths short of its ceiling (LadderOf builds that view for
// every engine). The ranking (depthPriorityFor) puts the deepest ladder first, then the one planned
// cheaper to finish, then the lower trade id; a ladder at or past its costed ceiling keeps its
// place but keeps nothing. DepthPriorityApplies tells an engine which ticks consume the wallet
// view. The gate reads the wallet's ladders and free balance alone and fails open without them.
// Its vocabulary is package cooldown's (GateDepthPriority, DepthPriorityEvent,
// DepthPriorityHoldMarker).
//
// Vocabulary: depth, ladder, wallet, reserve, hold, event, row.
package depthpriority
