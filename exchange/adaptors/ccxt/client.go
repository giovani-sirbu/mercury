package ccxt

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// newClient constructs a CCXT exchange instance for the given mercury
// Exchange. Routing by `e.Name` lets the same factory handle every supported
// exchange — Binance today, Crypto.com next, anything CCXT covers in the
// future with one extra case.
//
// SandboxMode is honoured via the canonical CCXT method (`SetSandboxMode`)
// so test_net=true on the platform routes orders to the exchange's sandbox
// instead of mainnet — same semantics as the binance adaptor's
// `binance.UseTestnet = true` flip, but cleanly scoped to the constructed
// instance instead of a package-level global.
//
// The returned value is the concrete CCXT type so callers can use methods
// that aren't on the `ccxt.IExchange` interface (e.g. exchange-specific
// helpers). Spot-only for now — futures use a separate constructor that picks
// the right CCXT exchange ID (binanceusdm, etc.).
func newClient(e aggregates.Exchange) (ccxt.IExchange, error) {
	config := map[string]any{
		"apiKey":          e.ApiKey,
		"secret":          e.ApiSecret,
		"enableRateLimit": true,
	}

	var client ccxt.IExchange
	switch e.Name {
	case ExchangeBinance:
		client = ccxt.NewBinance(config)
	case ExchangeCryptocom:
		client = ccxt.NewCryptocom(config)
	default:
		return nil, fmt.Errorf("ccxt adaptor: unsupported exchange %q", e.Name)
	}

	if e.TestNet {
		client.SetSandboxMode(true)
	}
	return client, nil
}

// newFuturesClient returns a CCXT instance configured for the futures
// counterpart of the given exchange. Today only Binance has a futures
// integration in the platform; Crypto.com futures is out of scope until we
// productise a Crypto.com user.
func newFuturesClient(e aggregates.Exchange) (ccxt.IExchange, error) {
	config := map[string]any{
		"apiKey":          e.ApiKey,
		"secret":          e.ApiSecret,
		"enableRateLimit": true,
		"options": map[string]any{
			"defaultType": "future",
		},
	}

	var client ccxt.IExchange
	switch e.Name {
	case ExchangeBinance:
		client = ccxt.NewBinanceusdm(config)
	default:
		return nil, fmt.Errorf("ccxt adaptor: futures not supported for exchange %q", e.Name)
	}

	if e.TestNet {
		client.SetSandboxMode(true)
	}
	return client, nil
}
