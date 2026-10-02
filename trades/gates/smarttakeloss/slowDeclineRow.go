package smarttakeloss

import (
	"slices"
	"strings"
)

// Finding is what one feature of the slow decline gate read on a tick: the
// feature, one of the Feature constants, and the reasons it names on the row —
// the exit reasons or the break reasons of a decline, the sophos reasons of the
// indecision latch. A Finding with no reasons still names its feature in the
// row's head and in its event's features, and prints no group.
type Finding struct {
	Feature string
	Reasons []string
}

// featureOrder is the features in the order every row names them, with the
// words a row prints for each.
var featureOrder = []struct{ feature, label string }{
	{FeatureIndecision, "indecision direction"},
	{FeatureQuietDecline, "quiet decline"},
	{FeatureSwingDecline, "swing decline"},
}

// PendingRow is the gate's row at the fill on the tick a ladder goes pending: the
// findings are the features that triggered — the quiet decline, the swing
// decline or both — and FeatureIndecision when the latch landed on the same
// tick. Apply and a fixture build the row with it alike (buildRow gives its text).
func PendingRow(fill float64, findings ...Finding) Row {
	return buildRow(rowTriggered, fill, findings)
}

// ConfirmedRow is the row of a pending ladder's new fill the gate confirmed, on
// the decline features whose reading held; its event is the pending kind.
func ConfirmedRow(fill float64, findings ...Finding) Row {
	return buildRow(rowConfirmed, fill, findings)
}

// CancelledRow is the row of a pending ladder's new fill the gate cancelled,
// naming what broke in each decline feature that applied.
func CancelledRow(fill float64, findings ...Finding) Row {
	return buildRow(rowCancelled, fill, findings)
}

// ResetRow is the row of a pending ladder a depth priority holds: the findings
// are the decline features the ladder was pending on, with no reasons.
func ResetRow(fill float64, findings ...Finding) Row {
	return buildRow(rowReset, fill, findings)
}

// LatchedRow is the row of a tick that latched the indecision direction and did
// nothing else: the finding names the sophos reasons, or — when the swing
// decline latched a ladder already pending — the swing decline's. Its event
// carries FeatureIndecision either way.
func LatchedRow(fill float64, findings ...Finding) Row {
	return buildRow(rowLatched, fill, findings)
}

// buildRow is the one grammar of the gate's rows. The message is "smartTakeLoss:
// ", a head, and the reasons in parentheses when any finding names some. The
// head is, by kind: the features' labels listed ("a", "a and b", "a, b and c")
// then "triggered"; "indecision direction triggered" for the latch alone; the
// decline features' labels then "confirmed at the new fill"; "slow decline
// cancelled at the new fill"; "slow decline reset, depth priority holds this
// ladder" — the last three behind "indecision direction triggered, " when the
// latch landed on the same row. The reasons are one group per finding, "label:
// reason, reason", joined by "; " in the order of featureOrder, and findings
// that name the same reasons print them once under the labels of all of them.
// Event.Reasons are the printed groups flattened and Event.Features the
// findings' features in the same order, FeatureIndecision alone for the latch.
func buildRow(kind rowKind, fill float64, findings []Finding) Row {
	ordered := inFeatureOrder(findings)
	row := Row{Price: fill, Gate: GateSlowDecline, Event: kind.event(), Features: featuresOf(kind, ordered)}
	message := "smartTakeLoss: " + headOf(kind, ordered)
	var groups []string
	for _, group := range groupsOf(ordered) {
		groups = append(groups, labelList(group.features)+": "+strings.Join(group.reasons, ", "))
		row.Reasons = append(row.Reasons, group.reasons...)
	}
	if len(groups) > 0 {
		message += " (" + strings.Join(groups, "; ") + ")"
	}
	row.Message = message
	return row
}

// inFeatureOrder is the findings of the known features, the first of each, in
// featureOrder.
func inFeatureOrder(findings []Finding) []Finding {
	var ordered []Finding
	for _, known := range featureOrder {
		if at := slices.IndexFunc(findings, func(finding Finding) bool { return finding.Feature == known.feature }); at >= 0 {
			ordered = append(ordered, findings[at])
		}
	}
	return ordered
}

// featuresOf is the features of the event beside a row: the findings', or the
// indecision alone for the row that only latched.
func featuresOf(kind rowKind, ordered []Finding) []string {
	if kind == rowLatched {
		return []string{FeatureIndecision}
	}
	var features []string
	for _, finding := range ordered {
		features = append(features, finding.Feature)
	}
	return features
}

// headOf is the head of a row's message (buildRow).
func headOf(kind rowKind, ordered []Finding) string {
	var declines []string
	prefix := ""
	for _, finding := range ordered {
		if finding.Feature == FeatureIndecision {
			prefix = labelOf(FeatureIndecision) + " triggered, "
			continue
		}
		declines = append(declines, finding.Feature)
	}
	switch kind {
	case rowTriggered:
		return labelList(featuresOf(kind, ordered)) + " triggered"
	case rowLatched:
		return labelOf(FeatureIndecision) + " triggered"
	case rowConfirmed:
		return prefix + labelList(declines) + " confirmed at the new fill"
	case rowCancelled:
		return prefix + "slow decline cancelled at the new fill"
	}
	return prefix + "slow decline reset, depth priority holds this ladder"
}

// reasonGroup is the reasons one or more findings name alike.
type reasonGroup struct {
	features []string
	reasons  []string
}

// groupsOf is the groups a row prints: one per finding that names reasons, the
// findings that name the same ones sharing a group, in the order of the first.
func groupsOf(ordered []Finding) []reasonGroup {
	var groups []reasonGroup
	for _, finding := range ordered {
		if len(finding.Reasons) == 0 {
			continue
		}
		shared := slices.IndexFunc(groups, func(group reasonGroup) bool { return slices.Equal(group.reasons, finding.Reasons) })
		if shared < 0 {
			groups = append(groups, reasonGroup{features: []string{finding.Feature}, reasons: finding.Reasons})
			continue
		}
		groups[shared].features = append(groups[shared].features, finding.Feature)
	}
	return groups
}

// labelOf is the words a row prints for a feature.
func labelOf(feature string) string {
	for _, known := range featureOrder {
		if known.feature == feature {
			return known.label
		}
	}
	return feature
}

// labelList is the labels of the features listed as a sentence does: "a",
// "a and b", "a, b and c". No feature at all reads as the gate, "slow decline".
func labelList(features []string) string {
	labels := make([]string, len(features))
	for index, feature := range features {
		labels[index] = labelOf(feature)
	}
	switch len(labels) {
	case 0:
		return "slow decline"
	case 1:
		return labels[0]
	}
	return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
}
