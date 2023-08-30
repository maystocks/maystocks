// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package openfigi

import (
	"context"
	"encoding/json"
	"maystocks/mock"
	"maystocks/stockapi"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testFigi = "BBG000BVPV84"
const testIsin = "US0231351067"
const testSymbol = "AMZN"

func TestFindAssetByMapping(t *testing.T) {
	srv := newOpenFigiMock(t)
	logger, _ := mock.NewLogger(t)
	r := make(chan stockapi.SearchRequest, 1)
	defer close(r)
	response := make(chan stockapi.SearchResponse, 1)
	searchTool := NewSearchTool(logger)
	err := searchTool.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go searchTool.FindAsset(context.Background(), r, response)
	r <- stockapi.SearchRequest{
		RequestId:         testFigi,
		Text:              testSymbol,
		MaxNumResults:     100,
		UnambiguousLookup: true,
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.RequestId)
	assert.Nil(t, responseData.Error)
	assert.Equal(t, 1, len(responseData.Result))
}

func TestFindAssetByMappingError(t *testing.T) {
	srv := newOpenFigiMock(t)
	logger, _ := mock.NewLogger(t)
	r := make(chan stockapi.SearchRequest, 1)
	defer close(r)
	response := make(chan stockapi.SearchResponse, 1)
	searchTool := NewSearchTool(logger)
	err := searchTool.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go searchTool.FindAsset(context.Background(), r, response)
	r <- stockapi.SearchRequest{
		RequestId:         testFigi,
		Text:              "INVALID",
		MaxNumResults:     100,
		UnambiguousLookup: true,
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.RequestId)
	assert.NotNil(t, responseData.Error)
	assert.Equal(t, 0, len(responseData.Result))
}

func TestFindAssetBySearch(t *testing.T) {
	srv := newOpenFigiMock(t)
	logger, _ := mock.NewLogger(t)
	r := make(chan stockapi.SearchRequest, 1)
	defer close(r)
	response := make(chan stockapi.SearchResponse, 1)
	searchTool := NewSearchTool(logger)
	err := searchTool.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go searchTool.FindAsset(context.Background(), r, response)
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

func TestFindAssetBySearchError(t *testing.T) {
	srv := newOpenFigiMock(t)
	logger, _ := mock.NewLogger(t)
	r := make(chan stockapi.SearchRequest, 1)
	defer close(r)
	response := make(chan stockapi.SearchResponse, 1)
	searchTool := NewSearchTool(logger)
	err := searchTool.ReadConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.NoError(t, err)
	go searchTool.FindAsset(context.Background(), r, response)
	r <- stockapi.SearchRequest{
		RequestId:         testFigi,
		Text:              "INVALID",
		MaxNumResults:     100,
		UnambiguousLookup: false,
	}
	responseData := <-response
	assert.Equal(t, testFigi, responseData.RequestId)
	assert.Equal(t, 0, len(responseData.Result))
	assert.NotNil(t, responseData.Error)
}

func TestCheckConfig(t *testing.T) {
	srv := newOpenFigiMock(t)
	valid := IsValidConfig(mock.NewBrokerConfig(GetBrokerId(), srv.URL))
	assert.True(t, valid)
}

func getMappingResultMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var reply string
	if err != nil || m != "application/json" {
		reply = `[{
			"error": "Invalid query."
			}]`
	} else {

		var request []mappingRequest
		if err = json.NewDecoder(r.Body).Decode(&request); err != nil || len(request) != 1 || request[0].IdValue != testSymbol {
			reply = `[{
				"error": "No identifier found."
				}]`
		} else {
			reply = `[{
				"data": [{
					"figi": "` + testFigi + `",
					"securityType": "Common Stock",
					"marketSector": "Equity",
					"ticker": "` + testSymbol + `",
					"name": "AMAZON.COM INC",
					"exchCode": "US",
					"shareClassFIGI": "BBG001S5PQL7",
					"compositeFIGI": "BBG000BVPV84",
					"securityType2": "Common Stock",
					"securityDescription": "` + testSymbol + `"
				}]
			}]`
		}
	}
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func getSearchResultMock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	m, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var reply string
	if err != nil || m != "application/json" {
		reply = `[{
			"error": "Invalid query."
			}]`
	} else {

		var request searchRequest
		if err = json.NewDecoder(r.Body).Decode(&request); err != nil || request.Query != testSymbol {
			reply = `[{
				"error": "No identifier found."
				}]`
		} else {
			reply = `{
					"data": [{
						"figi": "` + testFigi + `",
						"name": "AMAZON.COM INC",
						"ticker": "` + testSymbol + `",
						"exchCode": "US",
						"compositeFIGI": "BBG000BVPV84",
						"securityType": "Common Stock",
						"marketSector": "Equity",
						"shareClassFIGI": "BBG001S5PQL7",
						"securityType2": "Common Stock",
						"securityDescription": "` + testSymbol + `"
					}]
				}`
		}
	}
	_, _ = w.Write([]byte(reply)) // ignore errors, test will fail anyway in case Write fails
}

func newOpenFigiMock(t *testing.T) *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/mapping", getMappingResultMock)
	handler.HandleFunc("/search", getSearchResultMock)

	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	return srv
}
