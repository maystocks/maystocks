// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package alpaca

import (
	"context"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/stretchr/testify/assert"
)

const testFigi = "BBG000B9XRY4"
const testIsin = "US0378331005"
const testSymbol = "AAPL"

func TestQueryQuote(t *testing.T) {
	srv := newAlpacaMock()
	defer srv.Close()
	isin := make(chan stockval.AssetData, 1)
	response := make(chan stockapi.QueryQuoteResponse, 1)
	requester := NewStockRequester(nil)
	requester.ReadConfig(newAlpacaConfig(srv.URL, srv.URL))
	go requester.QueryQuote(context.Background(), isin, response)
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
	response := make(chan stockapi.QueryCandlesResponse, 1)
	requester := NewStockRequester(nil)
	requester.ReadConfig(newAlpacaConfig(srv.URL, srv.URL))
	go requester.QueryCandles(context.Background(), c, response)
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
	w.Write([]byte(reply))
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
			"symbol": "AAPL"
		}`
	}
	w.Write([]byte(reply))
}

func newAlpacaMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/stocks/"+testSymbol+"/snapshot", getQuoteResultMock)
	handler.HandleFunc("/stocks/"+testSymbol+"/bars", getStockCandleResultMock)

	return httptest.NewServer(handler)
}

func newAlpacaConfig(dataUrl string, tradingUrl string) config.Config {
	c := config.NewTestConfig()
	appConfig, _ := c.Lock()
	brokerConfig := appConfig.BrokerConfig[GetBrokerId()]
	brokerConfig.DataUrl = dataUrl
	brokerConfig.PaperTradingUrl = tradingUrl
	appConfig.BrokerConfig[GetBrokerId()] = brokerConfig
	c.Unlock(appConfig)
	return c
}
