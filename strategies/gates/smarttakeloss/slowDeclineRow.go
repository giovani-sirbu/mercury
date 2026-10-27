package smarttakeloss

import (
	"slices"
	"strings"
)

// Finding is what one feature read on a tick: the feature (a Feature constant) and the reasons the
// row names; one with no reasons names its feature in the head and prints no group.
type Finding struct {
	Feature string
	Reasons []string
}

// featureOrder is the features in the order a row names them, with the words it prints.
var featureOrder = []struct{ feature, label string }{
	{FeatureEarlyTakeProfit, "early take profit"},
	{FeatureIndecision, "indecision"},
	{FeatureDepthPriority, "depth priority"},
	{FeatureDynamicParams, "dynamic params"},
	{FeatureQuietDecline, "quiet decline"},
	{FeatureSwingDecline, "swing decline"},
	{FeatureLinearDecline, "linear decline"},
	{FeatureFlatMarket, "flat market"},
	{FeatureFreeFallPrevention, "free fall prevention"},
}

// PendingRow is the gate's row at the fill a ladder goes pending from: the findings are the features
// that triggered, and FeatureEarlyTakeProfit when the latch landed on the same tick.
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

// LatchedRow is the row of a tick that only latched the early take profit: the findings name the
// reading it landed on.
func LatchedRow(fill float64, findings ...Finding) Row {
	return buildRow(rowLatched, fill, findings)
}

// buildRow is the one grammar of the gate's rows: "smartTakeLoss: " + the kind's head (headOf) +
// the reasons in parentheses, one group per finding in featureOrder (groupsOf); Event.Reasons are
// the groups flattened, Event.Features the findings' features (featuresOf).
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

// featuresOf is the features of the event beside a row: the latch first when it landed, then the
// decline features; the latch alone for the row that only latched. The readings' labels stay in
// the message.
func featuresOf(kind rowKind, ordered []Finding) []string {
	if kind == rowLatched {
		return []string{FeatureEarlyTakeProfit}
	}
	var features []string
	if latched(ordered) {
		features = append(features, FeatureEarlyTakeProfit)
	}
	return append(features, declinesOf(ordered)...)
}

// latched is whether the findings carry the latch: its marker (FeatureEarlyTakeProfit), or sophos'
// indecision (FeatureIndecision), which the latch alone names.
func latched(ordered []Finding) bool {
	return slices.ContainsFunc(ordered, func(finding Finding) bool {
		return finding.Feature == FeatureEarlyTakeProfit || finding.Feature == FeatureIndecision
	})
}

// declinesOf are the decline features among the findings, the ones a head names.
func declinesOf(ordered []Finding) []string {
	var declines []string
	for _, finding := range ordered {
		if declineFeature(finding.Feature) {
			declines = append(declines, finding.Feature)
		}
	}
	return declines
}

// headOf is the head of a row's message (buildRow): the latch first when it landed, then the kind's
// words on the decline features.
func headOf(kind rowKind, ordered []Finding) string {
	prefix := ""
	if latched(ordered) {
		prefix = labelOf(FeatureEarlyTakeProfit) + " latched, "
	}
	switch kind {
	case rowLatched:
		return labelOf(FeatureEarlyTakeProfit) + " latched"
	case rowTriggered:
		return prefix + labelList(declinesOf(ordered)) + " triggered"
	case rowConfirmed:
		return prefix + labelList(declinesOf(ordered)) + " confirmed at the new fill"
	case rowCancelled:
		return prefix + "slow decline cancelled at the new fill"
	}
	return prefix + "slow decline reset, depth priority holds this ladder"
}

// pendingMessage is a pending row's message with its declines saying verb and suffix after its
// reasons (mergedMessage); false without a verb or when the message is not buildRow's.
func pendingMessage(row Row, verb, suffix string) (string, bool) {
	if verb == "" {
		return "", false
	}
	findings := make([]Finding, 0, len(row.Features))
	for _, feature := range row.Features {
		findings = append(findings, Finding{Feature: feature})
	}
	ordered := inFeatureOrder(findings)
	if rest, ok := strings.CutPrefix(row.Message, "smartTakeLoss: "+headOf(rowTriggered, ordered)); ok {
		prefix := ""
		if latched(ordered) {
			prefix = labelOf(FeatureEarlyTakeProfit) + " latched, "
		}
		return "smartTakeLoss: " + prefix + labelList(declinesOf(ordered)) + " " + verb + rest + suffix, true
	}
	confirmed := headOf(rowConfirmed, ordered)
	if rest, ok := strings.CutPrefix(row.Message, "smartTakeLoss: "+confirmed); ok {
		return "smartTakeLoss: " + confirmed + ", " + verb + rest + suffix, true
	}
	return "", false
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
