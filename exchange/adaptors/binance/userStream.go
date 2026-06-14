package binanceAdaptor

import (
	"context"

	"github.com/adshao/go-binance/v2/common"
)

// StartUserStream creates a new user-data stream and returns its listen key.
// The key is valid for 60 minutes; callers must PingUserStream every 30
// minutes to keep it alive, otherwise WsUserDataServe emits listenKeyExpired
// and UserWs returns.
func (e Binance) StartUserStream() (string, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return "", initErr
	}
	clientInfo, err := client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		return clientInfo, ApiError(err)
	}
	return clientInfo, nil
}

// PingUserStream extends the TTL of a listen key. Safe to call at any point
// — binance is idempotent over the operation.
func (e Binance) PingUserStream(listenKey string) *common.APIError {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return initErr
	}
	err := client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background())
	if err != nil {
		return ApiError(err)
	}
	return nil
}
