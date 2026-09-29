package cooldown

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// entrySettingsWallet is the balance the entries of this file are weighed
// against: large enough that a first entry's bid clears the pair's minimum on
// the stored rows and on the entry rows alike, so what the bid costs is the
// rows' doing and never the floor's.
const entrySettingsWallet = 10000

// entryRows is a set of rows an engine can name for a first entry
// (Params.EntrySettings), shaped like the rows a raise names: every stored
// row one depth deeper and a point wider. The stored rows are not written.
func entryRows(stored []aggragates.StrategySettings) []aggragates.StrategySettings {
	rows := append([]aggragates.StrategySettings(nil), stored...)
	for index := range rows {
		rows[index].Depths++
		rows[index].Percentage++
	}

	return rows
}

// nextEntryCostWith is what the managed trade's next entry costs out of the
// wallet on the tick the engine named the given entry rows, read the way the
// gate reads it: through Params.SizingTrade.
func nextEntryCostWith(t *testing.T, trade aggragates.Trades, entry []aggragates.StrategySettings) float64 {
	t.Helper()

	_, cost := ladder.NextEntryCost(aggragates.Params{EntrySettings: entry}.SizingTrade(trade), entrySettingsWallet)
	if cost <= 0 {
		t.Fatalf("%s must cost something to place its next entry", trade.Symbol)
	}

	return cost
}

// entrySettingsReason is the gate's answer for the managed trade on a tick
// whose wallet keeps the given amount for a deeper ladder, with the entry
// rows the engine named, and the ladder it keeps the wallet for.
func entrySettingsReason(trade aggragates.Trades, oldPosition string, reserve float64, entry []aggragates.StrategySettings) (string, aggragates.LadderDepth) {
	keeper := ruleLadder(14, "LINK/USDT", 6, walletDepths, walletAsset, reserve)
	event := priorityEvent(trade, oldPosition, []aggragates.LadderDepth{keeper}, entrySettingsWallet)
	event.Params.EntrySettings = entry

	return DepthPriorityHoldReason(event, "stopLoss"), keeper
}

// A ladder that has not filled yet is weighed at the first entry Buy will
// place, and Buy sizes that entry from the rows the engine named for it. The
// reserve is set exactly where the two sizings part: the wallet covers it
// together with the first entry on the entry rows, and falls short of it with
// the first entry on the stored rows. So the gate lets the ladder through when
// it prices the entry as Buy will, and holds it when no entry rows are named.
func TestDepthPriorityPricesAFirstEntryOnTheEntryRows(t *testing.T) {
	requireDepthPriority(t)

	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)
	entry := entryRows(newcomer.StrategyPair.StrategySettings)

	stored := nextEntryCostWith(t, newcomer, nil)
	sized := nextEntryCostWith(t, newcomer, entry)
	if sized >= stored {
		t.Fatalf("a first entry sized for more depths must cost less, got %f on the entry rows and %f on the stored rows", sized, stored)
	}

	reserve := entrySettingsWallet - sized

	if reason, _ := entrySettingsReason(newcomer, "new", reserve, entry); reason != "" {
		t.Fatalf("a wallet that covers the reserve and the entry Buy will place must let it through, got %q", reason)
	}

	reason, keeper := entrySettingsReason(newcomer, "new", reserve, nil)
	if !strings.Contains(reason, ruleKeeps(keeper)) {
		t.Fatalf("reason = %q, want the first entry priced on the stored rows held for %s", reason, keeper.Symbol)
	}

	// The gate priced a copy: the managed trade keeps its own rows.
	if got := newcomer.StrategyPair.StrategySettings[0].Depths; got != walletDepths {
		t.Fatalf("the managed trade's depths moved to %v", got)
	}
}

// An add is priced on the trade's own rows whatever rows the engine named: a
// ladder with a fill is sized by multiplying its last entry forward, and the
// entry rows size only a first entry. The entry rows here carry a larger
// multiplier too — not something a raise moves, but the one field an add is
// priced from — so an add priced on them would cost something else, and the
// gate is asked at reserves on both sides of where the two would part.
func TestDepthPriorityPricesAnAddOnTheStoredRows(t *testing.T) {
	requireDepthPriority(t)

	own := testutil.LadderDepthTrade(12, "ETH/USDT", 2, walletDepths)
	entry := entryRows(own.StrategyPair.StrategySettings)
	for index := range entry {
		entry[index].Multiplier++
	}

	onEntryRows := own
	onEntryRows.StrategyPair.StrategySettings = entry
	_, misread := ladder.NextEntryCost(onEntryRows, entrySettingsWallet)

	stored := nextEntryCostWith(t, own, nil)
	if misread == stored {
		t.Fatalf("an add priced on the entry rows must cost something else for this case to mean anything, both cost %f", stored)
	}
	if sized := nextEntryCostWith(t, own, entry); sized != stored {
		t.Fatalf("an add is priced on the stored rows, got %f with entry rows named and %f without", sized, stored)
	}

	for name, reserve := range map[string]float64{
		"level with the stored add":        entrySettingsWallet - stored,
		"one unit over the stored add":     entrySettingsWallet - stored + 1,
		"level with an add on entry rows":  entrySettingsWallet - misread,
		"half way between the two sizings": entrySettingsWallet - (stored+misread)/2,
	} {
		named, _ := entrySettingsReason(own, "buy", reserve, entry)
		unnamed, _ := entrySettingsReason(own, "buy", reserve, nil)
		if named != unnamed {
			t.Errorf("%s: reason with entry rows named = %q, without = %q, want the same", name, named, unnamed)
		}
	}
}

// With no entry rows named — nil or empty — the gate prices a first entry on
// the trade's own rows, as it did before an engine could name any: level with
// the reserve and the stored sizing it lets the entry through, and one unit
// short of them it holds.
func TestDepthPriorityWithoutEntryRowsPricesTheStoredRows(t *testing.T) {
	requireDepthPriority(t)

	newcomer := testutil.LadderDepthTrade(21, "ADA/USDT", 0, walletDepths)
	_, stored := ladder.NextEntryCost(newcomer, entrySettingsWallet)

	for name, entry := range map[string][]aggragates.StrategySettings{
		"nil":   nil,
		"empty": {},
	} {
		if reason, _ := entrySettingsReason(newcomer, "new", entrySettingsWallet-stored, entry); reason != "" {
			t.Errorf("%s: a wallet level with the reserve and the stored sizing must let the entry through, got %q", name, reason)
		}

		reason, keeper := entrySettingsReason(newcomer, "new", entrySettingsWallet-stored+1, entry)
		if !strings.Contains(reason, ruleKeeps(keeper)) {
			t.Errorf("%s: reason = %q, want the entry held one unit short of the stored sizing", name, reason)
		}
	}
}
