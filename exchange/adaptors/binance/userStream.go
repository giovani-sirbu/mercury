package binanceAdaptor

import (
	"context"

	"github.com/adshao/go-binance/v2/common"
)

// StartUserStream creates a user-data stream and returns its listen key, kept alive by PingUserStream.
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

// PingUserStream extends a listen key's TTL.
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
