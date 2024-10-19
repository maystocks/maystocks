// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockviz

import (
	"context"
	"image"
	"log"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/candles"
	"maystocks/indapi/indicators"
	"maystocks/stockapi"
	"maystocks/stockplot"
	"maystocks/stockval"
	"maystocks/widgets"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inkeliz/giohyperlink"
	"github.com/zhangyunhao116/skipmap"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type stockAppUiState int

const (
	StatePlot stockAppUiState = iota
	StateSettings
	StateIndicators
)

type StockWindow struct {
	win  *app.Window
	size widgets.DpPoint
}

type StockApp struct {
	windows            []StockWindow
	brokerData         map[stockval.BrokerId]*BrokerData
	vizMap             *skipmap.Int32Map[PlotView]
	lastUiIndex        *int32
	numUiPlots         image.Point
	uiState            stockAppUiState
	indicatorsIndex    int
	addRemovePlotMutex *sync.Mutex
	terminateWg        *sync.WaitGroup
	terminateTimerChan chan struct{}
	config             config.Config
	plotLayouts        []layout.FlexChild
	plotRows           []layout.FlexChild
	widgetStack        []layout.StackChild
	configView         *widgets.ConfigView
	indicatorsView     *widgets.IndicatorsView
	messageField       *widgets.MessageField
	plotTheme          *widgets.PlotTheme
	matTheme           *material.Theme
	broker             map[stockval.BrokerId]stockapi.Broker
	defaultBroker      stockval.BrokerId
}

type BrokerData struct {
	dataRequestChan    chan stockapi.SubscribeDataRequest
	dataResponseChan   chan stockapi.SubscribeDataResponse
	tradeRequestChan   chan stockapi.TradeRequest
	tradeResponseChan  chan stockapi.TradeResponse
	stockMap           *skipmap.StringMap[PriceData]
	refreshTimeSeconds int
}

type plotData struct {
	Entry            stockval.AssetData
	CandleResolution candles.CandleResolution
	BrokerName       stockval.BrokerId
	UiIndex          int32
	ScalingX         stockval.PlotScaling
	SubPlots         []stockplot.SubPlotData
}

type StockUiUpdater interface {
	Invalidate()
	AddPlot(ctx context.Context, plotData plotData, appTradingUrl string)
	RemovePlot(entry stockval.AssetData, uiIndex int32)
	UpdatePlot(uiIndex int32, v PlotView)
	ShowSettings()
	ShowIndicators(uiIndex int32)
}

func NewStockApp(c config.Config) *StockApp {
	return &StockApp{
		windows:            make([]StockWindow, 1),
		brokerData:         make(map[stockval.BrokerId]*BrokerData),
		vizMap:             skipmap.NewInt32[PlotView](),
		lastUiIndex:        new(int32),
		addRemovePlotMutex: new(sync.Mutex),
		terminateWg:        new(sync.WaitGroup),
		config:             c,
		configView:         widgets.NewConfigView(config.NewBrokerConfigMap(), c),
		indicatorsView:     widgets.NewIndicatorsView(),
		messageField:       widgets.NewMessageField(),
	}
}

func (a *StockApp) Initialize(ctx context.Context, svr map[stockval.BrokerId]stockapi.Broker,
	defaultBroker stockval.BrokerId) error {
	a.broker = svr
	a.defaultBroker = defaultBroker
	a.terminateTimerChan = make(chan struct{})

	// Initialize broker data.
	for name, r := range a.broker {
		p := &BrokerData{
			// TODO size of buffered channels?
			dataRequestChan:   make(chan stockapi.SubscribeDataRequest, 10),
			dataResponseChan:  make(chan stockapi.SubscribeDataResponse, 10),
			tradeRequestChan:  make(chan stockapi.TradeRequest, 10),
			tradeResponseChan: make(chan stockapi.TradeResponse, 10),
			stockMap:          skipmap.NewString[PriceData](),
		}
		a.brokerData[name] = p

		go r.SubscribeData(ctx, p.dataRequestChan, p.dataResponseChan)
		a.terminateWg.Add(1)
		go a.handleDataResponseChan(p.dataResponseChan, p.stockMap)

		go r.TradeAsset(ctx, p.tradeRequestChan, p.tradeResponseChan, true)
		a.terminateWg.Add(1)
		go a.handleTradeResponseChan(p.tradeResponseChan)
	}

	err := a.reloadConfiguration(ctx)
	if err != nil {
		return err
	}

	for broker, data := range a.brokerData {
		a.terminateWg.Add(1)
		go a.handlePeriodicUpdate(broker, data.refreshTimeSeconds)
	}

	return nil
}

func (a *StockApp) reloadConfiguration(ctx context.Context) error {
	appConfig, err := a.config.Copy(false)
	if err != nil {
		return err
	}
	// Themes need to be set up first, because other settings might use them.
	if appConfig.LightTheme {
		a.matTheme = widgets.NewLightMaterialTheme()
		a.plotTheme = widgets.NewLightPlotTheme()
	} else {
		a.matTheme = widgets.NewDarkMaterialTheme()
		a.plotTheme = widgets.NewDarkPlotTheme()
	}

	if a.numUiPlots != appConfig.WindowConfig[0].NumPlots {
		a.numUiPlots = appConfig.WindowConfig[0].NumPlots
		// Clear and recreate all plots.
		a.vizMap.Range(
			func(uiIndex int32, w PlotView) bool {
				// Removing while iterating seems to work fine with skipmap. Fingers crossed.
				a.RemovePlot(w.AssetData, uiIndex)
				return true
			})
		atomic.StoreInt32(a.lastUiIndex, 0)
		for _, plotConfig := range appConfig.WindowConfig[0].PlotConfig {
			broker := plotConfig.BrokerId
			_, exists := a.broker[broker]
			if !exists {
				broker = a.defaultBroker
			}
			subPlots := make([]stockplot.SubPlotData, 0, len(plotConfig.SubPlotConfig))
			for _, s := range plotConfig.SubPlotConfig {
				indicatorData := make([]indapi.IndicatorData, 0, len(s.Indicators))
				for _, c := range s.Indicators {
					indicatorData = append(indicatorData, indicators.Create(c.IndicatorId, c.Properties, c.Colors))
				}
				subPlots = append(subPlots, stockplot.SubPlotData{Type: s.Type, Indicators: indicatorData})
			}
			a.AddPlot(
				ctx,
				plotData{
					plotConfig.AssetData,
					plotConfig.Resolution,
					broker,
					0,
					plotConfig.PlotScalingX,
					subPlots,
				},
				appConfig.BrokerConfig[broker].AppTradingUrl)
		}
	}
	a.vizMap.Range(
		func(uiIndex int32, w PlotView) bool {
			configIndex := int(uiIndex - 1)
			if configIndex >= len(appConfig.WindowConfig[0].PlotConfig) {
				return true
			}
			plotConfig := appConfig.WindowConfig[0].PlotConfig[configIndex]
			subPlotData := w.Plot.GetSubPlotData()
			changed := len(plotConfig.SubPlotConfig) != len(subPlotData)
			if !changed {
				for i, s := range subPlotData {
					if plotConfig.SubPlotConfig[i].Type != s.Type {
						changed = true
						break
					}
					if len(plotConfig.SubPlotConfig[i].Indicators) != len(s.Indicators) {
						changed = true
						break
					}
					for j, c := range s.Indicators {
						if plotConfig.SubPlotConfig[i].Indicators[j].IndicatorId != c.GetId() {
							changed = true
							break
						}
						if !reflect.DeepEqual(plotConfig.SubPlotConfig[i].Indicators[j].Properties, c.GetProperties()) {
							changed = true
							break
						}
						if !reflect.DeepEqual(plotConfig.SubPlotConfig[i].Indicators[j].Colors, c.GetColors()) {
							changed = true
							break
						}
					}
				}
			}
			if changed {
				// Update subplots
				subPlots := make([]stockplot.SubPlotData, 0, len(plotConfig.SubPlotConfig))
				for _, s := range plotConfig.SubPlotConfig {
					indicatorData := make([]indapi.IndicatorData, 0, len(s.Indicators))
					for _, c := range s.Indicators {
						indicatorData = append(indicatorData, indicators.Create(c.IndicatorId, c.Properties, c.Colors))
					}
					subPlots = append(subPlots, stockplot.SubPlotData{Type: s.Type, Indicators: indicatorData})
				}
				newPlotView := w
				newPlotView.UpdateSubPlots(subPlots)
				a.UpdatePlot(w.UiIndex, newPlotView)
			}
			return true
		})

	for broker, data := range a.brokerData {
		data.refreshTimeSeconds = appConfig.BrokerConfig[broker].RefreshIntervalSeconds
	}

	a.configView.UpdateUiFromConfig(&appConfig)
	a.configView.SetWindowConfig(&appConfig)
	a.indicatorsView.SetIndicatorConfig(&appConfig)
	a.windows[0].size.X = unit.Dp(appConfig.WindowConfig[0].Size.X)
	a.windows[0].size.Y = unit.Dp(appConfig.WindowConfig[0].Size.Y)
	return nil
}

func (a *StockApp) saveConfiguration() error {
	appConfig, err := a.config.Lock()
	if err != nil {
		return err
	}
	a.indicatorsView.GetIndicatorConfig(appConfig)
	a.configView.GetWindowConfig(appConfig)
	a.vizMap.Range(
		func(uiIndex int32, w PlotView) bool {
			configIndex := int(uiIndex - 1)
			if configIndex >= len(appConfig.WindowConfig[0].PlotConfig) {
				return true
			}
			w.saveConfiguration(&appConfig.WindowConfig[0].PlotConfig[configIndex])
			return true
		})
	appConfig.WindowConfig[0].Size.X = int(a.windows[0].size.X)
	appConfig.WindowConfig[0].Size.Y = int(a.windows[0].size.Y)
	forceWriting := a.configView.UpdateConfigFromUi(appConfig)
	return a.config.Unlock(appConfig, forceWriting)
}

func (a *StockApp) saveAndReloadConfiguration(ctx context.Context) {
	err := a.saveConfiguration()
	if err != nil {
		log.Printf("error updating configuration: %v", err)
	}
	err = a.reloadConfiguration(ctx)
	if err != nil {
		log.Printf("error reloading configuration: %v", err)
	}
}

func (a *StockApp) handleDataResponseChan(dataResponseChan chan stockapi.SubscribeDataResponse, stockMap *skipmap.StringMap[PriceData]) {
	defer a.terminateWg.Done()
	for responseData := range dataResponseChan {
		if responseData.Error != nil {
			log.Printf("error requesting realtime data: %v", responseData.Error)
			continue
		}
		if responseData.Type == stockapi.RealtimeTradesSubscribe {
			data, ok := stockMap.Load(responseData.Figi)
			if !ok {
				log.Printf("error: invalid realtime trades channel")
				continue
			}
			data.SetRealtimeTradesChan(responseData.TickData, a)
		} else if responseData.Type == stockapi.RealtimeBidAskSubscribe {
			data, ok := stockMap.Load(responseData.Figi)
			if !ok {
				log.Printf("error: invalid realtime bid/ask channel")
				continue
			}
			data.SetRealtimeBidAskChan(responseData.BidAskData, a)
		}
	}
}

func (a *StockApp) handleTradeResponseChan(tradeResponseChan chan stockapi.TradeResponse) {
	defer a.terminateWg.Done()
	for responseData := range tradeResponseChan {
		if responseData.Error != nil {
			log.Printf("error trading: %v", responseData.Error)
			continue
		}
	}
}

func (a *StockApp) handlePeriodicUpdate(brokerName stockval.BrokerId, refreshTimeSeconds int) {
	defer a.terminateWg.Done()
	if refreshTimeSeconds <= 0 {
		return
	}
	terminated := false
	type requestedCandles struct {
		figi       string
		resolution candles.CandleResolution
	}
	var refreshedCandles []requestedCandles
	for !terminated {
		select {
		case <-a.terminateTimerChan:
			terminated = true
		case <-time.After(time.Duration(refreshTimeSeconds) * time.Second):
			refreshedCandles = refreshedCandles[:0]
			// TODO only update brokers which are "in use"
			a.vizMap.Range(
				func(key int32, w PlotView) bool {
					// Update data for all plots of this broker.
					if w.GetLastBrokerName() != brokerName {
						return true
					}
					// Avoid duplicate queries here, this can add up pretty much.
					r := w.GetLastCandleResolution()
					for _, refreshed := range refreshedCandles {
						if refreshed.figi == w.AssetData.Figi && refreshed.resolution == r {
							// this is a duplicate, do not request twice.
							return true
						}
					}
					refreshedCandles = append(refreshedCandles, requestedCandles{figi: w.AssetData.Figi, resolution: r})

					brokerData, ok := a.brokerData[brokerName]
					if !ok {
						log.Printf("Could not find broker for refresh: %s", brokerName)
						return true
					}
					priceData, ok := brokerData.stockMap.Load(w.AssetData.Figi)
					if ok {
						priceData.RefreshCandles(r)
					} else {
						log.Printf("Could not find price data for refresh: %s", w.AssetData.Figi)
					}
					return true
				},
			)
		}
	}
}

func (a *StockApp) Run(ctx context.Context) {
	a.createWindows()
	err := a.handleEvents(ctx)
	if err != nil {
		log.Printf("terminating with error: %v", err)
	}
	a.terminate()
}

func (a *StockApp) Invalidate() {
	a.windows[0].win.Invalidate()
}

func (a *StockApp) createWindows() {
	// TODO store window size and position
	size := a.windows[0].size
	if size.X == 0 || size.Y == 0 {
		size.X = 1280
		size.Y = 1024
	}
	a.windows[0].win = new(app.Window)
	a.windows[0].win.Option(
		app.Title(a.config.GetAppName()),
		app.Size(size.X, size.Y),
		// Not working on mac: app.Maximized.Option(),
	)
}

func (a *StockApp) handleEvents(ctx context.Context) error {
	var ops op.Ops
	firstFrame := true
	// TODO support multiple windows
	for {
		event := a.windows[0].win.Event()
		giohyperlink.ListenEvents(event)
		switch e := event.(type) {
		case app.FrameEvent:
			if firstFrame {
				// Maximize does not work during window creation on MacOS.
				// Therefore it is executed in first frame.
				a.windows[0].win.Perform(system.ActionMaximize)
				firstFrame = false
			}
			gtx := app.NewContext(&ops, e)
			paint.Fill(gtx.Ops, a.matTheme.Bg)
			switch a.uiState {
			case StatePlot:
				a.layoutPlots(ctx, gtx)
			case StateSettings:
				a.configView.Layout(a.matTheme, gtx)
				if a.configView.ConfirmClicked() {
					a.saveAndReloadConfiguration(ctx)
					a.uiState = StatePlot
				}
			case StateIndicators:
				a.indicatorsView.Layout(a.matTheme, gtx, a.indicatorsIndex)
				if a.indicatorsView.ConfirmClicked() {
					a.saveAndReloadConfiguration(ctx)
					a.uiState = StatePlot
				}
			}
			e.Frame(gtx.Ops)
		case app.DestroyEvent:
			return e.Err
		}
	}
}

func (a *StockApp) layoutPlots(ctx context.Context, gtx layout.Context) {
	a.plotLayouts = a.plotLayouts[:0]
	if a.numUiPlots.X*a.numUiPlots.Y > 0 { // do not divide by zero even if "kind of" a race condition occurs
		a.vizMap.Range(
			func(uiIndex int32, w PlotView) bool {
				brokerData, ok := a.brokerData[w.GetLastBrokerName()]
				if !ok {
					return true
				}
				priceData, ok := brokerData.stockMap.Load(w.AssetData.Figi)
				if !ok {
					return true
				}
				a.plotLayouts = append(a.plotLayouts, layout.Flexed(
					1/float32(a.numUiPlots.X),
					func(gtx layout.Context) layout.Dimensions {
						d, refreshQuote := w.Layout(ctx, gtx, a.matTheme, &priceData)
						startTime, endTime, refreshPlot := w.UpdatePlotRange()
						if refreshQuote {
							priceData.RefreshQuote()
						}
						if refreshPlot {
							candleResolution := w.GetLastCandleResolution()
							priceData.candlesMutex.Lock()
							candleUpdater := priceData.candles[candleResolution]
							priceData.candlesMutex.Unlock()
							candleUpdater.SetCandleTime(uiIndex, startTime, endTime)
							priceData.RefreshCandles(w.GetLastCandleResolution())
						}
						return d
					}))
				return true
			},
		)
	}

	a.plotRows = a.plotRows[:0]
	if len(a.plotLayouts) == a.numUiPlots.X*a.numUiPlots.Y {
		for row := 0; row < a.numUiPlots.Y; row++ {
			minIndex := row * a.numUiPlots.X
			maxIndex := (row + 1) * a.numUiPlots.X
			a.plotRows = append(
				a.plotRows,
				layout.Flexed(
					1/float32(a.numUiPlots.Y),
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:    layout.Horizontal,
							Spacing: layout.SpaceEnd,
						}.Layout(
							gtx,
							a.plotLayouts[minIndex:maxIndex]...,
						)
					}),
			)
		}
	}
	a.widgetStack = a.widgetStack[:0]
	a.widgetStack = append(
		a.widgetStack,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceEnd,
			}.Layout(
				gtx,
				a.plotRows...,
			)
		}),
	)

	for p, r := range a.broker {
		if r.RemainingApiLimit() < 1 {
			a.widgetStack = append(
				a.widgetStack,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return a.messageField.Layout(string(p)+" API Limit exceeded. No more requests possible for now.", gtx, a.matTheme)
				}),
			)
			break
		}
	}
	layout.Stack{
		Alignment: layout.Center,
	}.Layout(
		gtx,
		a.widgetStack...,
	)
}

func (a *StockApp) terminate() {
	err := a.saveConfiguration()
	if err != nil {
		log.Printf("error saving configuration: %v", err)
	}
	a.terminateTimerChan <- struct{}{}
	close(a.terminateTimerChan)
	for _, p := range a.brokerData {
		close(p.dataRequestChan)
		close(p.tradeRequestChan)
	}
	a.terminateWg.Wait()
}

func (a *StockApp) getBrokerList() stockval.BrokerList {
	brokerList := make(stockval.BrokerList, len(a.brokerData))
	var i int
	for p := range a.brokerData {
		brokerList[i] = p
		i++
	}
	sort.Sort(brokerList)
	return brokerList
}

func (a *StockApp) AddPlot(ctx context.Context, plotData plotData, appTradingUrl string) {
	log.Printf("Adding plot %d for asset %s", plotData.UiIndex, plotData.Entry.Figi)
	a.addRemovePlotMutex.Lock()
	defer a.addRemovePlotMutex.Unlock()
	brokerData, ok := a.brokerData[plotData.BrokerName]
	if !ok {
		panic("invalid data broker name")
	}
	broker, ok := a.broker[plotData.BrokerName]
	if !ok {
		panic("invalid broker name")
	}
	w := NewPlotView(a.getBrokerList(), a.plotTheme)
	if plotData.UiIndex == 0 {
		plotData.UiIndex = atomic.AddInt32(a.lastUiIndex, 1)
	}
	w.Initialize(ctx, plotData, broker, a, appTradingUrl)
	a.vizMap.Store(w.UiIndex, w)

	_, loaded := brokerData.stockMap.LoadOrStoreLazy(plotData.Entry.Figi, func() PriceData {
		priceData := NewPriceData(plotData.Entry)
		priceData.Initialize(ctx, broker, a)
		return priceData
	})
	if !loaded {
		// Request realtime data for new stocks.
		dataRequest := stockapi.SubscribeDataRequest{
			Asset: plotData.Entry,
			Type:  stockapi.RealtimeTradesSubscribe,
		}
		brokerData.dataRequestChan <- dataRequest
		if broker.GetCapabilities().RealtimeBidAsk {
			bidAskRequest := stockapi.SubscribeDataRequest{
				Asset: plotData.Entry,
				Type:  stockapi.RealtimeBidAskSubscribe,
			}
			brokerData.dataRequestChan <- bidAskRequest
		}

		// Re-request asset data in order to update tradable flag.
		w.SearchRequestChan <- stockapi.SearchRequest{
			RequestId:         strconv.Itoa(int(plotData.UiIndex)),
			Text:              plotData.Entry.Symbol,
			UnambiguousLookup: true,
		}
	}
}

func (a *StockApp) RemovePlot(entry stockval.AssetData, uiIndex int32) {
	a.addRemovePlotMutex.Lock()
	defer a.addRemovePlotMutex.Unlock()
	plotWindow, loaded := a.vizMap.LoadAndDelete(uiIndex)
	if !loaded {
		panic("race condition: trying to delete non-existing plot window")
	}
	brokerName := plotWindow.GetLastBrokerName()
	plotWindow.Cleanup()
	// If there is no more plot, remove and unsubscribe data.
	var dataNeeded bool
	a.vizMap.Range(
		func(key int32, w PlotView) bool {
			if entry.Figi == w.AssetData.Figi && brokerName == w.GetLastBrokerName() {
				dataNeeded = true
				return false
			}
			return true
		},
	)
	if !dataNeeded {
		brokerData, ok := a.brokerData[brokerName]
		if !ok {
			panic("invalid broker data name when removing plot")
		}
		broker, ok := a.broker[brokerName]
		if !ok {
			panic("invalid broker name when removing plot")
		}
		priceData, loaded := brokerData.stockMap.LoadAndDelete(entry.Figi)
		if !loaded {
			panic("race condition: trying to delete non-existing data")
		}
		priceData.Cleanup()
		// unsubscribe realtime data
		tradesRequestData := stockapi.SubscribeDataRequest{
			Asset: entry,
			Type:  stockapi.RealtimeTradesUnsubscribe,
		}
		brokerData.dataRequestChan <- tradesRequestData
		if broker.GetCapabilities().RealtimeBidAsk {
			bidAskRequestData := stockapi.SubscribeDataRequest{
				Asset: entry,
				Type:  stockapi.RealtimeBidAskUnsubscribe,
			}
			brokerData.dataRequestChan <- bidAskRequestData
		}
	}
}

func (a *StockApp) UpdatePlot(uiIndex int32, v PlotView) {
	a.vizMap.Store(uiIndex, v)
}

func (a *StockApp) ShowSettings() {
	a.uiState = StateSettings
}

func (a *StockApp) ShowIndicators(uiIndex int32) {
	a.uiState = StateIndicators
	a.indicatorsIndex = max(0, int(uiIndex)-1)
}
