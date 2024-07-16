package binanceAdaptor

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

func getUrlByExchange(exchange string, pairs []string) string {
	var modifiedPairs = make([]string, len(pairs))

	switch exchange {
	case "binance":
		{
			for i, pair := range pairs {
				modifiedPairs[i] = fmt.Sprintf("%s@aggTrade", strings.ToLower(pair))
			}
			pairsString := strings.Join(modifiedPairs[:], "/")

			url := fmt.Sprintf("wss://stream.binance.com:443/stream?streams=%s", pairsString)
			return url
		}
	}
	return ""
}

type WSResponse struct {
	Data aggregates.PriceWSResponseData
}

func keepAlive(c *websocket.Conn, timeout time.Duration) {
	lastResponse := time.Now()
	c.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	go func() {
		for {
			err := c.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			if err != nil {
				return
			}
			time.Sleep(timeout / 2)
			if time.Since(lastResponse) > timeout {
				c.Close()
				return
			}
		}
	}()
}

var closedByChannel = false

func listenToMessages(conn *websocket.Conn, done <-chan string) {
	//// Listen for messages
	select {
	case msg := <-done:
		if msg == "close" {
			log.Info("Closed by done channel", "", "") // Log when manager was closed by channel
			closedByChannel = true
			conn.Close()
			return
		}
	}
}

func (e Binance) PriceWSHandler(pairs []string, handler func(aggregates.PriceWSResponseData), done <-chan string) {
	socketUrl := getUrlByExchange(e.Name, pairs)
	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	keepAlive(conn, time.Minute)

	if err != nil {
		log.Info(fmt.Sprintf("Error connecting to Websocket Server: %s", err.Error()), "", "")
		e.PriceWSHandler(pairs, handler, done)
	}
	defer conn.Close()
	defer func() {
		if err := recover(); err != nil {
			e.PriceWSHandler(pairs, handler, done)
		}
	}()

	go listenToMessages(conn, done)

	var response *WSResponse
	for {
		_, msg, err := conn.ReadMessage()

		if err != nil {
			log.Info(fmt.Sprintf("Error in receive: %s", err.Error()), "", "")
			if strings.Contains(err.Error(), "use of closed network connection") && closedByChannel {
				closedByChannel = false
				return
			}
			e.PriceWSHandler(pairs, handler, done)
			return
		}

		if err = json.Unmarshal(msg, &response); err != nil {
			return
		}
		handler(aggregates.PriceWSResponseData{response.Data.Price, response.Data.Symbol})
	}
}
