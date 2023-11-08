// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockviz

import (
	"context"
	"fmt"
	"image"
	"log"
	"maystocks/config"
	"maystocks/indapi/candles"
	"maystocks/stockapi"
	"maystocks/stockplot"
	"maystocks/stockval"
	"maystocks/widgets"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/eventx"
)

type PlotView struct {
	AssetData            stockval.AssetData
	PlotTheme            *widgets.PlotTheme
	searchField          *widgets.SearchField
	indicatorsButton     *widget.Clickable
	brokerDropdown       *widgets.DropDown
	resolutionDropDown   *widgets.DropDown
	contextMenuArea      *component.ContextArea
	contextMenu          *component.MenuState
	settingsMenuItem     *widget.Clickable
	brokerList           stockval.BrokerList
	lastBroker           *int32
	lastCandleResolution *candles.CandleResolution // use atomic accessor
	lastPlotTimeRange    *PlotTimeRange
	Plot                 *stockplot.Plot
	QuoteField           *widgets.QuoteField
	UiIndex              int32
	uiUpdater            StockUiUpdater
	appTradingUrl        string
	SearchRequestChan    chan stockapi.SearchRequest
	SearchResponseChan   chan stockapi.SearchResponse
	scalingX             *stockval.PlotScaling
	scalingXmutex        *sync.Mutex
}

type PlotTimeRange struct {
	lastPlotStartTime time.Time
	lastPlotEndTime   time.Time
}

const maxLookupResults = 32

func NewPlotView(brokerList stockval.BrokerList, theme *widgets.PlotTheme) PlotView {
	return PlotView{
		PlotTheme:            theme,
		brokerList:           brokerList,
		indicatorsButton:     new(widget.Clickable),
		contextMenuArea:      new(component.ContextArea),
		contextMenu:          new(component.MenuState),
		settingsMenuItem:     new(widget.Clickable),
		lastBroker:           new(int32),
		lastCandleResolution: new(candles.CandleResolution),
		lastPlotTimeRange:    new(PlotTimeRange),
		scalingX:             new(stockval.PlotScaling),
		scalingXmutex:        new(sync.Mutex),
	}
}

func (v *PlotView) Initialize(ctx context.Context, plotData plotData, symbolSearchTool stockapi.SymbolSearchTool, uiUpdater StockUiUpdater, appTradingUrl string) {
	v.AssetData = plotData.Entry
	v.searchField = widgets.NewSearchField(plotData.Entry.Symbol)
	brokerList := make([]string, len(v.brokerList))
	for i, v := range v.brokerList {
		brokerList[i] = string(v)
	}
	brokerIndex := stockval.IndexOf(v.brokerList, plotData.BrokerName)
	if brokerIndex < 0 {
		panic("unknown data broker")
	}
	resolutionList := candles.CandleResolutionUiStringList()
	if int(plotData.CandleResolution) >= len(resolutionList) {
		panic("unknown candle resolution")
	}
	if len(plotData.SubPlots) == 0 {
		panic("missing subplots")
	}

	v.brokerDropdown = widgets.NewDropDown(brokerList, brokerIndex)
	v.resolutionDropDown = widgets.NewDropDown(resolutionList, int(plotData.CandleResolution))
	v.Plot = stockplot.NewPlot(v.PlotTheme, plotData.CandleResolution, plotData.ScalingX, plotData.SubPlots)
	fullAppTradingUrl := fmt.Sprintf(appTradingUrl, plotData.Entry.Symbol)
	v.QuoteField = widgets.NewQuoteField(string(plotData.BrokerName), fullAppTradingUrl)
	v.UiIndex = plotData.UiIndex
	v.uiUpdater = uiUpdater
	v.appTradingUrl = appTradingUrl

	atomic.StoreInt32(v.lastBroker, int32(brokerIndex))
	atomic.StoreInt32((*int32)(v.lastCandleResolution), int32(plotData.CandleResolution))

	// TODO size of buffered channels?
	v.SearchRequestChan = make(chan stockapi.SearchRequest, 10)
	v.SearchResponseChan = make(chan stockapi.SearchResponse, 10)

	go v.handleSearchResult(ctx)
	go symbolSearchTool.FindAsset(ctx, v.SearchRequestChan, v.SearchResponseChan)
}

func (v *PlotView) UpdateSubPlots(subPlots []stockplot.SubPlotData) {
	v.Plot = stockplot.NewPlot(v.PlotTheme, v.GetLastCandleResolution(), v.GetLastPlotScalingX(), subPlots)
}

func (v *PlotView) Cleanup() {
	close(v.SearchRequestChan)
}

func (v *PlotView) saveConfiguration(plotConfig *config.PlotConfig) {
	plotConfig.AssetData = v.AssetData
	plotConfig.BrokerId = v.GetLastBrokerName()
	plotConfig.Resolution = v.GetLastCandleResolution()
	plotConfig.PlotScalingX = v.GetLastPlotScalingX()
}

func (v *PlotView) GetLastBrokerName() stockval.BrokerId {
	return v.brokerList[atomic.LoadInt32(v.lastBroker)]
}

func (v *PlotView) GetLastCandleResolution() candles.CandleResolution {
	return candles.CandleResolution(atomic.LoadInt32((*int32)(v.lastCandleResolution)))
}

func (v *PlotView) GetLastPlotScalingX() stockval.PlotScaling {
	v.scalingXmutex.Lock()
	defer v.scalingXmutex.Unlock()
	return *v.scalingX
}

func (v *PlotView) setLastPlotScalingX(s stockval.PlotScaling) {
	v.scalingXmutex.Lock()
	defer v.scalingXmutex.Unlock()
	*v.scalingX = s
}

func (v *PlotView) handleSearchResult(ctx context.Context) {
	for searchResponse := range v.SearchResponseChan {
		if searchResponse.Error != nil {
			log.Printf("Asset search error: %v", searchResponse.Error)
			continue
		}
		if searchResponse.UnambiguousLookup {
			if len(searchResponse.Result) > 0 {
				if v.AssetData.Figi == searchResponse.Result[0].Figi {
					// Same Figi as already shown. Just update asset data (especially tradable flag).
					// TODO maybe visual feedback?
					newPlotView := *v
					newPlotView.AssetData = searchResponse.Result[0]
					v.uiUpdater.UpdatePlot(v.UiIndex, newPlotView)
				} else {
					v.uiUpdater.RemovePlot(v.AssetData, v.UiIndex)
					v.uiUpdater.AddPlot(
						ctx,
						plotData{
							searchResponse.Result[0],
							v.GetLastCandleResolution(),
							v.GetLastBrokerName(),
							v.UiIndex,
							v.GetLastPlotScalingX(),
							v.Plot.GetSubPlotData(),
						},
						v.appTradingUrl,
					)
					v.uiUpdater.Invalidate()
				}
			}
		} else {
			items := make([]widgets.SearchFieldItem, len(searchResponse.Result))
			for i, r := range searchResponse.Result {
				items[i] = widgets.SearchFieldItem{
					TitleText: r.Symbol,
					DescText:  r.CompanyName,
				}
			}
			v.searchField.SetItems(items)
			v.uiUpdater.Invalidate()
		}
	}
	log.Printf("Terminating search result handler %d.", v.UiIndex)
}

func (v *PlotView) handleInput(ctx context.Context, gtx layout.Context) {
	t, ok := v.searchField.EnteredSearchText()
	if ok && len(t) > 1 {
		v.SearchRequestChan <- stockapi.SearchRequest{RequestId: strconv.Itoa(int(v.UiIndex)), Text: t, MaxNumResults: maxLookupResults}
	}

	t, ok = v.searchField.SubmittedSearchText()
	if ok && t != "" {
		v.SearchRequestChan <- stockapi.SearchRequest{RequestId: strconv.Itoa(int(v.UiIndex)), Text: t, MaxNumResults: maxLookupResults, UnambiguousLookup: true}
	}

	resolutionIndex := v.resolutionDropDown.ClickedIndex()
	if resolutionIndex >= 0 {
		v.resolutionDropDown.SetSelectedIndex(resolutionIndex)
		atomic.StoreInt32((*int32)(v.lastCandleResolution), int32(resolutionIndex))
	}

	brokerIndex := int32(v.brokerDropdown.ClickedIndex())
	if brokerIndex >= 0 {
		if atomic.LoadInt32(v.lastBroker) != brokerIndex {
			// It is safe to do this asynchronously.
			v.uiUpdater.RemovePlot(v.AssetData, v.UiIndex)
			v.uiUpdater.AddPlot(
				ctx,
				plotData{
					v.AssetData,
					v.GetLastCandleResolution(),
					v.brokerList[brokerIndex],
					v.UiIndex,
					v.GetLastPlotScalingX(),
					v.Plot.GetSubPlotData(),
				},
				v.appTradingUrl)
			v.uiUpdater.Invalidate()
		}
	}
	if v.indicatorsButton.Clicked() {
		v.uiUpdater.ShowIndicators(v.UiIndex)
	}
	if v.settingsMenuItem.Clicked() {
		v.uiUpdater.ShowSettings()
	}
}

func (v *PlotView) Layout(ctx context.Context, gtx layout.Context, th *material.Theme, priceData *PriceData) (layout.Dimensions, bool) {
	refreshData := false
	gtx.Constraints.Min = image.Point{} // in order to be able to calculate widget width
	var spy *eventx.Spy
	spy, gtx = eventx.Enspy(gtx)

	v.handleInput(ctx, gtx)

	v.contextMenu.Options = []func(gtx layout.Context) layout.Dimensions{
		component.MenuItem(th, v.settingsMenuItem, "Settings").Layout,
	}
	quote := priceData.GetQuoteCopy()
	bidAsk := priceData.GetBidAskCopy()

	layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceEnd,
			}.Layout(
				gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return v.searchField.Layout(gtx, th, v.Plot.Theme)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							button := material.Button(th, v.indicatorsButton, "Indicators...")
							return layout.Inset{Top: 10, Right: 10, Bottom: 0, Left: 10}.Layout(gtx, button.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 10, Right: 0, Bottom: 0, Left: 10}.Layout(gtx, material.Body1(th, "Resolution:").Layout)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 0, Right: 10, Bottom: 0, Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return v.resolutionDropDown.Layout(th, gtx)
									})
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 10, Right: 0, Bottom: 0, Left: 10}.Layout(gtx, material.Body1(th, "Broker:").Layout)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 0, Right: 20, Bottom: 0, Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return v.brokerDropdown.Layout(th, gtx)
									})
								}),
							)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							resolution := v.GetLastCandleResolution()
							candleResolutionChanged := v.Plot.InitializeFrame(gtx, resolution)
							d := v.Plot.Layout(gtx, th)
							// TODO allow displaying lines instead of candles
							candleUpdater, loaded := priceData.LoadOrAddCandleResolution(ctx, resolution)
							if !loaded || candleResolutionChanged {
								refreshData = true
								v.lastPlotTimeRange.lastPlotStartTime = time.Time{}
								v.lastPlotTimeRange.lastPlotEndTime = time.Time{}
							}
							for _, s := range v.Plot.Sub {
								s.UpdateIndicators(candleUpdater.CandleData)
								s.Plot(candleUpdater.CandleData, quote, gtx, th)
							}
							newScalingX, scalingChanged := v.Plot.GetPlotScalingX()
							if scalingChanged {
								v.setLastPlotScalingX(newScalingX)
							}
							return d
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 5, Top: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{
									Axis:    layout.Vertical,
									Spacing: layout.SpaceEnd,
								}.Layout(
									gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return stockplot.LayoutTitleField(gtx, th, v.PlotTheme, v.AssetData)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Left: 30}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return v.QuoteField.Layout(
												gtx,
												th,
												v.PlotTheme,
												v.AssetData,
												quote,
												bidAsk,
											)
										})
									}),
								)
							})
						}),
					)
				}),
			)
		},
		),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return v.contextMenuArea.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return widgets.NewMenu(th, v.contextMenu).Layout(gtx)
			})
		},
		),
	)

	// TODO This is a hack, this code should be in searchfield.go, but currently cannot be placed there
	// due to gio not supporting event handling in a different way.
	// TODO spy should later be replaced by custom editor key handling.
	for _, evs := range spy.AllEvents() {
		for _, ev := range evs.Items {
			if e, ok := ev.(key.Event); ok {
				if e.State == key.Press {
					v.searchField.HandleKey(e.Name)
				}
			} else if e, ok := ev.(key.FocusEvent); ok {
				v.searchField.HandleFocus(e.Focus)
			}
		}
	}
	// TODO Currently, we need to handle search field input after spying. This should be done before layout.
	v.searchField.HandleInput(gtx)

	return layout.Dimensions{Size: gtx.Constraints.Max}, refreshData
}

// Call in same thread as Layout()
func (v *PlotView) UpdatePlotRange() (startTime time.Time, endTime time.Time, refreshPlot bool) {
	plotStartTime, plotEndTime, r := v.Plot.GetCandleRange()
	// For now, we do not filter requesting future data. Brokers need to do that if required.
	startTime = r.GetNthCandleTime(plotStartTime, -stockval.PreloadCandlesBefore)
	endTime = r.GetNthCandleTime(plotEndTime, stockval.PreloadCandlesAfter)

	// Start refreshing after passing half the refresh interval.
	startRefreshDiff := plotStartTime.Sub(startTime) / 2
	endRefreshDiff := plotEndTime.Sub(endTime) / 2

	if v.lastPlotTimeRange.lastPlotStartTime.IsZero() ||
		v.lastPlotTimeRange.lastPlotEndTime.IsZero() ||
		v.lastPlotTimeRange.lastPlotStartTime.Sub(startTime) > startRefreshDiff ||
		v.lastPlotTimeRange.lastPlotEndTime.Sub(endTime) < endRefreshDiff {

		v.lastPlotTimeRange.lastPlotStartTime = startTime
		v.lastPlotTimeRange.lastPlotEndTime = endTime
		refreshPlot = true
	}
	return
}
