package crashguard

import (
	"sync"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
)

// capitulationLive is the per-trade in-process bookkeeping of an episode:
// whether a flush was seen during it and how many quiet 15m windows have
// passed since the last adverse shock.
type capitulationLive struct {
	hadCrash   bool
	quietCount int
	lastWindow int64
}

var (
	liveMu       sync.Mutex
	liveEpisodes = map[uint]capitulationLive{}
)

func freezeClearReason(event events.Events, ai aggragates.AIIndicators, ep capitulationEpisode) (string, bool) {
	adverse := regime.ShockBlocks(ai.Regimes[regime.ShockHoldTimeframe], event.Trade.Inverse)
	if ep.hadCrash {
		if !ai.CrashActive && !adverse {
			return "cleared", true
		}
		return "", false
	}
	if adverse {
		resetQuietWindows(event.Trade.ID)
		return "", false
	}
	if noteQuietWindow(event) >= CapitulationClearQuietWindows {
		return "quiet-15m", true
	}
	return "", false
}

func markCapitulationCrash(tradeID uint) {
	if tradeID == 0 {
		return
	}
	liveMu.Lock()
	state := liveEpisodes[tradeID]
	state.hadCrash = true
	liveEpisodes[tradeID] = state
	liveMu.Unlock()
}

func liveHadCrash(tradeID uint) bool {
	if tradeID == 0 {
		return false
	}
	liveMu.Lock()
	defer liveMu.Unlock()
	return liveEpisodes[tradeID].hadCrash
}

func clearCapitulationLive(tradeID uint) {
	if tradeID == 0 {
		return
	}
	liveMu.Lock()
	delete(liveEpisodes, tradeID)
	liveMu.Unlock()
}

func resetQuietWindows(tradeID uint) {
	if tradeID == 0 {
		return
	}
	liveMu.Lock()
	state := liveEpisodes[tradeID]
	state.quietCount = 0
	state.lastWindow = 0
	liveEpisodes[tradeID] = state
	liveMu.Unlock()
}

func noteQuietWindow(event events.Events) int {
	tradeID := event.Trade.ID
	window := helpers.FloorMillis(event.TickMillis(), 15)
	if tradeID == 0 || window == 0 {
		return 0
	}
	liveMu.Lock()
	defer liveMu.Unlock()
	state := liveEpisodes[tradeID]
	if state.lastWindow == window {
		return state.quietCount
	}
	state.lastWindow = window
	state.quietCount++
	liveEpisodes[tradeID] = state
	return state.quietCount
}
