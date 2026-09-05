package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestApplyProtectiveTickUseAIAloneDoesNotSellLoss(t *testing.T) {
	trade := testutil.DeepTrade(false)
	trade.Strategy.Params.UseAI = true
	got := ApplyProtectiveTick(trade, "stopLoss", 97, time.Now(), aggragates.AIIndicators{CrashActive: true}, true)
	if got.Position != "stopLoss" {
		t.Fatalf("UseAI without CrashGuard or SmartTakeLoss must not emit sellLoss, got %+v", got)
	}
}

func TestApplyProtectiveTickCrashGuardDoesNotSellLossWithoutSTL(t *testing.T) {
	trade := testutil.DeepTrade(true)
	got := ApplyProtectiveTick(trade, "stopLoss", 97, time.Now(), aggragates.AIIndicators{CrashActive: true}, true)
	if got.Position != "stopLoss" {
		t.Fatalf("CrashGuard without SmartTakeLoss must not emit sellLoss, got %+v", got)
	}
}

func TestApplyProtectiveTickSmartTakeLossWithoutUseAI(t *testing.T) {
	trade := testutil.DeepLadderTrade(8, false)
	trade.Strategy.Params.UseAI = false
	trade.Strategy.Params.SmartTakeLoss = true
	got := ApplyProtectiveTick(trade, "stopLoss", 90, time.Now(), highRiskAI(false), false)
	if !got.STLForced {
		t.Fatalf("SmartTakeLoss must force without UseAI, got %+v", got)
	}
}
