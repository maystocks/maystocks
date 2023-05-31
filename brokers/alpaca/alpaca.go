// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package alpaca

import (
	"context"
	"encoding/json"
	"errors"
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
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/gorilla/websocket"
)

// We are not using the official alpaca client SDK, because it uses float64.
// While float64 is better than the float32 used by finnhub, it is still bad for price values.
// We directly unmarshal values into decimal.Big.
type alpacaStockRequester struct {
	// "golang.org/x/time/rate" does not work well, as alpaca resets every 60 seconds.
	rateLimiter   *webclient.RateLimiter
	apiClient     *http.Client
	realtimeConn  *websocket.Conn
	tickDataMap   *stockval.RealtimeChanMap[stockapi.RealtimeTickData]
	bidAskDataMap *stockval.RealtimeChanMap[stockapi.RealtimeBidAskData]
	cache         *cache.AssetCache
	figiReq       stockapi.SymbolSearchTool
	config        config.BrokerConfig
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
	messageTypeQuote   = "q"
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

func getRealtimeSubscribeCommand(s stockapi.RealtimeDataSubscription, entry stockval.AssetData) realtimeSubscribeCommand {
	switch s {
	case stockapi.RealtimeTradesSubscribe:
		return realtimeSubscribeCommand{
			Action: "subscribe",
			Trades: []string{entry.Symbol},
		}
	case stockapi.RealtimeTradesUnsubscribe:
		return realtimeSubscribeCommand{
			Action: "unsubscribe",
			Trades: []string{entry.Symbol},
		}
	case stockapi.RealtimeBidAskSubscribe:
		return realtimeSubscribeCommand{
			Action: "subscribe",
			Quotes: []string{entry.Symbol},
		}
	case stockapi.RealtimeBidAskUnsubscribe:
		return realtimeSubscribeCommand{
			Action: "unsubscribe",
			Quotes: []string{entry.Symbol},
		}
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
		rateLimiter:   webclient.NewRateLimiter(),
		apiClient:     &http.Client{},
		tickDataMap:   stockval.NewRealtimeChanMap[stockapi.RealtimeTickData](),
		bidAskDataMap: stockval.NewRealtimeChanMap[stockapi.RealtimeBidAskData](),
		cache:         cache.NewAssetCache(GetBrokerId()),
		figiReq:       figiReq,
	}
}

func GetBrokerId() stockval.BrokerId {
	return "alpaca"
}

func (rq *alpacaStockRequester) GetCapabilities() stockapi.Capabilities {
	return stockapi.Capabilities{
		RealtimeBidAsk: true,
	}
}

func (rq *alpacaStockRequester) RemainingApiLimit() int {
	return rq.rateLimiter.Remaining()
}

func (rq *alpacaStockRequester) createRequest(cmd string, t requestType) (*http.Request, error) {
	var url string
	if t == requestTypeTrading {
		url = rq.config.PaperTradingUrl
	} else {
		url = rq.config.DataUrl
	}
	req, err := http.NewRequest("GET", url+cmd, nil)
	if err != nil {
		return req, err
	}
	req.Header.Add("APCA-API-KEY-ID", rq.config.ApiKey)
	req.Header.Add("APCA-API-SECRET-KEY", rq.config.ApiSecret)

	return req, err
}

func (rq *alpacaStockRequester) runRequest(ctx context.Context, cmd string, query url.Values, t requestType) (*http.Response, error) {
	retry := true
	var resp *http.Response
	for retry {
		// Throttle according to http headers.
		err := rq.rateLimiter.Wait(ctx)
		if err != nil {
			return nil, err
		}

		req, err := rq.createRequest(cmd, t)
		if err != nil {
			return nil, err
		}
		if query != nil {
			req.URL.RawQuery = query.Encode()
		}

		resp, err = rq.apiClient.Do(req)
		if err != nil {
			return nil, err
		}
		retry, err = rq.rateLimiter.HandleResponseHeadersWithWait(ctx, resp)
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

func (rq *alpacaStockRequester) FindAsset(ctx context.Context, entry <-chan stockapi.SearchRequest, response chan<- stockapi.SearchResponse) {
	defer close(response)

	// Use sync queries when requesting figi (unbuffered channels).
	figiRequestChan := make(chan stockapi.SearchRequest)
	figiResponseChan := make(chan stockapi.SearchResponse)
	defer close(figiRequestChan)
	go rq.figiReq.FindAsset(ctx, figiRequestChan, figiResponseChan)

	symbols := rq.cache.GetAssetList(ctx, func(ctx context.Context) ([]stockval.AssetData, error) {
		query := make(url.Values)
		query.Add("asset_class", getAssetClassStr(stockval.DefaultExchange))
		resp, err := rq.runRequest(ctx, "/assets", query, requestTypeTrading)
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
		responseData := rq.queryAsset(ctx, symbols, entry)
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

func (rq *alpacaStockRequester) queryAsset(ctx context.Context, symbols cache.AssetList, entry stockapi.SearchRequest) stockapi.SearchResponse {
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

func (rq *alpacaStockRequester) QueryQuote(ctx context.Context, entry <-chan stockval.AssetData, response chan<- stockapi.QueryQuoteResponse) {
	defer close(response)

	for entry := range entry {
		resp := rq.querySymbolQuote(ctx, entry)
		if resp.Error != nil {
			log.Print(resp.Error)
		}
		response <- resp
	}
	log.Println("alpaca QueryQuote terminating.")
}

func (rq *alpacaStockRequester) querySymbolQuote(ctx context.Context, entry stockval.AssetData) stockapi.QueryQuoteResponse {
	resp, err := rq.runRequest(ctx, "/stocks/"+entry.Symbol+"/snapshot", nil, requestTypeMarketData)
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

func (rq *alpacaStockRequester) QueryCandles(ctx context.Context, request <-chan stockapi.CandlesRequest, response chan<- stockapi.QueryCandlesResponse) {
	defer close(response)

	for req := range request {
		resp := rq.querySymbolCandles(ctx, req.Stock, req.Resolution, req.FromTime, req.ToTime)
		if resp.Error != nil {
			log.Print(resp.Error)
		}
		response <- resp
	}
	log.Println("finnhub QueryCandles terminating.")
}

func (rq *alpacaStockRequester) querySymbolCandles(ctx context.Context, entry stockval.AssetData, resolution candles.CandleResolution,
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
		resp, err := rq.runRequest(ctx, "/stocks/"+entry.Symbol+"/bars", query, requestTypeMarketData)
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

func (rq *alpacaStockRequester) initRealtimeConnection(ctx context.Context) {
	if rq.realtimeConn != nil {
		log.Fatal("only a single realtime connection is supported")
	}
	log.Printf("establishing alpaca realtime connection.")
	var err error
	rq.realtimeConn, _, err = websocket.DefaultDialer.DialContext(ctx, rq.config.WsUrl+"/iex", nil)
	if err != nil {
		// TODO this should not be a fatal error
		log.Fatalf("could not connect to alpaca websocket: %v", err)
	}
	// wait for "connect" message
	var initMessage []realtimeMessage
	err = rq.realtimeConn.ReadJSON(&initMessage)
	if err != nil || len(initMessage) != 1 || initMessage[0].Type != messageTypeSuccess || initMessage[0].Msg != messageConnected {
		// TODO this should not be a fatal error
		log.Fatalf("could not read alpaca realtime connect message: %v", err)
	}
	// authenticate
	authCmd := realtimeAuthCommand{
		Action: "auth",
		Key:    rq.config.ApiKey,
		Secret: rq.config.ApiSecret,
	}
	msg, _ := json.Marshal(authCmd)
	rq.realtimeConn.WriteMessage(websocket.TextMessage, msg)
	// wait for confirmation
	var confirmMessage []realtimeMessage
	err = rq.realtimeConn.ReadJSON(&confirmMessage)
	if err != nil || len(confirmMessage) != 1 || confirmMessage[0].Type != messageTypeSuccess || confirmMessage[0].Msg != messageAuthenticated {
		// TODO this should not be a fatal error
		log.Fatalf("could not authenticate alpaca realtime: %v", err)
	}

}

func (rq *alpacaStockRequester) handleRealtimeData() {
	for {
		var data []realtimeMessage
		err := rq.realtimeConn.ReadJSON(&data)

		rq.tickDataMap.ClearPendingClose()
		rq.bidAskDataMap.ClearPendingClose()

		if err != nil {
			rq.tickDataMap.Clear()
			rq.bidAskDataMap.Clear()
			// TODO reconnect
			log.Print("alpaca realtime connection was terminated.")
			break
		}
		for i := range data {
			if data[i].Timestamp.Before(time.Now().Add(-time.Minute)) {
				log.Printf("Symbol %s: Old realtime data received.", data[i].Symbol)
			}
			if data[i].Type == messageTypeTrade {
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
				err = rq.tickDataMap.AddNewData(data[i].Symbol, tickData)
				if err != nil {
					log.Println(err)
				}
			} else if data[i].Type == messageTypeQuote {
				bidAskData := stockapi.RealtimeBidAskData{
					Timestamp: data[i].Timestamp,
					BidPrice:  data[i].BidPrice,
					BidSize:   data[i].BidSize,
					AskPrice:  data[i].AskPrice,
					AskSize:   data[i].AskSize,
				}
				err = rq.bidAskDataMap.AddNewData(data[i].Symbol, bidAskData)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (rq *alpacaStockRequester) SubscribeData(ctx context.Context, request <-chan stockapi.SubscribeDataRequest, response chan<- stockapi.SubscribeDataResponse) {
	defer close(response)
	for entry := range request {
		// connect whenever we receive a first subscription message.
		// this avoids creating a realtime connection to brokers which are not used.
		if rq.realtimeConn == nil {
			rq.initRealtimeConnection(ctx)
			go rq.handleRealtimeData()
		}

		var tickData chan stockapi.RealtimeTickData
		var bidAskData chan stockapi.RealtimeBidAskData
		var err error
		switch entry.Type {
		case stockapi.RealtimeTradesSubscribe:
			tickData, err = rq.tickDataMap.Subscribe(entry.Stock)
		case stockapi.RealtimeTradesUnsubscribe:
			err = rq.tickDataMap.Unsubscribe(entry.Stock)
		case stockapi.RealtimeBidAskSubscribe:
			bidAskData, err = rq.bidAskDataMap.Subscribe(entry.Stock)
		case stockapi.RealtimeBidAskUnsubscribe:
			err = rq.bidAskDataMap.Unsubscribe(entry.Stock)
		default:
			panic("unsupported realtime data subscription mode")
		}
		if err == nil {
			subscribeCommand := getRealtimeSubscribeCommand(entry.Type, entry.Stock)
			msg, _ := json.Marshal(subscribeCommand)
			rq.realtimeConn.WriteMessage(websocket.TextMessage, msg)
		}

		responseData := stockapi.SubscribeDataResponse{
			Figi:       entry.Stock.Figi,
			Error:      err,
			Type:       entry.Type,
			TickData:   tickData,
			BidAskData: bidAskData,
		}
		response <- responseData
	}
	if rq.realtimeConn != nil {
		rq.realtimeConn.Close()
		rq.realtimeConn = nil
	}
}

func (rq *alpacaStockRequester) ReadConfig(c config.Config) error {
	appConfig, err := c.Copy()
	if err != nil {
		return err
	}
	rq.config = appConfig.BrokerConfig[GetBrokerId()]
	rq.apiClient.Timeout = time.Second * time.Duration(rq.config.DataTimeoutSeconds)
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
