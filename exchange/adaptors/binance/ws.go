package binanceAdaptor

import (
	"encoding/json"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/gorilla/websocket"
	"github.com/jinzhu/copier"
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

type UserWSResponse struct {
	Data aggregates.WsUserDataEvent
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

func (e Binance) WS(url string, done <-chan string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	keepAlive(conn, time.Minute)

	go listenToMessages(conn, done)

	return conn, err
}

const expireEvent = "listenKeyExpired"

func (e Binance) UserWs(listenKey string, handler func(order aggregates.WsUserDataEvent, expireEvent string), done <-chan string) {
	wsHandler := func(message *binance.WsUserDataEvent) {
		var orderDetails aggregates.WsUserDataEvent
		err := copier.Copy(&orderDetails, &message)
		fmt.Println(err)
		fmt.Println(orderDetails, message, &message)
		handler(orderDetails, expireEvent)
	}
	errHandler := func(err error) {
		fmt.Println(err)
	}
	doneC, _, err := binance.WsUserDataServe(listenKey, wsHandler, errHandler)
	if err != nil {
		fmt.Println(err)
		return
	}
	<-doneC
}

func (e Binance) PriceWSHandler(pairs []string, handler func(aggregates.PriceWSResponseData), done <-chan string) {
	socketUrl := getUrlByExchange(e.Name, pairs)

	conn, err := e.WS(socketUrl, done)

	if err != nil {
		log.Info(fmt.Sprintf("Error connecting to Websocket Server: %s", err.Error()), "", "")
		e.PriceWSHandler(pairs, handler, done)
	}
	defer conn.Close()
	defer func() {
		if recoverErr := recover(); recoverErr != nil {
			e.PriceWSHandler(pairs, handler, done)
		}
	}()

	var response *WSResponse
	for {
		_, msg, connErr := conn.ReadMessage()

		if connErr != nil {
			log.Info(fmt.Sprintf("Error in receive: %s", connErr.Error()), "", "")
			if strings.Contains(connErr.Error(), "use of closed network connection") && closedByChannel {
				closedByChannel = false
				return
			}
			e.PriceWSHandler(pairs, handler, done)
			return
		}

		if err = json.Unmarshal(msg, &response); err != nil {
			return
		}
		handler(aggregates.PriceWSResponseData{Price: response.Data.Price, Symbol: response.Data.Symbol, Exchange: e.Name})
	}
}
