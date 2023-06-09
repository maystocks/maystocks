// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockviz

import (
	"context"
	"image"
	"log"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/calc"
	"maystocks/indapi/candles"
	"maystocks/indapi/indicators"
	"maystocks/stockapi"
	"maystocks/stockval"
	"maystocks/widgets"
	"reflect"
	"sort"
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
	brokerData         map[stockval.BrokerId]BrokerData
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
	stockRequester     map[stockval.BrokerId]stockapi.StockValueRequester
	defaultBroker      stockval.BrokerId
}

type BrokerData struct {
	stockValueRequester stockapi.StockValueRequester
	dataRequestChan     chan stockapi.SubscribeDataRequest
	dataResponseChan    chan stockapi.SubscribeDataResponse
	stockMap            *skipmap.StringMap[PriceData]
}

type StockUiUpdater interface {
	Invalidate()
	AddPlot(ctx context.Context, entry stockval.AssetData, candleResolution candles.CandleResolution, brokerName stockval.BrokerId,
		uiIndex int32, indicators []indapi.IndicatorData, scalingX stockval.PlotScaling)
	RemovePlot(entry stockval.AssetData, uiIndex int32)
	ShowSettings()
	ShowIndicators(uiIndex int32)
}

func NewStockApp(c config.Config) *StockApp {
	return &StockApp{
		windows:            make([]StockWindow, 1),
		brokerData:         make(map[stockval.BrokerId]BrokerData),
		vizMap:             skipmap.NewInt32[PlotView](),
		lastUiIndex:        new(int32),
		addRemovePlotMutex: new(sync.Mutex),
		terminateWg:        new(sync.WaitGroup),
		config:             c,
		configView:         widgets.NewConfigView(config.NewBrokerConfigMap()),
		indicatorsView:     widgets.NewIndicatorsView(),
		messageField:       widgets.NewMessageField(),
	}
}

func (a *StockApp) Initialize(ctx context.Context, svr map[stockval.BrokerId]stockapi.StockValueRequester,
	defaultBroker stockval.BrokerId) error {
	a.stockRequester = svr
	a.defaultBroker = defaultBroker

	a.terminateTimerChan = make(chan struct{})
	for name, r := range svr {
		p := BrokerData{
			stockValueRequester: r,
			// TODO size of buffered channels?
			dataRequestChan:  make(chan stockapi.SubscribeDataRequest, 10),
			dataResponseChan: make(chan stockapi.SubscribeDataResponse, 10),
			stockMap:         skipmap.NewString[PriceData](),
		}
		a.brokerData[name] = p
		go r.SubscribeData(context.Background(), p.dataRequestChan, p.dataResponseChan)
		a.terminateWg.Add(1)
		go a.handleTradesResponseChan(p)
	}
	a.terminateWg.Add(1)
	go a.handlePeriodicUpdate()

	err := a.reloadConfiguration(ctx)
	if err != nil {
		return err
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
			_, exists := a.stockRequester[broker]
			if !exists {
				broker = a.defaultBroker
			}
			indicatorData := make([]indapi.IndicatorData, 0, len(plotConfig.Indicators))
			for _, c := range plotConfig.Indicators {
				indicatorData = append(indicatorData, indicators.Create(c.IndicatorId, c.Properties, c.Color))
			}
			a.AddPlot(ctx, plotConfig.AssetData, plotConfig.Resolution, broker, 0, indicatorData, plotConfig.PlotScalingX)
		}
	}
	a.vizMap.Range(
		func(uiIndex int32, w PlotView) bool {
			configIndex := int(uiIndex - 1)
			if configIndex >= len(appConfig.WindowConfig[0].PlotConfig) {
				return true
			}
			plotConfig := appConfig.WindowConfig[0].PlotConfig[configIndex]
			changed := len(plotConfig.Indicators) != len(w.indicators)
			if !changed {
				for i := range w.indicators {
					if plotConfig.Indicators[i].IndicatorId != w.indicators[i].GetId() {
						changed = true
						break
					}
					if !reflect.DeepEqual(plotConfig.Indicators[i].Properties, w.indicators[i].GetProperties()) {
						changed = true
						break
					}
					if plotConfig.Indicators[i].Color != w.indicators[i].GetColor() {
						changed = true
						break
					}
				}
			}
			if changed {
				// TODO just change indicators instead of reloading
				a.RemovePlot(w.AssetData, uiIndex)
				indicatorData := make([]indapi.IndicatorData, 0, len(plotConfig.Indicators))
				for _, c := range plotConfig.Indicators {
					indicatorData = append(indicatorData, indicators.Create(c.IndicatorId, c.Properties, c.Color))
				}
				a.AddPlot(ctx, w.AssetData, w.GetLastCandleResolution(), w.GetLastBrokerName(), w.UiIndex, indicatorData, w.GetLastPlotScalingX())
			}
			return true
		})

	a.configView.SetBrokerConfig(&appConfig)
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
	a.configView.GetBrokerConfig(appConfig)
	return a.config.Unlock(appConfig, false)
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

func (a *StockApp) handleTradesResponseChan(p BrokerData) {
	defer a.terminateWg.Done()
	for tradesResponseData := range p.dataResponseChan {
		if tradesResponseData.Error != nil {
			log.Printf("error requesting realtime data: %v", tradesResponseData.Error)
			continue
		}
		if tradesResponseData.Type == stockapi.RealtimeTradesSubscribe {
			data, ok := p.stockMap.Load(tradesResponseData.Figi)
			if !ok {
				log.Printf("error: invalid realtime trades channel")
				continue
			}
			data.SetRealtimeTradesChan(tradesResponseData.TickData, a)
		} else if tradesResponseData.Type == stockapi.RealtimeBidAskSubscribe {
			data, ok := p.stockMap.Load(tradesResponseData.Figi)
			if !ok {
				log.Printf("error: invalid realtime bid/ask channel")
				continue
			}
			data.SetRealtimeBidAskChan(tradesResponseData.BidAskData, a)
		}
	}
}

func (a *StockApp) handlePeriodicUpdate() {
	defer a.terminateWg.Done()
	terminated := false
	type requestedCandles struct {
		brokerName stockval.BrokerId
		figi       string
		resolution candles.CandleResolution
	}
	var refreshedCandles []requestedCandles
	for !terminated {
		select {
		// TODO 20 seconds is hard coded
		case <-a.terminateTimerChan:
			terminated = true
		case <-time.After(20 * time.Second):
			refreshedCandles = refreshedCandles[:0]
			a.vizMap.Range(
				func(key int32, w PlotView) bool {
					// Update data for all plots.
					// Avoid duplicate queries here, this can add up pretty much.
					brokerName := w.GetLastBrokerName()
					r := w.GetLastCandleResolution()
					for _, refreshed := range refreshedCandles {
						if refreshed.figi == w.AssetData.Figi && refreshed.brokerName == brokerName && refreshed.resolution == r {
							// this is a duplicate, do not request twice.
							return true
						}
					}
					refreshedCandles = append(refreshedCandles, requestedCandles{brokerName: brokerName, figi: w.AssetData.Figi, resolution: r})

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
	a.windows[0].win = app.NewWindow(
		app.Title(a.config.GetAppName()),
		app.Size(size.X, size.Y),
		// TODO not working on mac app.Maximized.Option(),
		//app.Fullscreen.Option(),
	)
	a.windows[0].win.Perform(system.ActionMaximize)
}

func (a *StockApp) handleEvents(ctx context.Context) error {
	var ops op.Ops

	for i := range a.windows {
		for e := range a.windows[i].win.Events() {
			giohyperlink.ListenEvents(e)
			switch e := e.(type) {
			case system.FrameEvent:
				gtx := layout.NewContext(&ops, e)
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
			case system.DestroyEvent:
				return e.Err
			}
		}
	}
	return nil
}

func (a *StockApp) layoutPlots(ctx context.Context, gtx layout.Context) {
	a.plotLayouts = a.plotLayouts[:0]
	if a.numUiPlots.X*a.numUiPlots.Y > 0 { // do not divide by zero even if "kind of" a race condition occurs
		a.vizMap.Range(
			func(uiIndex int32, w PlotView) bool {
				brokerName := w.GetLastBrokerName()
				brokerData, ok := a.brokerData[brokerName]
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
						d, refresh := w.Layout(ctx, gtx, a.matTheme, &priceData)

						startTime, endTime, refreshPlot := w.UpdatePlotRange()
						if refreshPlot {
							candleResolution := w.GetLastCandleResolution()
							priceData.candlesMutex.Lock()
							candleUpdater := priceData.candles[candleResolution]
							priceData.candlesMutex.Unlock()
							candleUpdater.SetCandleTime(uiIndex, startTime, endTime)
						}
						if refresh || refreshPlot {
							priceData.RefreshCandles(w.GetLastCandleResolution())
						}
						if refresh {
							priceData.RefreshQuote()
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

	for p, r := range a.stockRequester {
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

func (a *StockApp) AddPlot(ctx context.Context, entry stockval.AssetData, candleResolution candles.CandleResolution,
	brokerName stockval.BrokerId, uiIndex int32, indicators []indapi.IndicatorData, scalingX stockval.PlotScaling) {
	log.Printf("Adding plot %d for asset %s", uiIndex, entry.Figi)
	a.addRemovePlotMutex.Lock()
	defer a.addRemovePlotMutex.Unlock()
	brokerData, ok := a.brokerData[brokerName]
	if !ok {
		panic("invalid data broker name")
	}
	w := NewPlotView(a.getBrokerList(), a.plotTheme)
	if uiIndex == 0 {
		uiIndex = atomic.AddInt32(a.lastUiIndex, 1)
	}
	w.Initialize(ctx, entry, candleResolution, uiIndex, brokerName, brokerData.stockValueRequester, a, indicators, scalingX)
	a.vizMap.Store(w.UiIndex, w)

	_, loaded := brokerData.stockMap.LoadOrStoreLazy(entry.Figi, func() PriceData {
		priceData := NewPriceData(entry)
		priceData.Initialize(ctx, brokerData.stockValueRequester, a)
		return priceData
	})
	if !loaded {
		// Request realtime data for new stocks.
		tradesRequestData := stockapi.SubscribeDataRequest{
			Stock: entry,
			Type:  stockapi.RealtimeTradesSubscribe,
		}
		brokerData.dataRequestChan <- tradesRequestData
		if brokerData.stockValueRequester.GetCapabilities().RealtimeBidAsk {
			bidAskRequestData := stockapi.SubscribeDataRequest{
				Stock: entry,
				Type:  stockapi.RealtimeBidAskSubscribe,
			}
			brokerData.dataRequestChan <- bidAskRequestData
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
			panic("invalid broker name when removing plot")
		}
		priceData, loaded := brokerData.stockMap.LoadAndDelete(entry.Figi)
		if !loaded {
			panic("race condition: trying to delete non-existing data")
		}
		priceData.Cleanup()
		// unsubscribe realtime data
		tradesRequestData := stockapi.SubscribeDataRequest{
			Stock: entry,
			Type:  stockapi.RealtimeTradesUnsubscribe,
		}
		brokerData.dataRequestChan <- tradesRequestData
		if brokerData.stockValueRequester.GetCapabilities().RealtimeBidAsk {
			bidAskRequestData := stockapi.SubscribeDataRequest{
				Stock: entry,
				Type:  stockapi.RealtimeBidAskUnsubscribe,
			}
			brokerData.dataRequestChan <- bidAskRequestData
		}
	}
}

func (a *StockApp) ShowSettings() {
	a.uiState = StateSettings
}

func (a *StockApp) ShowIndicators(uiIndex int32) {
	a.uiState = StateIndicators
	a.indicatorsIndex = calc.Max(0, int(uiIndex)-1)
}
