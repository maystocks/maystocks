// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package finnhub

import (
	"context"
	"encoding/json"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/mock"
	"maystocks/stockapi"
	"maystocks/stockval"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

const testFigi = "BBG000BVPV84"
const testIsin = "US0231351067"
const testSymbol = "AMZN"

func TestQueryQuote(t *testing.T) {
	srv := newFinnhubMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	isin := make(chan stockval.AssetData, 1)
	response := make(chan stockapi.QueryQuoteResponse, 1)
	broker := NewBroker(nil, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go broker.QueryQuote(context.Background(), isin, response)
	isin <- stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Nil(t, responseData.Error)
	assert.Equal(t, 0, decimal.New(11615, 2).CmpTotal(responseData.CurrentPrice))
	assert.Equal(t, 0, decimal.New(11478, 2).CmpTotal(responseData.PreviousClosePrice))
	assert.Equal(t, 0, decimal.New(12141, 4).CmpTotal(responseData.DeltaPercentage))
}

func TestQueryCandles(t *testing.T) {
	srv := newFinnhubMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	c := make(chan stockapi.CandlesRequest, 1)
	response := make(chan stockapi.QueryCandlesResponse, 1)
	broker := NewBroker(nil, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go broker.QueryCandles(context.Background(), c, response)
	c <- stockapi.CandlesRequest{
		Asset:      stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
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
			Timestamp:  time.Unix(1664784000, 0),
			OpenPrice:  decimal.New(112, 0),
			HighPrice:  decimal.New(112, 0),
			LowPrice:   decimal.New(111, 0),
			ClosePrice: decimal.New(11112, 2),
			Volume:     decimal.New(33109, 0),
		},
		{
			Timestamp:  time.Unix(1664787600, 0),
			OpenPrice:  decimal.New(11103, 2),
			HighPrice:  decimal.New(1115, 1),
			LowPrice:   decimal.New(11078, 2),
			ClosePrice: decimal.New(11126, 2),
			Volume:     decimal.New(21942, 0),
		},
		{
			Timestamp:  time.Unix(1664791200, 0),
			OpenPrice:  decimal.New(11149, 2),
			HighPrice:  decimal.New(11198, 2),
			LowPrice:   decimal.New(11144, 2),
			ClosePrice: decimal.New(11195, 2),
			Volume:     decimal.New(24349, 0),
		},
		{
			Timestamp:  time.Unix(1664794800, 0),
			OpenPrice:  decimal.New(11198, 2),
			HighPrice:  decimal.New(11244, 2),
			LowPrice:   decimal.New(11184, 2),
			ClosePrice: decimal.New(1122, 1),
			Volume:     decimal.New(77377, 0),
		},
		{
			Timestamp:  time.Unix(1664798400, 0),
			OpenPrice:  decimal.New(1121996, 4),
			HighPrice:  decimal.New(113, 0),
			LowPrice:   decimal.New(111718, 3),
			ClosePrice: decimal.New(11263, 2),
			Volume:     decimal.New(155176, 0),
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

func TestQueryCandlesError(t *testing.T) {
	srv := newFinnhubMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	c := make(chan stockapi.CandlesRequest, 1)
	response := make(chan stockapi.QueryCandlesResponse, 1)
	broker := NewBroker(nil, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go broker.QueryCandles(context.Background(), c, response)
	c <- stockapi.CandlesRequest{
		Asset:      stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
		Resolution: candles.CandleOneMinute,
		FromTime:   time.Unix(1684712905, 0), // use out of range time
		ToTime:     time.Unix(1684799305, 0), // use out of range time
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Equal(t, candles.CandleOneMinute, responseData.Resolution)
	assert.NotNil(t, responseData.Error)
}

func TestSubscribeData(t *testing.T) {
	srv := newFinnhubWsMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	c := make(chan stockapi.SubscribeDataRequest)
	defer close(c)
	response := make(chan stockapi.SubscribeDataResponse)
	broker := NewBroker(nil, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
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
	srv := newFinnhubWsMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	c := make(chan stockapi.SubscribeDataRequest)
	defer close(c)
	response := make(chan stockapi.SubscribeDataResponse)
	broker := NewBroker(nil, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go broker.SubscribeData(context.Background(), c, response)
	c <- stockapi.SubscribeDataRequest{}
	responseData := <-response
	assert.NotNil(t, responseData.Error)
}

func TestSubscribeDataRealtime(t *testing.T) {
	srv := newFinnhubWsMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	c := make(chan stockapi.SubscribeDataRequest)
	defer close(c)
	response := make(chan stockapi.SubscribeDataResponse)
	broker := NewBroker(nil, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
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

func TestFindAsset(t *testing.T) {
	srv := newFinnhubMock(t)
	cache := mock.NewAssetCache(t)
	logger, _ := mock.NewLogger(t)
	searchTool := mock.NewSearchTool()
	r := make(chan stockapi.SearchRequest, 1)
	defer close(r)
	response := make(chan stockapi.SearchResponse, 1)
	broker := NewBroker(searchTool, cache, logger)
	err := broker.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go broker.FindAsset(context.Background(), r, response)
	r <- stockapi.SearchRequest{
		RequestId:         testFigi,
		Text:              testSymbol,
		MaxNumResults:     100,
		UnambiguousLookup: false,
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.RequestId)
	assert.Nil(t, responseData.Error)
	assert.Equal(t, 1, len(responseData.Result))
}

func getQuoteResultMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := `{
		"c": 116.15,
		"d": 1.37,
		"dp": 1.2141,
		"h": 117.335,
		"l": 113.13,
		"o": 113.295,
		"pc": 114.78,
		"t": 1664222404
	  }`
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func getStockCandleResultMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query()
	from, _ := strconv.ParseInt(query["from"][0], 10, 64)
	to, _ := strconv.ParseInt(query["to"][0], 10, 64)
	reply := `{"s": "no_data"}`
	// simplified: use valid reply only for certain requests
	if from <= 1664784000 && to >= 1664798400 {
		reply = `{
			"c": [111.12,111.26,111.95,112.2,112.63],
			"h": [112,111.5,111.98,112.44,113],
			"l": [111,110.78,111.44,111.84,111.718],
			"o": [112,111.03,111.49,111.98,112.1996],
			"s": "ok",
			"t": [1664784000,1664787600,1664791200,1664794800,1664798400],
			"v": [33109,21942,24349,77377,155176]
		}`
	}
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func getStockSymbolsMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := `[
	{
		  "currency": "USD",
		  "description": "AMAZON.COM INC.",
		  "displaySymbol": "` + testSymbol + `",
		  "figi": "` + testFigi + `",
		  "mic": "XNGS",
		  "symbol": "` + testSymbol + `",
		  "type": "Common Stock"
		}
	  ]`
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func getCryptoSymbolsMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := `[
	{
		  "description": "Binance ETHBTC",
		  "displaySymbol": "ETH/BTC",
		  "symbol": "ETHBTC"
		}
	  ]`

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

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			break // connection was closed
		}
		if messageType != websocket.TextMessage {
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		var cmd realtimeCommand
		err = json.Unmarshal(p, &cmd)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			break
		}
		if cmd.Type == "subscribe" {
			// send a realtime tick as response to a subscription request
			data := realtimeTickData{
				Data: []realtimeTickEntry{{S: cmd.Symbol, C: &[]string{"1"}, P: decimal.New(11615, 2), T: time.Now().UnixMilli(), V: decimal.New(54109, 0)}},
				Type: messageTypeTrade,
			}
			d, err := json.Marshal(data)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				break
			}
			err = conn.WriteMessage(int(websocket.TextMessage), d)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				break
			}
		}
	}
}

func newFinnhubMock(t *testing.T) *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/quote", getQuoteResultMock)
	handler.HandleFunc("/stock/candle", getStockCandleResultMock)
	handler.HandleFunc("/stock/symbol", getStockSymbolsMock)
	handler.HandleFunc("/crypto/symbol", getCryptoSymbolsMock)

	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	return srv
}

func newFinnhubWsMock(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(webSocketHandler))
	t.Cleanup(func() { srv.Close() })
	return srv
}
