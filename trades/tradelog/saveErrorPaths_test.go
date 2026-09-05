package tradelog

import (
	"errors"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func nopUpdateTradeForSaveError(event events.Events) (events.Events, error) {
	return event, nil
}

// TestSaveError_APIErrorBlocksTradeAndLogsAsError pins the branch in
// SaveError that flips Trade.Status to Blocked and tags the log entry as
// LOG_ERROR when the inbound error message starts with "<APIError>".
func TestSaveError_APIErrorBlocksTradeAndLogsAsError(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade:  aggragates.Trades{PositionType: "buy", PositionPrice: 100},
		Params: aggragates.Params{OldPosition: "active", OldPositionPrice: 99},
	}

	got, err := SaveError(event, errors.New("<APIError> code=-2010, msg=insufficient balance"))
	if err == nil {
		t.Fatal("expected error to be returned unchanged")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("Trade.Status = %q, want blocked", got.Trade.Status)
	}
	if len(got.Trade.Logs) != 1 {
		t.Fatalf("expected one log appended, got %d", len(got.Trade.Logs))
	}
	if got.Trade.Logs[0].Type != aggragates.LOG_ERROR {
		t.Errorf("log type = %q, want ERROR", got.Trade.Logs[0].Type)
	}
}

// TestSaveError_InsufficientFundsBlocksTradeAndLogsAsError mirrors the
// APIError test for the insufficient-funds branch.
func TestSaveError_InsufficientFundsBlocksTradeAndLogsAsError(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade:  aggragates.Trades{PositionType: "buy"},
		Params: aggragates.Params{OldPosition: "active"},
	}

	got, err := SaveError(event, errors.New("Insufficient funds (5 USDC) for the requested action"))
	if err == nil {
		t.Fatal("expected error returned")
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("Trade.Status = %q, want blocked", got.Trade.Status)
	}
	if got.Trade.Logs[0].Type != aggragates.LOG_ERROR {
		t.Errorf("log type = %q, want ERROR", got.Trade.Logs[0].Type)
	}
}

// TestSaveError_GenericErrorKeepsStatusAndLogsAsWarning verifies the
// default branch — anything that's neither <APIError> nor insufficient
// funds is recorded as LOG_WARNING and Status is left untouched.
func TestSaveError_GenericErrorKeepsStatusAndLogsAsWarning(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade:  aggragates.Trades{PositionType: "buy", Status: aggragates.Active},
		Params: aggragates.Params{OldPosition: "active"},
	}

	got, err := SaveError(event, errors.New("price moved before order placement"))
	if err == nil {
		t.Fatal("expected error returned")
	}
	if got.Trade.Status != aggragates.Active {
		t.Errorf("Trade.Status = %q, want active (unchanged)", got.Trade.Status)
	}
	if got.Trade.Logs[0].Type != aggragates.LOG_WARNING {
		t.Errorf("log type = %q, want WARNING", got.Trade.Logs[0].Type)
	}
}

// TestSaveError_DuplicateErrorShortCircuitsWithoutAppending pins the
// dedupe path: if the last log message matches (modulo digits) the new
// error, the function returns without appending a second log.
func TestSaveError_DuplicateErrorShortCircuitsWithoutAppending(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade: aggragates.Trades{
			PositionType: "buy",
			Logs: []aggragates.TradesLogs{
				{Message: "price moved before order placement"},
			},
		},
		Params: aggragates.Params{OldPosition: "active"},
	}

	got, err := SaveError(event, errors.New("price moved before order placement"))
	if err == nil {
		t.Fatal("expected error to still be returned on dedupe path")
	}
	if len(got.Trade.Logs) != 1 {
		t.Errorf("expected dedupe to skip the new log, got %d entries", len(got.Trade.Logs))
	}
}

// TestSaveError_RevertsPositionToOldWhenNotImpasse verifies the rollback:
// when PositionType is anything other than "impasse", SaveError resets
// PositionType and PositionPrice back to Params.OldPosition / OldPrice so
// the next tick doesn't try to act on the failed-attempt state.
func TestSaveError_RevertsPositionToOldWhenNotImpasse(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade: aggragates.Trades{
			PositionType:  "takeProfit",
			PositionPrice: 101000,
		},
		Params: aggragates.Params{
			OldPosition:      "active",
			OldPositionPrice: 100500,
		},
	}

	got, _ := SaveError(event, errors.New("some generic error"))
	if got.Trade.PositionType != "active" {
		t.Errorf("PositionType = %q, want active (reverted)", got.Trade.PositionType)
	}
	if got.Trade.PositionPrice != 100500 {
		t.Errorf("PositionPrice = %v, want 100500 (reverted)", got.Trade.PositionPrice)
	}
}

// TestSaveError_PreservesImpasseAcrossSave pins the negative of the prior
// test: when PositionType is "impasse", the revert is skipped so the
// engine can proceed with impasse-children creation on the next tick.
func TestSaveError_PreservesImpasseAcrossSave(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade: aggragates.Trades{
			PositionType:  "impasse",
			PositionPrice: 95000,
		},
		Params: aggragates.Params{
			OldPosition:      "buy",
			OldPositionPrice: 100000,
		},
	}

	got, _ := SaveError(event, errors.New("Insufficient funds for impasse trigger"))
	if got.Trade.PositionType != "impasse" {
		t.Errorf("expected PositionType preserved as impasse, got %q", got.Trade.PositionType)
	}
	if got.Trade.PositionPrice != 95000 {
		t.Errorf("expected PositionPrice preserved at 95000, got %v", got.Trade.PositionPrice)
	}
}

// TestSaveError_SetsPreventInfoLogFlag asserts the side effect that
// downstream UpdateTrade uses to skip emitting a duplicate INFO log on
// the same tick.
func TestSaveError_SetsPreventInfoLogFlag(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade:  aggragates.Trades{PositionType: "buy"},
		Params: aggragates.Params{OldPosition: "active"},
	}

	got, _ := SaveError(event, errors.New("anything"))
	if !got.Params.PreventInfoLog {
		t.Error("expected PreventInfoLog flag set after SaveError")
	}
}
