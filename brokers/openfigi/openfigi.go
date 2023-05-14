// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package openfigi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maystocks/config"
	"maystocks/indapi/calc"
	"maystocks/stockapi"
	"maystocks/stockval"
	"maystocks/webclient"
	"net/http"
	"time"

	"github.com/zhangyunhao116/skipmap"
)

type openFigiRequester struct {
	searchRateLimiter  *webclient.RateLimiter
	mappingRateLimiter *webclient.RateLimiter
	apiClient          *http.Client
	tickerFigiCache    *skipmap.StringMap[FigiData]
	config             config.BrokerConfig
}

type mappingFilters struct {
	ExchangeCode  string `json:"exchCode,omitempty"`
	MicCode       string `json:"micCode,omitempty"`
	Currency      string `json:"currency,omitempty"`
	MarketSector  string `json:"marketSecDes,omitempty"`
	SecurityType  string `json:"securityType,omitempty"`
	SecurityType2 string `json:"securityType2,omitempty"`
}

type mappingRequest struct {
	IdType  string `json:"idType"`
	IdValue string `json:"idValue"`
	mappingFilters
}

type searchRequest struct {
	Query string `json:"query"`
	Start string `json:"start,omitempty"`
	mappingFilters
}

type FigiData struct {
	Figi                string `json:"figi"`
	Name                string `json:"name"`
	Ticker              string `json:"ticker"`
	ExchangeCode        string `json:"exchCode"`
	CompositeFigi       string `json:"compositeFIGI"`
	SecurityType        string `json:"securityType"`
	MarketSector        string `json:"marketSector"`
	ShareClassFigi      string `json:"shareClassFIGI"`
	SecurityType2       string `json:"securityType2"`
	SecurityDescription string `json:"securityDescription"`
	MetaData            string `json:"metadata"`
}

type searchResponse struct {
	Data    []FigiData `json:"data"`
	Error   string     `json:"error,omitempty"`
	Warning string     `json:"warning,omitempty"`
}

type mappingResponse []searchResponse

func NewRequester() stockapi.SymbolSearchTool {
	return &openFigiRequester{
		searchRateLimiter:  webclient.NewRateLimiter(),
		mappingRateLimiter: webclient.NewRateLimiter(),
		apiClient:          http.DefaultClient,
		tickerFigiCache:    skipmap.NewString[FigiData](),
	}
}

func GetBrokerId() stockval.BrokerId {
	return "openfigi"
}

func (requester *openFigiRequester) RemainingApiLimit() int {
	return calc.Min(requester.mappingRateLimiter.Remaining(), requester.searchRateLimiter.Remaining())
}

func (requester *openFigiRequester) createOpenFigiRequest(cmd string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", requester.config.DataUrl+cmd, body)
	if err != nil {
		return req, err
	}
	token := requester.config.ApiKey
	if token != "" {
		req.Header.Add("X-OPENFIGI-APIKEY", token)
	}
	req.Header.Set("Content-Type", "application/json")

	return req, err
}

func mapSymbolData(s FigiData) stockval.AssetData {
	return stockval.AssetData{
		Figi:                  s.Figi,
		Symbol:                s.Ticker,
		CompanyName:           s.Name,
		CompanyNameNormalized: stockval.NormalizeAssetName(s.Name),
		Currency:              "USD",
		Mic:                   s.ExchangeCode,
	}
}

func (requester *openFigiRequester) FindAsset(ctx context.Context, entry <-chan stockapi.SearchRequest, response chan<- stockapi.SearchResponse) {
	defer close(response)

	for entry := range entry {
		responseData := requester.queryFigi(ctx, entry)
		if responseData.Error != nil {
			log.Print(responseData.Error)
		}
		response <- responseData
	}
}

func (requester *openFigiRequester) queryFigi(ctx context.Context, searchData stockapi.SearchRequest) stockapi.SearchResponse {
	searchText := stockval.NormalizeAssetName(searchData.Text)
	mappingFilters := mappingFilters{
		ExchangeCode: stockval.DefaultExchange,
		MarketSector: "Equity",
	}

	var figiData []FigiData
	var err error
	if stockval.IsinRegex.MatchString(searchText) {
		mappingReq := mappingRequest{
			IdType:         "ID_ISIN",
			IdValue:        searchText,
			mappingFilters: mappingFilters,
		}
		figiData, err = requester.executeOpenFigiMappingQuery(ctx, mappingReq)
	} else if searchData.UnambiguousLookup {
		cacheData, ok := requester.tickerFigiCache.Load(searchText)
		if ok {
			figiData = []FigiData{cacheData}
		} else {
			mappingReq := mappingRequest{
				IdType:         "TICKER",
				IdValue:        searchText,
				mappingFilters: mappingFilters,
			}
			figiData, err = requester.executeOpenFigiMappingQuery(ctx, mappingReq)
			if err == nil {
				requester.tickerFigiCache.Store(searchText, figiData[0])
			}
		}
	} else {
		searchReq := searchRequest{
			Query:          searchText,
			mappingFilters: mappingFilters,
		}
		figiData, err = requester.executeOpenFigiSearchQuery(ctx, searchReq)
	}
	if err != nil {
		return stockapi.SearchResponse{SearchRequest: searchData, Error: err}
	}
	result := make([]stockval.AssetData, 0, len(figiData))

	for _, d := range figiData {
		result = append(result, mapSymbolData(d))
	}
	return stockapi.SearchResponse{
		SearchRequest: searchData,
		Result:        result,
	}
}

func (requester *openFigiRequester) executeOpenFigiSearchQuery(ctx context.Context, searchReq searchRequest) ([]FigiData, error) {
	searchJson, err := json.Marshal(searchReq)
	if err != nil {
		return []FigiData{}, err
	}

	retry := true
	var resp *http.Response
	for retry {
		err := requester.searchRateLimiter.Wait(ctx)
		if err != nil {
			return []FigiData{}, err
		}
		req, err := requester.createOpenFigiRequest("/search", bytes.NewBuffer(searchJson))
		if err != nil {
			return []FigiData{}, err
		}

		resp, err := requester.apiClient.Do(req)
		if err != nil {
			return []FigiData{}, err
		}
		retry, err = requester.searchRateLimiter.HandleResponseHeadersWithWait(ctx, resp)
		if err != nil {
			resp.Body.Close()
			return []FigiData{}, err
		}
		if retry {
			resp.Body.Close()
		}
	}
	defer resp.Body.Close()

	var responseData searchResponse
	if err = webclient.ParseJsonResponse(resp, &responseData); err != nil {
		return []FigiData{}, err
	}
	if responseData.Error != "" {
		return []FigiData{}, fmt.Errorf("openFIGI error: %s", responseData.Error)
	}
	if responseData.Warning != "" {
		return []FigiData{}, fmt.Errorf("openFIGI warning: %s", responseData.Warning)
	}

	return responseData.Data, nil
}

func (requester *openFigiRequester) executeOpenFigiMappingQuery(ctx context.Context, mappingReq mappingRequest) ([]FigiData, error) {
	mappingReqList := [1]mappingRequest{
		mappingReq,
	}
	mappingJson, err := json.Marshal(mappingReqList)
	if err != nil {
		return []FigiData{}, err
	}

	retry := true
	var resp *http.Response
	for retry {
		err := requester.mappingRateLimiter.Wait(ctx)
		if err != nil {
			return []FigiData{}, err
		}

		req, err := requester.createOpenFigiRequest("/mapping", bytes.NewBuffer(mappingJson))
		if err != nil {
			return []FigiData{}, err
		}

		resp, err = requester.apiClient.Do(req)
		if err != nil {
			return []FigiData{}, err
		}
		retry, err = requester.mappingRateLimiter.HandleResponseHeadersWithWait(ctx, resp)
		if err != nil {
			resp.Body.Close()
			return []FigiData{}, err
		}
		if retry {
			resp.Body.Close()
		}
	}

	defer resp.Body.Close()
	var responseData mappingResponse
	if err = webclient.ParseJsonResponse(resp, &responseData); err != nil {
		return []FigiData{}, err
	}
	if len(responseData) != 1 {
		return []FigiData{}, errors.New("openFIGI invalid or missing mapping response")
	}
	if responseData[0].Error != "" {
		return []FigiData{}, fmt.Errorf("openFIGI error: %s", responseData[0].Error)
	}
	if responseData[0].Warning != "" {
		return []FigiData{}, fmt.Errorf("openFIGI warning: %s", responseData[0].Warning)
	}

	return responseData[0].Data, nil
}

func (requester *openFigiRequester) ReadConfig(c config.Config) error {
	appConfig, err := c.Copy()
	if err != nil {
		return err
	}
	requester.config = appConfig.BrokerConfig[GetBrokerId()]
	requester.apiClient.Timeout = time.Second * time.Duration(requester.config.DataTimeoutSeconds)
	return nil
}

func IsValidConfig(c config.Config) bool {
	appConfig, err := c.Copy()
	if err != nil {
		return false
	}
	openFigiConfig := appConfig.BrokerConfig[GetBrokerId()]
	return len(openFigiConfig.DataUrl) > 0
}
