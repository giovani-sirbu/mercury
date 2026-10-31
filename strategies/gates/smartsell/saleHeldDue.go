package smartsell

// heldTrigger is the trigger the episode's newest held row names; unnamed for a bound's, the trend's or a legacy row.
type heldTrigger struct {
	pct   float64
	named bool
}

// heldTriggerOf is the trigger a held event names (EventData.Trigger), unnamed when it names none.
func heldTriggerOf(data EventData) heldTrigger {
	if data.Trigger == nil {
		return heldTrigger{}
	}
	return heldTrigger{pct: *data.Trigger, named: true}
}

// profitRowDue is whether a held row by the profit is due under pct: none named yet, or the trigger moved.
func (trigger saleTrigger) profitRowDue(pct float64) bool {
	return !trigger.heldUnder.named || trigger.heldUnder.pct != pct
}

// heldRows is the held row, worded by why, of every trigger with no held row in its episode yet.
func heldRows(triggers []saleTrigger, fill float64, why string) []Row {
	var rows []Row
	for _, trigger := range triggers {
		if trigger.rowDue {
			rows = append(rows, saleHeldRow(trigger.gate, fill, why, trigger.features))
		}
	}
	return rows
}

// profitHeldRows is the held row of every trigger the profit falls short of, due under its own trigger (profitRowDue).
func profitHeldRows(triggers []saleTrigger, fill, profitPct float64, shared sharedTrigger) []Row {
	var rows []Row
	for _, trigger := range triggers {
		pct := trigger.terms.triggerPct(shared)
		if trigger.profitRowDue(pct) {
			rows = append(rows, saleHeldByProfitRow(trigger.gate, fill, profitPct, pct, trigger.features))
		}
	}
	return rows
}
