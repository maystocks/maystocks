// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package finnhub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maystocks/cache"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/calc"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"maystocks/webclient"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zhangyunhao116/skipmap"

	"github.com/ericlagergren/decimal"
)

// We are not using the finnhub apiClient, because it uses float32, which is bad for price calculations.
// We directly unmarshal values into decimal.Big.
type finnhubStockRequester struct {
	// "golang.org/x/time/rate" does not work well, as finnhub resets every 60 seconds.
	rateLimiter           *webclient.RateLimiter
	perSecondRateLimiter  *webclient.RateLimiter
	apiClient             *http.Client
	realtimeConn          *websocket.Conn
	tickDataMap           *skipmap.StringMap[chan stockapi.RealtimeTickData]
	pendingCloseList      []chan stockapi.RealtimeTickData
	pendingCloseListMutex *sync.Mutex
	cache                 *cache.AssetCache
	figiReq               stockapi.SymbolSearchTool
	config                config.BrokerConfig
}

type stockSymbol struct {
	Description    string  `json:"description,omitempty"`
	DisplaySymbol  string  `json:"displaySymbol,omitempty"`
	Symbol         string  `json:"symbol,omitempty"`
	SecurityType   string  `json:"type,omitempty"`
	Mic            string  `json:"mic,omitempty"`
	Figi           string  `json:"figi,omitempty"`
	ShareClassFigi string  `json:"shareClassFIGI,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	Symbol2        string  `json:"symbol2,omitempty"` // "Alternative ticker for exchanges with multiple tickers for 1 stock such as BSE."
	Isin           *string `json:"isin,omitempty"`    // "This field is only available for EU stocks and selected Asian markets. Entitlement from Finnhub is required to access this field."
}

type quote struct {
	O  *decimal.Big `json:"o,omitempty"`
	H  *decimal.Big `json:"h,omitempty"`
	L  *decimal.Big `json:"l,omitempty"`
	C  *decimal.Big `json:"c,omitempty"`
	Pc *decimal.Big `json:"pc,omitempty"`
	D  *decimal.Big `json:"d,omitempty"`
	Dp *decimal.Big `json:"dp,omitempty"`
}

type stockCandles struct {
	O []*decimal.Big `json:"o,omitempty"`
	H []*decimal.Big `json:"h,omitempty"`
	L []*decimal.Big `json:"l,omitempty"`
	C []*decimal.Big `json:"c,omitempty"`
	V []*decimal.Big `json:"v,omitempty"`
	T []int64        `json:"t,omitempty"`
	S string         `json:"s,omitempty"`
}

type realtimeTickEntry struct {
	C *[]string    `json:"c,omitempty"`
	P *decimal.Big `json:"p,omitempty"`
	S string       `json:"s,omitempty"`
	T int64        `json:"t,omitempty"`
	V *decimal.Big `json:"v,omitempty"`
}

type realtimeTickData struct {
	Data []realtimeTickEntry `json:"data,omitempty"`
	Type string              `json:"type,omitempty"`
}

type realtimeCommand struct {
	Type   string `json:"type"`
	Symbol string `json:"symbol"`
}

const MessageTypeTrade = "trade"

func getCandleResolutionStr(r candles.CandleResolution) string {
	switch r {
	case candles.CandleOneMinute:
		return "1"
	case candles.CandleFiveMinutes:
		return "5"
	case candles.CandleFifteenMinutes:
		return "15"
	case candles.CandleThirtyMinutes:
		return "30"
	case candles.CandleSixtyMinutes:
		return "60"
	case candles.CandleOneDay:
		return "D"
	case candles.CandleOneWeek:
		return "W"
	case candles.CandleOneMonth:
		return "M"
	default:
		panic("unsupported candle resolution")
	}
}

func getRealtimeDataSubscriptionStr(s stockval.RealtimeDataSubscription) string {
	switch s {
	case stockval.RealtimeDataSubscribe:
		return "subscribe"
	case stockval.RealtimeDataUnsubscribe:
		return "unsubscribe"
	default:
		panic("unsupported realtime data subscription mode")
	}
}

// Map to trade condition filter.
// Source: Finnhub documentation at https://docs.google.com/spreadsheets/d/1PUxiSWPHSODbaTaoL2Vef6DgU-yFtlRGZf19oBb9Hp0/edit?usp=sharing
var tradeConditionMap = map[string]stockval.TradeContext{
	"1":  stockval.TradeConditionRegular(),
	"2":  stockval.TradeConditionAcquisition(),
	"3":  stockval.TradeConditionAveragePrice(),
	"4":  stockval.TradeConditionBunched(),
	"5":  stockval.TradeConditionCashSale(),
	"6":  stockval.TradeConditionDistribution(),
	"7":  stockval.TradeConditionAutomaticExecution(),
	"8":  stockval.TradeConditionIntermarketSweepOrder(),
	"9":  stockval.TradeConditionBunchedSold(),
	"10": stockval.TradeConditionPriceVariationTrade(),
	"11": stockval.TradeConditionCapElection(),
	"12": stockval.TradeConditionOddLotTrade(),
	"13": stockval.TradeConditionRule127(),
	"14": stockval.TradeConditionRule155(),
	"15": stockval.TradeConditionSoldLast(),
	"16": stockval.TradeConditionMarketCenterOfficialClose(),
	"17": stockval.TradeConditionNextDay(),
	"18": stockval.TradeConditionMarketCenterOpeningTrade(),
	"19": stockval.TradeConditionOpeningPrints(),
	"20": stockval.TradeConditionMarketCenterOfficialOpen(),
	"21": stockval.TradeConditionPriorReferencePrice(),
	"22": stockval.TradeConditionSeller(),
	"23": stockval.TradeConditionSplitTrade(),
	"24": stockval.TradeConditionFormTTrade(),
	"25": stockval.TradeConditionExtendedHoursSoldOutOfSequence(),
	"26": stockval.TradeConditionContingentTrade(),
	"27": stockval.TradeConditionStockOptionTrade(),
	"28": stockval.TradeConditionCrossTrade(),
	"29": stockval.TradeConditionYellowFlag(),
	"30": stockval.TradeConditionSoldOutOfSequence(),
	"31": stockval.TradeConditionStoppedStock(),
	"32": stockval.TradeConditionDerivativelyPriced(),
	"33": stockval.TradeConditionMarketCenterReopeningTrade(),
	"34": stockval.TradeConditionReopeningPrints(),
	"35": stockval.TradeConditionMarketCenterClosingTrade(),
	"36": stockval.TradeConditionClosingPrints(),
	"37": stockval.TradeConditionQualifiedContigentTrade(),
	"38": stockval.TradeConditionPlaceholderFor611Exempt(),
	"39": stockval.TradeConditionCorrectedConsolidatedClose(),
	"40": stockval.TradeConditionOpened(),
	"41": stockval.TradeConditionTradeThroughExempt(),
}

func NewStockRequester(figiReq stockapi.SymbolSearchTool) stockapi.StockValueRequester {
	return &finnhubStockRequester{
		rateLimiter:           webclient.NewRateLimiter(),
		perSecondRateLimiter:  webclient.NewRateLimiter(),
		apiClient:             &http.Client{},
		tickDataMap:           skipmap.NewString[chan stockapi.RealtimeTickData](),
		pendingCloseListMutex: new(sync.Mutex),
		cache:                 cache.NewAssetCache(GetBrokerId()),
		figiReq:               figiReq,
	}
}

func GetBrokerId() stockval.BrokerId {
	return "finnhub"
}

func (requester *finnhubStockRequester) RemainingApiLimit() int {
	return calc.Min(requester.perSecondRateLimiter.Remaining(), requester.rateLimiter.Remaining())
}

func (requester *finnhubStockRequester) createRequest(cmd string) (*http.Request, error) {
	req, err := http.NewRequest("GET", requester.config.DataUrl+cmd, nil)
	if err != nil {
		return req, err
	}
	req.Header.Add("X-Finnhub-Token", requester.config.ApiKey)

	return req, err
}

func (requester *finnhubStockRequester) runRequest(ctx context.Context, cmd string, query url.Values) (*http.Response, error) {
	retry := true
	var resp *http.Response
	for retry {
		// Throttle according to http headers with an additional limit per second.
		err := requester.perSecondRateLimiter.Wait(ctx)
		if err != nil {
			return nil, err
		}
		err = requester.rateLimiter.Wait(ctx)
		if err != nil {
			return nil, err
		}

		req, err := requester.createRequest(cmd)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = query.Encode()

		resp, err = requester.apiClient.Do(req)
		if err != nil {
			return nil, err
		}
		requester.perSecondRateLimiter.HandleManualTimer()
		retry, err = requester.rateLimiter.HandleResponseHeadersWithWait(ctx, resp)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		if retry {
			resp.Body.Close()
		}
	}
	return resp, nil
}

func mapSymbolData(s stockSymbol) stockval.AssetData {
	return stockval.AssetData{
		Figi:                  s.Figi,
		Symbol:                s.Symbol,
		CompanyName:           s.Description,
		Mic:                   s.Mic,
		Currency:              "USD",
		CompanyNameNormalized: stockval.NormalizeAssetName(s.Description),
		Tradable:              false,
	}
}

func (requester *finnhubStockRequester) FindAsset(ctx context.Context, entry <-chan stockapi.SearchRequest, response chan<- stockapi.SearchResponse) {
	defer close(response)

	// Use sync queries when requesting figi (unbuffered channels).
	figiRequestChan := make(chan stockapi.SearchRequest)
	figiResponseChan := make(chan stockapi.SearchResponse)
	defer close(figiRequestChan)
	go requester.figiReq.FindAsset(ctx, figiRequestChan, figiResponseChan)

	symbols := requester.cache.GetAssetList(ctx, func(ctx context.Context) ([]stockval.AssetData, error) {
		query := make(url.Values)
		query.Add("exchange", stockval.DefaultExchange)
		resp, err := requester.runRequest(ctx, "/stock/symbol", query)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var symbols []stockSymbol
		if err = webclient.ParseJsonResponse(resp, &symbols); err != nil {
			return nil, err
		}
		assetData := make([]stockval.AssetData, 0, len(symbols))
		for _, s := range symbols {
			assetData = append(assetData, mapSymbolData(s))
		}
		return assetData, nil
	})

	for entry := range entry {
		if stockval.IsinRegex.MatchString(entry.Text) {
			// finnhub does not provide isin data in its free plan (because isin is kind of commercial).
			// We use openfigi to find data for isin values.
			req := entry
			req.UnambiguousLookup = true
			figiRequestChan <- entry
			figiResponseData := <-figiResponseChan
			if figiResponseData.Error == nil && len(figiResponseData.Result) == 1 {
				// Continue lookup using the symbol.
				entry.Text = figiResponseData.Result[0].Symbol
			}
		}
		responseData := requester.queryAsset(ctx, symbols, entry)
		if responseData.Error != nil {
			log.Print(responseData.Error)
		}
		response <- responseData
	}
}

func (requester *finnhubStockRequester) queryAsset(ctx context.Context, symbols cache.AssetList, entry stockapi.SearchRequest) stockapi.SearchResponse {
	assetList := symbols.Find(entry.Text, entry.MaxNumResults)
	responseData := stockapi.SearchResponse{
		SearchRequest: entry,
		Result:        assetList,
	}
	if entry.UnambiguousLookup && len(assetList) != 1 {
		responseData.Error = errors.New("unambiguous lookup was not successful")
	}
	return responseData
}

func (requester *finnhubStockRequester) QueryQuote(ctx context.Context, entry <-chan stockval.AssetData, response chan<- stockapi.QueryQuoteResponse) {
	defer close(response)

	for entry := range entry {
		resp := requester.querySymbolQuote(ctx, entry)
		if resp.Error != nil {
			log.Print(resp.Error)
		}
		response <- resp
	}
	log.Println("finnhub QueryQuote terminating.")
}

func (requester *finnhubStockRequester) querySymbolQuote(ctx context.Context, entry stockval.AssetData) stockapi.QueryQuoteResponse {
	query := make(url.Values)
	query.Add("symbol", entry.Symbol)
	resp, err := requester.runRequest(ctx, "/quote", query)
	if err != nil {
		return stockapi.QueryQuoteResponse{Figi: entry.Figi, Error: err}
	}
	defer resp.Body.Close()

	var quote quote
	if err = webclient.ParseJsonResponse(resp, &quote); err != nil {
		return stockapi.QueryQuoteResponse{Figi: entry.Figi, Error: err}
	}

	if quote.C == nil || quote.Pc == nil || quote.Dp == nil {
		return stockapi.QueryQuoteResponse{Figi: entry.Figi, Error: errors.New("finnhub quote error: missing data")}
	}

	return stockapi.QueryQuoteResponse{
		Figi:               entry.Figi,
		CurrentPrice:       quote.C,
		PreviousClosePrice: quote.Pc,
		DeltaPercentage:    quote.Dp,
	}
}

func (requester *finnhubStockRequester) QueryCandles(ctx context.Context, request <-chan stockapi.CandlesRequest, response chan<- stockapi.QueryCandlesResponse) {
	defer close(response)

	for req := range request {
		resp := requester.querySymbolCandles(ctx, req.Stock, req.Resolution, req.FromTime, req.ToTime)
		if resp.Error != nil {
			log.Print(resp.Error)
		}
		response <- resp
	}
	log.Println("finnhub QueryCandles terminating.")
}

func (requester *finnhubStockRequester) querySymbolCandles(ctx context.Context, entry stockval.AssetData, resolution candles.CandleResolution,
	fromTime time.Time, toTime time.Time) stockapi.QueryCandlesResponse {
	query := make(url.Values)
	query.Add("symbol", entry.Symbol)
	query.Add("resolution", getCandleResolutionStr(resolution))
	query.Add("from", fmt.Sprint(fromTime.Unix()))
	query.Add("to", fmt.Sprint(toTime.Unix()))
	resp, err := requester.runRequest(ctx, "/stock/candle", query)
	if err != nil {
		return stockapi.QueryCandlesResponse{Figi: entry.Figi, Resolution: resolution, Error: err}
	}
	defer resp.Body.Close()

	var candles stockCandles
	if err = webclient.ParseJsonResponse(resp, &candles); err != nil {
		return stockapi.QueryCandlesResponse{Figi: entry.Figi, Resolution: resolution, Error: err}
	}

	if candles.S != "ok" {
		return stockapi.QueryCandlesResponse{Figi: entry.Figi, Resolution: resolution, Error: fmt.Errorf("finnhub candles error: %s", candles.S)}
	}
	log.Printf("# candles %s: %d", entry.Figi, len(candles.O))

	data := make([]indapi.CandleData, 0, len(candles.T))
	for i := range candles.T {
		data = append(data, indapi.CandleData{
			Timestamp:  time.Unix(candles.T[i], 0),
			OpenPrice:  candles.O[i],
			HighPrice:  candles.H[i],
			LowPrice:   candles.L[i],
			ClosePrice: candles.C[i],
			Volume:     candles.V[i],
		})
	}
	return stockapi.QueryCandlesResponse{
		Figi:       entry.Figi,
		Resolution: resolution,
		Data:       data,
	}
}

func (requester *finnhubStockRequester) initRealtimeConnection(ctx context.Context) {
	if requester.realtimeConn != nil {
		log.Fatal("only a single realtime connection is supported")
	}
	log.Printf("establishing finnhub realtime connection.")
	var err error
	requester.realtimeConn, _, err = websocket.DefaultDialer.DialContext(
		ctx,
		fmt.Sprintf("%s?token=%s", requester.config.WsUrl, requester.config.ApiKey),
		nil)
	if err != nil {
		// TODO this should not be a fatal error
		log.Fatalf("could not connect to finnhub websocket: %v", err)
	}
}

func (requester *finnhubStockRequester) handleRealtimeData() {
	for {
		var data realtimeTickData
		err := requester.realtimeConn.ReadJSON(&data)

		requester.pendingCloseListMutex.Lock()
		for _, c := range requester.pendingCloseList {
			close(c)
		}
		requester.pendingCloseList = nil
		requester.pendingCloseListMutex.Unlock()

		if err != nil {
			requester.tickDataMap.Range(
				func(k string, tickDataEntry chan stockapi.RealtimeTickData) bool {
					close(tickDataEntry)
					return true
				},
			)
			log.Print("finnhub realtime connection was terminated.")
			break
		}
		if data.Type == MessageTypeTrade {
			for _, tickEntry := range data.Data {
				tickChan, exists := requester.tickDataMap.Load(tickEntry.S)
				if exists {
					tradeTime := time.UnixMilli(tickEntry.T)
					// var file, _ = os.OpenFile("/tmp/trades.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
					// file.WriteString(fmt.Sprintf("%s;%f;%f;%v;%v\n", tickEntry.S, tickEntry.P, tickEntry.V, tickEntry.C, tradeTime))
					// file.Close()
					if tradeTime.Before(time.Now().Add(-time.Minute)) {
						log.Printf("Symbol %s: Old realtime data received.", tickEntry.S)
					}
					// Default: Normal trade.
					tradeContext := stockval.NewTradeContext()
					if tickEntry.C != nil {
						for _, c := range *tickEntry.C {
							context, exists := tradeConditionMap[c]
							if exists {
								tradeContext = tradeContext.Combine(context)
							} else {
								log.Printf("Symbol %s: Unknown trade context %s.", tickEntry.S, c)
							}
						}
					}
					tickData := stockapi.RealtimeTickData{
						Timestamp:    tradeTime,
						Price:        tickEntry.P,
						Volume:       tickEntry.V,
						TradeContext: tradeContext,
					}
					select {
					case tickChan <- tickData:
					// usually if a golang channel is full, we would drop additional data.
					// but new data is much more important in this case, so instead we
					// delete old data.
					// we might steal one entry without necessity in some corner cases,
					// but in general this code is fine.
					default:
						select {
						// try to remove first entry, non-blocking
						case <-tickChan:
							// try again to push the new entry, non-blocking
							select {
							case tickChan <- tickData:
								log.Printf("Symbol %s: Buffer overflow. Old realtime data is being removed.", tickEntry.S)
							default:
								log.Printf("Symbol %s: Buffer overflow. New realtime data is being dropped.", tickEntry.S)
							}
						default:
							log.Printf("Symbol %s: Buffer cannot be read from or written to.", tickEntry.S)
						}
					}
				}
			}
		}
	}
}

func (requester *finnhubStockRequester) SubscribeTrades(ctx context.Context, entry <-chan stockapi.SubscribeTradesRequest, response chan<- stockapi.SubscribeTradesResponse) {
	defer close(response)
	for entry := range entry {
		// connect whenever we receive a first subscription message.
		// this avoids establishing a realtime connection to brokers which are not used.
		if requester.realtimeConn == nil {
			requester.initRealtimeConnection(ctx)
			go requester.handleRealtimeData()
		}

		var tickData chan stockapi.RealtimeTickData
		var exists bool
		var err error
		switch entry.Type {
		case stockval.RealtimeDataSubscribe:
			if tickData, exists = requester.tickDataMap.Load(entry.Stock.Symbol); exists {
				err = fmt.Errorf("already subscribed to %s", entry.Stock.Symbol)
			} else {
				// this is required to be a buffered channel, so that it is possible to delete old data in case processing is too slow
				// new tick data is always more important than old data
				tickData = make(chan stockapi.RealtimeTickData, 1024)
				requester.tickDataMap.Store(entry.Stock.Symbol, tickData)
			}
		case stockval.RealtimeDataUnsubscribe:

			if tickData, exists = requester.tickDataMap.LoadAndDelete(entry.Stock.Symbol); exists {
				// we should not close the channel here, because this might cause a race condition.
				requester.pendingCloseListMutex.Lock()
				requester.pendingCloseList = append(requester.pendingCloseList, tickData)
				requester.pendingCloseListMutex.Unlock()
			} else {
				err = fmt.Errorf("cannot unsubscribe %s: not subscribed", entry.Stock.Symbol)
			}
		default:
			panic("unsupported realtime data subscription mode")
		}
		if err == nil {
			subscribeCommand := realtimeCommand{
				Type:   getRealtimeDataSubscriptionStr(entry.Type),
				Symbol: entry.Stock.Symbol,
			}
			msg, _ := json.Marshal(subscribeCommand)
			requester.realtimeConn.WriteMessage(websocket.TextMessage, msg)
		}

		responseData := stockapi.SubscribeTradesResponse{
			Figi:     entry.Stock.Figi,
			Error:    err,
			Type:     entry.Type,
			TickData: tickData,
		}
		response <- responseData
	}
	if requester.realtimeConn != nil {
		requester.realtimeConn.Close()
		requester.realtimeConn = nil
	}
}

func (requester *finnhubStockRequester) ReadConfig(c config.Config) error {
	appConfig, err := c.Copy()
	if err != nil {
		return err
	}
	requester.config = appConfig.BrokerConfig[GetBrokerId()]
	requester.apiClient.Timeout = time.Second * time.Duration(requester.config.DataTimeoutSeconds)
	requester.perSecondRateLimiter = webclient.NewManualRateLimiter(time.Second, uint32(requester.config.RateLimitPerSecond))
	return nil
}

func IsValidConfig(c config.Config) bool {
	appConfig, err := c.Copy()
	if err != nil {
		return false
	}
	finnhubConfig := appConfig.BrokerConfig[GetBrokerId()]
	return len(finnhubConfig.DataUrl) > 0 && len(finnhubConfig.WsUrl) > 0 && len(finnhubConfig.ApiKey) > 0
}
