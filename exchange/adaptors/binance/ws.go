package binanceAdaptor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/metrics"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/copier"
)

// wsStalenessSampleInterval is how often the staleness sampler refreshes
// ws_last_message_age_seconds. 5s gives Prometheus (scrape every 15s) at
// least 2 fresh samples per scrape window so a stalled stream is caught
// within ~25s — fast enough that hermes doesn't trade on stale prices.
const wsStalenessSampleInterval = 5 * time.Second

// Backoff bounds for reconnect loops. On every consecutive failure the delay
// doubles, capped at wsReconnectMaxBackoff. A clean disconnect via ctx.Done
// exits immediately without sleeping.
const (
	wsReconnectInitialBackoff = 1 * time.Second
	wsReconnectMaxBackoff     = 30 * time.Second
	wsKeepaliveInterval       = time.Minute
)

// ---- URL helpers ----

func getUrlByExchange(exchange string, pairs []string) string {
	switch exchange {
	case "binance":
		streams := make([]string, 0, len(pairs))
		for _, pair := range pairs {
			streams = append(streams, fmt.Sprintf("%s@aggTrade", strings.ToLower(pair)))
		}
		return fmt.Sprintf("wss://stream.binance.com:443/stream?streams=%s", strings.Join(streams, "/"))
	}
	return ""
}

// ---- Wire types ----

type WSResponse struct {
	Data aggregates.PriceWSResponseData
}

type UserWSResponse struct {
	Data aggregates.WsUserDataEvent
}

// ---- Keepalive ----

// keepAlive sends a ping every wsKeepaliveInterval/2 and closes the connection
// if no pong is received within wsKeepaliveInterval. lastPong is an atomic
// int64 (Unix nanos) because the pong handler runs on a different goroutine
// than the writer.
func keepAlive(ctx context.Context, c *websocket.Conn, timeout time.Duration) {
	var lastPong atomic.Int64
	lastPong.Store(time.Now().UnixNano())

	c.SetPongHandler(func(string) error {
		lastPong.Store(time.Now().UnixNano())
		return nil
	})

	go func() {
		ticker := time.NewTicker(timeout / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.WriteMessage(websocket.PingMessage, []byte("keepalive")); err != nil {
					return
				}
				if time.Since(time.Unix(0, lastPong.Load())) > timeout {
					_ = c.Close()
					return
				}
			}
		}
	}()
}

// ---- Price stream ----

// PriceWSHandler subscribes to aggTrade streams for pairs and delivers events
// to handler until ctx is cancelled. The connection auto-reconnects with an
// exponential backoff; cancelling the context returns nil immediately.
//
// Prior versions used a `done chan string` signal and re-entered themselves
// recursively on error, growing the goroutine stack without bound on flaky
// connections and leaving a shared package-level flag to coordinate shutdown.
// The new shape uses ctx and an ordinary loop, so shutdown is explicit and
// reconnect behaviour is bounded.
func (e Binance) PriceWSHandler(ctx context.Context, pairs []string, handler func(aggregates.PriceWSResponseData)) error {
	socketURL := getUrlByExchange(e.Name, pairs)
	if socketURL == "" {
		return fmt.Errorf("unsupported exchange: %s", e.Name)
	}

	// Shared staleness clock — runPriceStream stores the wall-clock time
	// of the most recently received frame. The sampler goroutine below
	// publishes (now - lastFrame) as ws_last_message_age_seconds so a
	// connected-but-silent stream is detectable from outside.
	var lastFrameNs atomic.Int64
	lastFrameNs.Store(time.Now().UnixNano())

	svc := serviceName()
	go runWSStalenessSampler(ctx, svc, e.Name, "price", &lastFrameNs)

	// Initial state = disconnected; runPriceStream flips to 1 on Dial success.
	metrics.WSConnectionState.WithLabelValues(svc, e.Name, "price").Set(0)

	backoff := wsReconnectInitialBackoff
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		err := e.runPriceStream(ctx, socketURL, handler, &lastFrameNs)
		if err == nil || ctx.Err() != nil {
			return nil
		}

		// Connection ended with an error → classify + count the reconnect.
		// "dial" vs "read" matters: dial failures usually mean upstream is
		// down or network is broken; read failures usually mean Binance
		// closed the socket (e.g. 24h limit, idle timeout).
		reason := "read"
		if strings.Contains(err.Error(), "dial") || strings.Contains(err.Error(), "connect") {
			reason = "dial"
		} else if strings.Contains(err.Error(), "panic") {
			reason = "panic"
		}
		metrics.WSReconnects.WithLabelValues(svc, e.Name, "price", reason).Inc()

		log.Info(fmt.Sprintf("Price WS error: %s (reconnecting in %s)", err.Error(), backoff), "", "PriceWSHandler")
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > wsReconnectMaxBackoff {
			backoff = wsReconnectMaxBackoff
		}
	}
}

// runWSStalenessSampler publishes ws_last_message_age_seconds every
// wsStalenessSampleInterval. Exits when ctx is cancelled. Cheap: one
// gauge Set per tick, no allocs.
func runWSStalenessSampler(ctx context.Context, service, exchange, stream string, lastFrameNs *atomic.Int64) {
	t := time.NewTicker(wsStalenessSampleInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			age := time.Since(time.Unix(0, lastFrameNs.Load())).Seconds()
			metrics.WSLastMessageAge.WithLabelValues(service, exchange, stream).Set(age)
		}
	}
}

// runPriceStream opens a single websocket session and reads until the server
// disconnects or ctx is cancelled. It returns nil on intentional shutdown and
// a non-nil error otherwise so the caller can decide whether to reconnect.
//
// lastFrameNs is updated on every successful frame read so the staleness
// sampler in PriceWSHandler can publish ws_last_message_age_seconds.
func (e Binance) runPriceStream(ctx context.Context, socketURL string, handler func(aggregates.PriceWSResponseData), lastFrameNs *atomic.Int64) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("price stream panic: %v", rec)
		}
	}()

	svc := serviceName()

	conn, _, dialErr := websocket.DefaultDialer.DialContext(ctx, socketURL, nil)
	if dialErr != nil {
		return dialErr
	}
	defer conn.Close()

	// Connection is up — flip the state gauge. Defer the reset so any
	// exit path (clean, error, panic) brings it back to 0.
	metrics.WSConnectionState.WithLabelValues(svc, e.Name, "price").Set(1)
	defer metrics.WSConnectionState.WithLabelValues(svc, e.Name, "price").Set(0)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	keepAlive(streamCtx, conn, wsKeepaliveInterval)

	// Close the connection as soon as ctx is cancelled so ReadMessage unblocks.
	go func() {
		<-streamCtx.Done()
		_ = conn.Close()
	}()

	var response WSResponse
	for {
		_, msg, readErr := conn.ReadMessage()
		if readErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return readErr
		}
		// Frame received — update staleness clock + counter before
		// dispatching so a slow handler doesn't make the stream look stale.
		lastFrameNs.Store(time.Now().UnixNano())
		metrics.WSMessagesReceived.WithLabelValues(svc, e.Name, "price").Inc()

		if jsonErr := json.Unmarshal(msg, &response); jsonErr != nil {
			// Malformed frame — log and continue; no point reconnecting for this.
			log.Error(jsonErr.Error(), "json.Unmarshal", "PriceWSHandler")
			continue
		}
		handler(aggregates.PriceWSResponseData{
			Price:    response.Data.Price,
			Symbol:   response.Data.Symbol,
			Exchange: e.Name,
		})
	}
}

// ---- User stream ----

const expireEvent = "listenKeyExpired"

// UserWs subscribes to user data updates for listenKey. It blocks until ctx
// is cancelled or WsUserDataServe reports the listen key expired; in the
// latter case the caller is expected to fetch a new key and re-subscribe.
func (e Binance) UserWs(ctx context.Context, listenKey string, handler func(order aggregates.WsUserDataEvent, expireEvent string)) error {
	wsHandler := func(message *binance.WsUserDataEvent) {
		var orderDetails aggregates.WsUserDataEvent
		copier.Copy(&orderDetails, &message)
		copier.Copy(&orderDetails.WsOrderUpdate, &message.OrderUpdate)
		copier.Copy(&orderDetails.WsBalanceUpdate, &message.BalanceUpdate)
		copier.Copy(&orderDetails.WsAccountUpdateList, &message.AccountUpdate)
		copier.Copy(&orderDetails.WsOCOUpdate, &message.OCOUpdate)
		handler(orderDetails, expireEvent)
	}
	errHandler := func(err error) {
		log.Error(err.Error(), "WsUserDataServe", "UserWs")
	}

	doneC, stopC, err := binance.WsUserDataServe(listenKey, wsHandler, errHandler)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		close(stopC)
		<-doneC
		return nil
	case <-doneC:
		return nil
	}
}
