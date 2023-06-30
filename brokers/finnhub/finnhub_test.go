// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package finnhub

import (
	"context"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/stretchr/testify/assert"
)

const testFigi = "BBG007254H26"
const testIsin = "US0231351067"
const testSymbol = "AMZN"

func TestQueryQuote(t *testing.T) {
	srv := newFinnhubMock()
	defer srv.Close()
	isin := make(chan stockval.AssetData, 1)
	response := make(chan stockapi.QueryQuoteResponse, 1)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newFinnhubConfig(srv.URL))
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
	srv := newFinnhubMock()
	defer srv.Close()
	c := make(chan stockapi.CandlesRequest, 1)
	response := make(chan stockapi.QueryCandlesResponse, 1)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newFinnhubConfig(srv.URL))
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
	srv := newFinnhubMock()
	defer srv.Close()
	c := make(chan stockapi.CandlesRequest, 1)
	response := make(chan stockapi.QueryCandlesResponse, 1)
	broker := NewBroker(nil)
	err := broker.ReadConfig(newFinnhubConfig(srv.URL))
	assert.NoError(t, err)
	go broker.QueryCandles(context.Background(), c, response)
	c <- stockapi.CandlesRequest{
		Stock:      stockval.AssetData{Figi: testFigi, Isin: testIsin, Symbol: testSymbol},
		Resolution: candles.CandleOneMinute,
		FromTime:   time.Unix(1684712905, 0), // use out of range time
		ToTime:     time.Unix(1684799305, 0), // use out of range time
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.Figi)
	assert.Equal(t, candles.CandleOneMinute, responseData.Resolution)
	assert.NotNil(t, responseData.Error)
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

func newFinnhubMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/quote", getQuoteResultMock)
	handler.HandleFunc("/stock/candle", getStockCandleResultMock)

	return httptest.NewServer(handler)
}

func newFinnhubConfig(dataUrl string) config.Config {
	c := config.NewTestConfig()
	appConfig, _ := c.Lock()
	brokerConfig := appConfig.BrokerConfig[GetBrokerId()]
	brokerConfig.DataUrl = dataUrl
	appConfig.BrokerConfig[GetBrokerId()] = brokerConfig
	_ = c.Unlock(appConfig, true)
	return c
}
