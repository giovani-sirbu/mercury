package firstfill

import "time"

// FirstFillMaxHold caps how long the first-fill gate holds an entry on the tick clock from the first
// activated event (firstFillExpired); past it the entry goes in at the tick price. Zero disables the
// cap; an unknown clock never expires.
const FirstFillMaxHold = 3 * time.Hour
