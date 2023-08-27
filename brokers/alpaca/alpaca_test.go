// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package alpaca

import (
	"context"
	"encoding/json"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

const testFigi = "BBG000B9XRY4"
const testIsin = "US0378331005"
const testSymbol = "AAPL"
const testOrderId = "61e69015-8549-4bfd-b9c3-01e75843f47d"

func TestQueryQuote(t *testing.T) {
	srv := newAlpacaMock()
	defer srv.Close()
	isin := make(chan stockval.AssetData, 1)
	response := make(chan stockapi.QueryQuoteResponse, 1)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newAlpacaConfig(srv.URL))
	assert.NoError(t, err)
	go broker.QueryQuote(context.Background(), isin, response)
	isin <- stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Nil(t, responseData.Error)
	assert.Equal(t, 0, decimal.New(12591, 2).CmpTotal(responseData.CurrentPrice))
	assert.Equal(t, 0, decimal.New(12685, 2).CmpTotal(responseData.PreviousClosePrice))
	assert.Equal(t, 0, decimal.New(-74, 2).CmpTotal(stockval.RoundPercentage(responseData.DeltaPercentage)))
}

func TestQueryCandles(t *testing.T) {
	srv := newAlpacaMock()
	defer srv.Close()
	c := make(chan stockapi.CandlesRequest, 1)
	defer close(c)
	response := make(chan stockapi.QueryCandlesResponse, 1)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newAlpacaConfig(srv.URL))
	assert.NoError(t, err)
	go broker.QueryCandles(context.Background(), c, response)
	c <- stockapi.CandlesRequest{
		Stock:      stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
		Resolution: candles.CandleOneMinute,
		FromTime:   time.Unix(1664712905, 0),
		ToTime:     time.Unix(1664799305, 0),
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Equal(t, candles.CandleOneMinute, responseData.Resolution)
	assert.Nil(t, responseData.Error)
	data := []indapi.CandleData{
		{
			Timestamp:  time.Unix(1649664000, 0).UTC(),
			OpenPrice:  decimal.New(16899, 2),
			HighPrice:  decimal.New(16981, 2),
			LowPrice:   decimal.New(16799, 2),
			ClosePrice: decimal.New(169, 0),
			Volume:     decimal.New(7170, 0),
		},
		{
			Timestamp:  time.Unix(1649750400, 0).UTC(),
			OpenPrice:  decimal.New(17099, 2),
			HighPrice:  decimal.New(17181, 2),
			LowPrice:   decimal.New(16999, 2),
			ClosePrice: decimal.New(171, 0),
			Volume:     decimal.New(7172, 0),
		},
	}
	assert.Len(t, responseData.Data, len(data))
	for i, c := range responseData.Data {
		assert.Equal(t, 0, data[i].ClosePrice.CmpTotal(c.ClosePrice), "close price at index %d invalid", i)
		assert.Equal(t, 0, data[i].HighPrice.CmpTotal(c.HighPrice), "high price at index %d invalid", i)
		assert.Equal(t, 0, data[i].LowPrice.CmpTotal(c.LowPrice), "low price at index %d invalid", i)
		assert.Equal(t, 0, data[i].OpenPrice.CmpTotal(c.OpenPrice), "open price at index %d invalid", i)
		assert.Equal(t, 0, data[i].Volume.CmpTotal(c.Volume), "volume at index %d invalid", i)
		assert.Equal(t, data[i].Timestamp, c.Timestamp)
	}
}

func TestSubscribeData(t *testing.T) {
	srv := newAlpacaWsMock()
	defer srv.Close()
	c := make(chan stockapi.SubscribeDataRequest)
	defer close(c)
	response := make(chan stockapi.SubscribeDataResponse)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newAlpacaConfig(srv.URL))
	assert.NoError(t, err)
	go broker.SubscribeData(context.Background(), c, response)
	c <- stockapi.SubscribeDataRequest{
		Asset: stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
		Type:  stockapi.RealtimeTradesSubscribe,
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Equal(t, stockapi.RealtimeTradesSubscribe, responseData.Type)
	assert.Nil(t, responseData.Error)
}

func TestSubscribeDataError(t *testing.T) {
	srv := newAlpacaWsMock()
	defer srv.Close()
	c := make(chan stockapi.SubscribeDataRequest)
	defer close(c)
	response := make(chan stockapi.SubscribeDataResponse)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newAlpacaConfig(srv.URL))
	assert.NoError(t, err)
	go broker.SubscribeData(context.Background(), c, response)
	c <- stockapi.SubscribeDataRequest{}
	responseData := <-response
	assert.NotNil(t, responseData.Error)
}

func TestSubscribeDataRealtime(t *testing.T) {
	srv := newAlpacaWsMock()
	defer srv.Close()
	c := make(chan stockapi.SubscribeDataRequest)
	defer close(c)
	response := make(chan stockapi.SubscribeDataResponse)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newAlpacaConfig(srv.URL))
	assert.NoError(t, err)
	go broker.SubscribeData(context.Background(), c, response)
	c <- stockapi.SubscribeDataRequest{
		Asset: stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
		Type:  stockapi.RealtimeTradesSubscribe,
	}
	responseData := <-response
	assert.Nil(t, responseData.Error)
	assert.NotNil(t, responseData.TickData)
	tickData := <-responseData.TickData
	assert.NotNil(t, tickData.Price)
	assert.NotNil(t, tickData.Volume)
}

func TestTradeAsset(t *testing.T) {
	srv := newAlpacaMock()
	defer srv.Close()
	c := make(chan stockapi.TradeRequest, 1)
	defer close(c)
	response := make(chan stockapi.TradeResponse, 1)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newAlpacaConfig(srv.URL))
	assert.NoError(t, err)
	go broker.TradeAsset(context.Background(), c, response, true)
	c <- stockapi.TradeRequest{
		RequestId: "Test",
		Asset:     stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
		Quantity:  decimal.New(10, 0),
		Sell:      false,
		Type:      stockapi.OrderTypeMarket,
	}
	responseData := <-response
	assert.NoError(t, responseData.Error)
	assert.Equal(t, "Test", responseData.RequestId)
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Equal(t, testOrderId, responseData.OrderId)
}

func getQuoteResultMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := `{
		"symbol": "` + testSymbol + `",
		"latestTrade": {
		  "t": "2021-05-11T20:00:00.435997104Z",
		  "x": "Q",
		  "p": 125.91,
		  "s": 5589631,
		  "c": ["@", "M"],
		  "i": 179430,
		  "z": "C"
		},
		"latestQuote": {
		  "t": "2021-05-11T22:05:02.307304704Z",
		  "ax": "P",
		  "ap": 125.68,
		  "as": 12,
		  "bx": "P",
		  "bp": 125.6,
		  "bs": 4,
		  "c": ["R"]
		},
		"minuteBar": {
		  "t": "2021-05-11T22:02:00Z",
		  "o": 125.66,
		  "h": 125.66,
		  "l": 125.66,
		  "c": 125.66,
		  "v": 396
		},
		"dailyBar": {
		  "t": "2021-05-11T04:00:00Z",
		  "o": 123.5,
		  "h": 126.27,
		  "l": 122.77,
		  "c": 125.91,
		  "v": 125863164
		},
		"prevDailyBar": {
		  "t": "2021-05-10T04:00:00Z",
		  "o": 129.41,
		  "h": 129.54,
		  "l": 126.81,
		  "c": 126.85,
		  "v": 79569305
		}
	  }`
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func getStockCandleResultMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query()
	pageToken := query.Get("page_token")
	var reply string
	if pageToken == "" {
		reply = `{
			"bars": [
			{
				"t": "2022-04-11T08:00:00Z",
				"o": 168.99,
				"h": 169.81,
				"l": 167.99,
				"c": 169,
				"v": 7170,
				"n": 206,
				"vw": 169.233976
			}
			],
			"symbol": "` + testSymbol + `",
			"next_page_token": "QUFQTHxNfDIwMjItMDQtMTFUMDg6MDA6MDAuMDAwMDAwMDAwWg=="
		}`
	} else {
		reply = `{
			"bars": [
			{
				"t": "2022-04-12T08:00:00Z",
				"o": 170.99,
				"h": 171.81,
				"l": 169.99,
				"c": 171,
				"v": 7172,
				"n": 208,
				"vw": 171.233976
			}
			],
			"symbol": "` + testSymbol + `"
		}`
	}
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func postOrderMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := `{
		"id": "` + testOrderId + `",
		"client_order_id": "eb9e2aaa-f71a-4f51-b5b4-52a6c565dad4",
		"created_at": "2021-03-16T18:38:01.942282Z",
		"updated_at": "2021-03-16T18:38:01.942282Z",
		"submitted_at": "2021-03-16T18:38:01.937734Z",
		"filled_at": null,
		"expired_at": null,
		"canceled_at": null,
		"failed_at": null,
		"replaced_at": null,
		"replaced_by": null,
		"replaces": null,
		"asset_id": "b0b6dd9d-8b9b-48a9-ba46-b9d54906e415",
		"symbol": "` + testSymbol + `",
		"asset_class": "us_equity",
		"notional": "500",
		"qty": null,
		"filled_qty": "0",
		"filled_avg_price": null,
		"order_class": "",
		"order_type": "market",
		"type": "market",
		"side": "buy",
		"time_in_force": "day",
		"limit_price": null,
		"stop_price": null,
		"status": "accepted",
		"extended_hours": false,
		"legs": null,
		"trail_percent": null,
		"trail_price": null,
		"hwm": null
	  }`
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func webSocketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade test http connection to a websocket connection.
	webSocketUpgrader := websocket.Upgrader{}
	conn, err := webSocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	connectedMsg := []realtimeMessage{
		{
			Type: messageTypeSuccess,
			Msg:  messageConnected,
		},
	}
	connMsg, _ := json.Marshal(connectedMsg)
	_ = conn.WriteMessage(int(websocket.TextMessage), connMsg)

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			break // connection was closed
		}
		if messageType != websocket.TextMessage {
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		var cmd realtimeSubscribeCommand
		err = json.Unmarshal(p, &cmd)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		if cmd.Action == "auth" {
			connectedMsg := []realtimeMessage{
				{
					Type: messageTypeSuccess,
					Msg:  messageAuthenticated,
				},
			}
			connMsg, _ := json.Marshal(connectedMsg)
			_ = conn.WriteMessage(int(websocket.TextMessage), connMsg)
		} else if cmd.Action == "subscribe" {
			for _, symbol := range cmd.Trades {
				// send a realtime message as response to a trades subscription request
				data := []realtimeMessage{
					{
						Symbol:    symbol,
						Type:      messageTypeTrade,
						Timestamp: time.Now(),
						Price:     decimal.New(11615, 2),
						TradeSize: decimal.New(54109, 0),
					},
				}
				d, err := json.Marshal(data)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				err = conn.WriteMessage(int(websocket.TextMessage), d)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
	}
}

func newAlpacaMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/stocks/"+testSymbol+"/snapshot", getQuoteResultMock)
	handler.HandleFunc("/stocks/"+testSymbol+"/bars", getStockCandleResultMock)
	handler.HandleFunc("/orders", postOrderMock)

	return httptest.NewServer(handler)
}

func newAlpacaWsMock() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(webSocketHandler))
}

func newAlpacaConfig(dataUrl string) config.Config {
	c := config.NewTestConfig()
	appConfig, _ := c.Lock()
	brokerConfig := appConfig.BrokerConfig[GetBrokerId()]
	brokerConfig.DataUrl = dataUrl
	brokerConfig.WsUrl = "ws" + strings.TrimPrefix(dataUrl, "http")
	brokerConfig.PaperTradingUrl = dataUrl
	appConfig.BrokerConfig[GetBrokerId()] = brokerConfig
	_ = c.Unlock(appConfig, true)
	return c
}
