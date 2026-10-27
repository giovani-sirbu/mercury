package flatmarket

import (
	"fmt"
	"math"
)

// lineHoldReasonCap is the most reasons a holding line reading names.
const lineHoldReasonCap = 5

// lineFrame opens the line's reason list (frame).
func lineFrame(reading LineReading) string {
	return frame("line", reading.From, reading.To)
}

// wayWord is the way a line moves: down or up.
func wayWord(movePct float64) string {
	if movePct < 0 {
		return "down"
	}
	return "up"
}

// lineHoldReasons are the reasons a holding line names, the only text TriggerLine formats: the
// frame, the line, the closes against it, the bars counted (if any) and the trend (unless off).
func lineHoldReasons(reading LineReading, need int) []string {
	reasons := make([]string, 0, lineHoldReasonCap)
	reasons = append(reasons, lineFrame(reading), lineReason(reading), deviationReason(reading))
	if reading.Counted > 0 {
		reasons = append(reasons, countedReason(reading))
	}
	if need > 0 {
		reasons = append(reasons, trendHoldReason(reading.Trend, reading.Side, need))
	}
	return reasons
}

// lineReason names the line: flat within the bound, or moving the ladder's way over the window.
func lineReason(reading LineReading) string {
	line := fmt.Sprintf("line %s %.2f%% over the window", wayWord(reading.MovePct), math.Abs(reading.MovePct))
	if flat(reading.MovePct, reading.Bound) {
		line = fmt.Sprintf("line flat, %+.2f%% over the window within %.2f%%", reading.MovePct, reading.Bound)
	}
	if reading.MaxSlope > 0 {
		line += fmt.Sprintf(", %.2f%%/h under %.2f%%/h", math.Abs(reading.SlopePct), reading.MaxSlope)
	}
	return line
}

// deviationReason names the farthest closes either side of the line against the bound.
func deviationReason(reading LineReading) string {
	return fmt.Sprintf("closes within %+.2f%%/%+.2f%% of the line, at most %.2f%%", reading.MaxAbove, reading.MaxBelow, reading.Bound)
}

// countedReason names the bars the tolerance let through: a close past the bound or a body over it.
func countedReason(reading LineReading) string {
	return fmt.Sprintf("%d of %d bars over the bound (%d on their body), at most %d", reading.Counted, reading.Of, reading.Wide, reading.Allowed)
}

// directionBreak names a line that moves against the ladder past the bound.
func directionBreak(reading LineReading) string {
	return fmt.Sprintf("line %s %.2f%% over the window against the ladder, flat within %.2f%%", wayWord(reading.MovePct), math.Abs(reading.MovePct), reading.Bound)
}

// slopeBreak names a line that slopes steeper than the cap, per hour either way.
func slopeBreak(reading LineReading) string {
	return fmt.Sprintf("line slopes %.2f%%/h, over the %.2f%%/h allowed", math.Abs(reading.SlopePct), reading.MaxSlope)
}

// countedBreak names more bars counted than the tolerance allows, with the widest body.
func countedBreak(reading LineReading, found deviations) string {
	return fmt.Sprintf("%d of %d bars over the bound %.2f%% (%d on their body, the widest %+.2f%% at %s), at most %d",
		reading.Counted, reading.Of, reading.Bound, reading.Wide, found.widest, barTime(found.widestAt), reading.Allowed)
}
