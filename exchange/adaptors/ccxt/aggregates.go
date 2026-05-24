// Package ccxt is mercury's adaptor for the official CCXT Go library
// (github.com/ccxt/ccxt/go/v4). It builds the `aggregates.Actions` and
// `aggregates.FuturesActions` function-bags that mercury's trade engine
// consumes, but delegates every REST call to CCXT instead of the
// per-exchange adshao/go-binance library.
//
// Why CCXT: multi-exchange (Binance + Crypto.com + 100+ others) via a single
// unified API, decoupling mercury from go-binance drift (e.g. Binance
// deprecating /api/v3/userDataStream in 2025). The function-bag pattern is
// preserved so hermes, agora, sisyphus, and the rest of mercury's actions
// keep talking to the same Actions surface they always have.
//
// WebSocket scope: REST only in this iteration. The PriceWSHandler and
// UserWSHandler fields on Actions are intentionally left for the binance
// adaptor to populate — CCXT Go v4 has no WebSocket streaming support yet,
// and Binance's new WS API (session.logon + Ed25519) lives in a separate
// follow-up PR.
package ccxt

// Supported exchange IDs. The mercury `Exchange.Name` field carries the
// platform's own naming ("binance", "cryptocom") and we translate to CCXT's
// canonical ID at the boundary so business code never deals with CCXT-specific
// strings. Keep this list in lockstep with the cases in `client.go::newClient`.
const (
	ExchangeBinance   = "binance"
	ExchangeCryptocom = "cryptocom"
)
