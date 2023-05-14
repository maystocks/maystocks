// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package alpaca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maystocks/cache"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockval"
	"maystocks/webclient"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/gorilla/websocket"
	"github.com/zhangyunhao116/skipmap"
)

// We are not using the official alpaca client SDK, because it uses float64.
// While float64 is better than the float32 used by finnhub, it is still bad for price values.
// We directly unmarshal values into decimal.Big.
type alpacaStockRequester struct {
	// "golang.org/x/time/rate" does not work well, as alpaca resets every 60 seconds.
	rateLimiter           *webclient.RateLimiter
	apiClient             *http.Client
	realtimeConn          *websocket.Conn
	tickDataMap           *skipmap.StringMap[chan stockapi.RealtimeTickData]
	pendingCloseList      []chan stockapi.RealtimeTickData
	pendingCloseListMutex *sync.Mutex
	cache                 *cache.AssetCache
	figiReq               stockapi.SymbolSearchTool
	config                config.BrokerConfig
}

type trade struct {
	Timestamp  time.Time    `json:"t"`
	Price      *decimal.Big `json:"p"`
	Size       uint32       `json:"s"`
	Exchange   string       `json:"x"`
	ID         int64        `json:"i"`
	Conditions []string     `json:"c"`
	Tape       string       `json:"z"`
	Update     string       `json:"u"`
}

type quote struct {
	Timestamp   time.Time    `json:"t"`
	BidPrice    *decimal.Big `json:"bp"`
	BidSize     uint32       `json:"bs"`
	BidExchange string       `json:"bx"`
	AskPrice    *decimal.Big `json:"ap"`
	AskSize     uint32       `json:"as"`
	AskExchange string       `json:"ax"`
	Conditions  []string     `json:"c"`
	Tape        string       `json:"z"`
}

type bar struct {
	Timestamp  time.Time    `json:"t"`
	Open       *decimal.Big `json:"o"`
	High       *decimal.Big `json:"h"`
	Low        *decimal.Big `json:"l"`
	Close      *decimal.Big `json:"c"`
	Volume     *decimal.Big `json:"v"`
	TradeCount uint64       `json:"n"`
	VWAP       *decimal.Big `json:"vw"`
}

type snapshot struct {
	LatestTrade  *trade `json:"latestTrade"`
	LatestQuote  *quote `json:"latestQuote"`
	MinuteBar    *bar   `json:"minuteBar"`
	DailyBar     *bar   `json:"dailyBar"`
	PrevDailyBar *bar   `json:"prevDailyBar"`
}

type stockBars struct {
	Symbol        string  `json:"symbol"`
	NextPageToken *string `json:"next_page_token"`
	Bars          []bar   `json:"bars"`
}

// This struct is a union type of all realtime messages.
type realtimeMessage struct {
	Type            string       `json:"T"`
	Code            int          `json:"code,omitempty"`
	Msg             string       `json:"msg,omitempty"`
	Trades          []string     `json:"trades,omitempty"`
	Quotes          []string     `json:"quotes,omitempty"`
	Bars            []string     `json:"bars,omitempty"`
	Symbol          string       `json:"S,omitempty"`
	TradeId         int          `json:"i,omitempty"`
	ExchangeCode    string       `json:"x,omitempty"`
	AskExchangeCode string       `json:"ax,omitempty"`
	AskPrice        *decimal.Big `json:"ap,omitempty"`
	AskSize         uint         `json:"as,omitempty"`
	BidExchangeCode string       `json:"bx,omitempty"`
	BidPrice        *decimal.Big `json:"bp,omitempty"`
	BidSize         uint         `json:"bs,omitempty"`
	Price           *decimal.Big `json:"p,omitempty"`
	TradeSize       *decimal.Big `json:"s,omitempty"`
	O               *decimal.Big `json:"o,omitempty"`
	H               *decimal.Big `json:"h,omitempty"`
	L               *decimal.Big `json:"l,omitempty"`
	C               *decimal.Big `json:"c,omitempty"`
	V               *decimal.Big `json:"v,omitempty"`
	StatusCode      string       `json:"sc,omitempty"`
	StatusMessage   string       `json:"sm,omitempty"`
	ReasonCode      string       `json:"rc,omitempty"`
	ReasonMessage   string       `json:"rm,omitempty"`
	Timestamp       time.Time    `json:"t,omitempty"`
	Cond            *[]string    `json:"c,omitempty"` // same json tag as close price
	Tape            string       `json:"z,omitempty"`
}

type asset struct {
	Id           string `json:"id"`
	Class        string `json:"class"`
	Exchange     string `json:"exchange"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Tradable     bool   `json:"tradable"`
	Marginable   bool   `json:"marginable"`
	Shortable    bool   `json:"shortable"`
	EasyToBorrow bool   `json:"easy_to_borrow"`
	Fractionable bool   `json:"fractionable"`
}

type realtimeSubscribeCommand struct {
	Action string   `json:"action"`
	Trades []string `json:"trades"`
	Quotes []string `json:"quotes"`
	Bars   []string `json:"bars"`
}

type realtimeAuthCommand struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type requestType int

const (
	requestTypeMarketData requestType = iota
	requestTypeTrading
)

const (
	messageTypeSuccess = "success"
	messageTypeTrade   = "t"
)

const (
	messageConnected     = "connected"
	messageAuthenticated = "authenticated"
)

func getCandleResolutionStr(r candles.CandleResolution) string {
	switch r {
	case candles.CandleOneMinute:
		return "1Min"
	case candles.CandleFiveMinutes:
		return "5Min"
	case candles.CandleFifteenMinutes:
		return "15Min"
	case candles.CandleThirtyMinutes:
		return "30Min"
	case candles.CandleSixtyMinutes:
		return "1Hour"
	case candles.CandleOneDay:
		return "1Day"
	case candles.CandleOneWeek:
		return "1Week"
	case candles.CandleOneMonth:
		return "1Month"
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

func getAssetClassStr(exchange string) string {
	switch exchange {
	case "US":
		return "us_equity"
	default:
		panic("unsupported exchange")
	}
}

func mapSymbolData(s asset) stockval.AssetData {
	return stockval.AssetData{
		Symbol:                s.Symbol,
		CompanyName:           s.Name,
		Mic:                   s.Exchange,
		Currency:              "USD",
		CompanyNameNormalized: stockval.NormalizeAssetName(s.Name),
		Tradable:              s.Tradable,
	}
}

func NewStockRequester(figiReq stockapi.SymbolSearchTool) stockapi.StockValueRequester {
	return &alpacaStockRequester{
		rateLimiter:           webclient.NewRateLimiter(),
		apiClient:             &http.Client{},
		tickDataMap:           skipmap.NewString[chan stockapi.RealtimeTickData](),
		pendingCloseListMutex: new(sync.Mutex),
		cache:                 cache.NewAssetCache(GetBrokerId()),
		figiReq:               figiReq,
	}
}

func GetBrokerId() stockval.BrokerId {
	return "alpaca"
}

func (requester *alpacaStockRequester) RemainingApiLimit() int {
	return requester.rateLimiter.Remaining()
}

func (requester *alpacaStockRequester) createRequest(cmd string, t requestType) (*http.Request, error) {
	var url string
	if t == requestTypeTrading {
		url = requester.config.PaperTradingUrl
	} else {
		url = requester.config.DataUrl
	}
	req, err := http.NewRequest("GET", url+cmd, nil)
	if err != nil {
		return req, err
	}
	req.Header.Add("APCA-API-KEY-ID", requester.config.ApiKey)
	req.Header.Add("APCA-API-SECRET-KEY", requester.config.ApiSecret)

	return req, err
}

func (requester *alpacaStockRequester) runRequest(ctx context.Context, cmd string, query url.Values, t requestType) (*http.Response, error) {
	retry := true
	var resp *http.Response
	for retry {
		// Throttle according to http headers.
		err := requester.rateLimiter.Wait(ctx)
		if err != nil {
			return nil, err
		}

		req, err := requester.createRequest(cmd, t)
		if err != nil {
			return nil, err
		}
		if query != nil {
			req.URL.RawQuery = query.Encode()
		}

		resp, err = requester.apiClient.Do(req)
		if err != nil {
			return nil, err
		}
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

func (requester *alpacaStockRequester) FindAsset(ctx context.Context, entry <-chan stockapi.SearchRequest, response chan<- stockapi.SearchResponse) {
	defer close(response)

	// Use sync queries when requesting figi (unbuffered channels).
	figiRequestChan := make(chan stockapi.SearchRequest)
	figiResponseChan := make(chan stockapi.SearchResponse)
	defer close(figiRequestChan)
	go requester.figiReq.FindAsset(ctx, figiRequestChan, figiResponseChan)

	symbols := requester.cache.GetAssetList(ctx, func(ctx context.Context) ([]stockval.AssetData, error) {
		query := make(url.Values)
		query.Add("asset_class", getAssetClassStr(stockval.DefaultExchange))
		resp, err := requester.runRequest(ctx, "/assets", query, requestTypeTrading)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var assets []asset
		if err = webclient.ParseJsonResponse(resp, &assets); err != nil {
			return nil, err
		}
		assetData := make([]stockval.AssetData, 0, len(assets))
		for _, s := range assets {
			assetData = append(assetData, mapSymbolData(s))
		}
		return assetData, nil
	})

	for entry := range entry {
		if stockval.IsinRegex.MatchString(entry.Text) {
			// alpaca does not provide isin data (because isin is kind of commercial).
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
		if responseData.Error == nil {
			if entry.UnambiguousLookup {
				// alpaca does not provide figi identifiers for their assets.
				// This is really bad. The id returned by alpaca is a custom id (created by alpaca), which is
				// pretty useless, given that we allow switching between brokers.
				// We cannot use this. Therefore, we request a figi using openfigi, which is slow and may stall.
				// TODO ask alpaca to provide figi identifiers!
				figiRequestChan <- stockapi.SearchRequest{
					RequestId:         entry.RequestId,
					Text:              responseData.Result[0].Symbol,
					UnambiguousLookup: true,
				}
				figiResponse := <-figiResponseChan
				if figiResponse.Error == nil {
					responseData.Result[0].Figi = figiResponse.Result[0].Figi
				} else {
					responseData.Error = figiResponse.Error
				}
			}
		}
		if responseData.Error != nil {
			log.Print(responseData.Error)
		}
		response <- responseData
	}
}

func (requester *alpacaStockRequester) queryAsset(ctx context.Context, symbols cache.AssetList, entry stockapi.SearchRequest) stockapi.SearchResponse {
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

func (requester *alpacaStockRequester) QueryQuote(ctx context.Context, entry <-chan stockval.AssetData, response chan<- stockapi.QueryQuoteResponse) {
	defer close(response)

	for entry := range entry {
		resp := requester.querySymbolQuote(ctx, entry)
		if resp.Error != nil {
			log.Print(resp.Error)
		}
		response <- resp
	}
	log.Println("alpaca QueryQuote terminating.")
}

func (requester *alpacaStockRequester) querySymbolQuote(ctx context.Context, entry stockval.AssetData) stockapi.QueryQuoteResponse {
	resp, err := requester.runRequest(ctx, "/stocks/"+entry.Symbol+"/snapshot", nil, requestTypeMarketData)
	if err != nil {
		return stockapi.QueryQuoteResponse{Figi: entry.Figi, Error: err}
	}
	defer resp.Body.Close()

	var snapshot snapshot
	if err = webclient.ParseJsonResponse(resp, &snapshot); err != nil {
		return stockapi.QueryQuoteResponse{Figi: entry.Figi, Error: err}
	}

	return stockapi.QueryQuoteResponse{
		Figi:               entry.Figi,
		CurrentPrice:       snapshot.DailyBar.Close,
		PreviousClosePrice: snapshot.PrevDailyBar.Close,
		DeltaPercentage:    stockval.CalculateDeltaPercentage(snapshot.PrevDailyBar.Close, snapshot.LatestTrade.Price),
	}
}

func (requester *alpacaStockRequester) QueryCandles(ctx context.Context, request <-chan stockapi.CandlesRequest, response chan<- stockapi.QueryCandlesResponse) {
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

func (requester *alpacaStockRequester) querySymbolCandles(ctx context.Context, entry stockval.AssetData, resolution candles.CandleResolution,
	fromTime time.Time, toTime time.Time) stockapi.QueryCandlesResponse {
	// Alpaca is really strange when processing time filters.
	// An end-filter of "now" will be rejected during after hours, for whatever reason.
	// Therefore, we only specify the filters if they are not today (or future),
	// otherwise we use defaults.
	nowUtc := time.Now().UTC()
	nowYear := nowUtc.Year()
	nowYearDay := nowUtc.YearDay()
	fromTimeUtc := fromTime.UTC()

	var data []indapi.CandleData
	var nextPageToken string
	hasNextPage := true

	for hasNextPage {
		query := make(url.Values)
		query.Add("timeframe", getCandleResolutionStr(resolution))
		if fromTimeUtc.Year() < nowYear || fromTimeUtc.YearDay() < nowYearDay {
			query.Add("start", fromTimeUtc.Format(time.RFC3339Nano))
		}
		toTimeUtc := toTime.UTC()
		if toTimeUtc.Year() < nowYear || toTimeUtc.YearDay() < nowYearDay {
			query.Add("end", toTimeUtc.Format(time.RFC3339Nano))
		}
		if nextPageToken != "" {
			query.Add("page_token", nextPageToken)
		}
		query.Add("adjustment", "all") // split & dividend adjustment
		query.Add("limit", "10000")
		resp, err := requester.runRequest(ctx, "/stocks/"+entry.Symbol+"/bars", query, requestTypeMarketData)
		if err != nil {
			return stockapi.QueryCandlesResponse{Figi: entry.Figi, Error: err}
		}

		var stockBars stockBars
		if err = webclient.ParseJsonResponse(resp, &stockBars); err != nil {
			resp.Body.Close()
			return stockapi.QueryCandlesResponse{Figi: entry.Figi, Error: err}
		}

		for _, b := range stockBars.Bars {
			data = append(data, indapi.CandleData{
				Timestamp:  b.Timestamp,
				OpenPrice:  b.Open,
				HighPrice:  b.High,
				LowPrice:   b.Low,
				ClosePrice: b.Close,
				Volume:     b.Volume,
			})
		}

		hasNextPage = stockBars.NextPageToken != nil && *stockBars.NextPageToken != ""
		if hasNextPage {
			nextPageToken = *stockBars.NextPageToken
		}

		resp.Body.Close()
	}
	return stockapi.QueryCandlesResponse{
		Figi:       entry.Figi,
		Resolution: resolution,
		Data:       data,
	}
}

func (requester *alpacaStockRequester) initRealtimeConnection(ctx context.Context) {
	if requester.realtimeConn != nil {
		log.Fatal("only a single realtime connection is supported")
	}
	log.Printf("establishing alpaca realtime connection.")
	var err error
	requester.realtimeConn, _, err = websocket.DefaultDialer.DialContext(ctx, requester.config.WsUrl+"/iex", nil)
	if err != nil {
		// TODO this should not be a fatal error
		log.Fatalf("could not connect to alpaca websocket: %v", err)
	}
	// wait for "connect" message
	var initMessage []realtimeMessage
	err = requester.realtimeConn.ReadJSON(&initMessage)
	if err != nil || len(initMessage) != 1 || initMessage[0].Type != messageTypeSuccess || initMessage[0].Msg != messageConnected {
		// TODO this should not be a fatal error
		log.Fatalf("could not read alpaca realtime connect message: %v", err)
	}
	// authenticate
	authCmd := realtimeAuthCommand{
		Action: "auth",
		Key:    requester.config.ApiKey,
		Secret: requester.config.ApiSecret,
	}
	msg, _ := json.Marshal(authCmd)
	requester.realtimeConn.WriteMessage(websocket.TextMessage, msg)
	// wait for confirmation
	var confirmMessage []realtimeMessage
	err = requester.realtimeConn.ReadJSON(&confirmMessage)
	if err != nil || len(confirmMessage) != 1 || confirmMessage[0].Type != messageTypeSuccess || confirmMessage[0].Msg != messageAuthenticated {
		// TODO this should not be a fatal error
		log.Fatalf("could not authenticate alpaca realtime: %v", err)
	}

}

func (requester *alpacaStockRequester) handleRealtimeData() {
	for {
		var data []realtimeMessage
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
			// TODO reconnect
			log.Print("alpaca realtime connection was terminated.")
			break
		}
		for i := range data {
			if data[i].Type == messageTypeTrade {
				tickChan, exists := requester.tickDataMap.Load(data[i].Symbol)
				if exists {
					if data[i].Timestamp.Before(time.Now().Add(-time.Minute)) {
						log.Printf("Symbol %s: Old realtime data received.", data[i].Symbol)
					}
					// Default: Normal trade.
					tradeContext := stockval.NewTradeContext()
					if data[i].Cond != nil {
						// consider trade conditions, they are different depending on tape
						conditionMap, tapeExists := stockval.TapeConditionMap[data[i].Tape]
						if tapeExists {
							for _, c := range *data[i].Cond {
								context, exists := conditionMap[c]
								if exists {
									tradeContext = tradeContext.Combine(context)
								}
							}
						} else {
							log.Printf("alpaca sent unknown tape: %v", data[i].Tape)
						}
					}
					tickData := stockapi.RealtimeTickData{
						Timestamp:    data[i].Timestamp,
						Price:        data[i].Price,
						Volume:       data[i].TradeSize,
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
								log.Printf("Symbol %s: Buffer overflow. Old realtime data is being removed.", data[i].Symbol)
							default:
								log.Printf("Symbol %s: Buffer overflow. New realtime data is being dropped.", data[i].Symbol)
							}
						default:
							log.Printf("Symbol %s: Buffer cannot be read from or written to.", data[i].Symbol)
						}
					}
				}
			}
		}
	}
}

func (requester *alpacaStockRequester) SubscribeTrades(ctx context.Context, entry <-chan stockapi.SubscribeTradesRequest, response chan<- stockapi.SubscribeTradesResponse) {
	defer close(response)
	for entry := range entry {
		// connect whenever we receive a first subscription message.
		// this avoids creating a realtime connection to brokers which are not used.
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
			subscribeCommand := realtimeSubscribeCommand{
				Action: getRealtimeDataSubscriptionStr(entry.Type),
				Trades: []string{entry.Stock.Symbol},
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

func (requester *alpacaStockRequester) ReadConfig(c config.Config) error {
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
	alpacaConfig := appConfig.BrokerConfig[GetBrokerId()]
	return len(alpacaConfig.DataUrl) > 0 && len(alpacaConfig.PaperTradingUrl) > 0 && len(alpacaConfig.TradingUrl) > 0 && len(alpacaConfig.WsUrl) > 0 && len(alpacaConfig.ApiKey) > 0 && len(alpacaConfig.ApiSecret) > 0
}
