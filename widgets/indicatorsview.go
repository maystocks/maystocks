// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"
	"maystocks/config"
	"maystocks/indapi"
	"maystocks/indapi/indicators"
	"maystocks/stockval"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"golang.org/x/image/colornames"
)

type IndicatorView struct {
	config.IndicatorConfig
	dropDownIndicator  *DropDown
	buttonRemove       widget.Clickable
	colorTextField     component.TextField
	propertyKeys       []string
	propertyChildren   []layout.FlexChild
	propertyTextFields map[string]*component.TextField
}

type IndicatorsView struct {
	configList      widget.List
	buttonContinue  widget.Clickable
	buttonAdd       widget.Clickable
	buttonClose     widget.Clickable
	confirmed       bool
	Margin          unit.Dp
	TextMargin      unit.Dp
	ItemMargin      unit.Dp
	configChildren  []layout.FlexChild
	indicatorConfig [][]IndicatorView
	indicatorsList  []string
}

func NewIndicatorsView() *IndicatorsView {
	v := IndicatorsView{
		configList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		Margin:     DefaultMargin,
		TextMargin: DefaultMargin * 2,
		ItemMargin: 50,
	}
	indList := indicators.GetList()
	v.indicatorsList = make([]string, len(indList))
	for i, ind := range indList {
		v.indicatorsList[i] = string(ind)
	}
	return &v
}

func (v *IndicatorsView) GetIndicatorConfig(appConfig *config.AppConfig) {
	for i := range v.indicatorConfig {
		// Clear existing config.
		for s := range appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig {
			appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators = appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators[:0]
		}
		// Add configuration according to ui.
		for j := range v.indicatorConfig[i] {
			for s := range appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig {
				if appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Type == indicators.GetSubPlotType(v.indicatorConfig[i][j].IndicatorId) {
					appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators =
						append(
							appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators,
							v.indicatorConfig[i][j].IndicatorConfig,
						)
				}
			}
		}
		// There may be additional or removed properties for indicators, we need to merge the maps.
		for s := range appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig {
			for k := range appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators {
				configProperties := appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators[k].Properties
				// Use default properties as starting point, assign values only for these default properties.
				appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators[k].Properties =
					indicators.GetDefaultProperties(appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators[k].IndicatorId)
				for key, value := range configProperties {
					if _, ok := appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators[k].Properties[key]; ok {
						appConfig.WindowConfig[0].PlotConfig[i].SubPlotConfig[s].Indicators[k].Properties[key] = value
					}
				}
			}
		}
	}
}

func (v *IndicatorsView) SetIndicatorConfig(appConfig *config.AppConfig) {
	v.indicatorConfig = v.indicatorConfig[:0]
	for i, p := range appConfig.WindowConfig[0].PlotConfig {
		v.indicatorConfig = append(v.indicatorConfig, make([]IndicatorView, 0, 8))
		for s := range p.SubPlotConfig {
			for _, ind := range p.SubPlotConfig[s].Indicators {
				newView := v.createIndicator(ind)
				v.indicatorConfig[i] = append(v.indicatorConfig[i], newView)
			}
		}
	}
}

func (v *IndicatorsView) createIndicator(ind config.IndicatorConfig) IndicatorView {
	indicatorIndex := stockval.IndexOf(v.indicatorsList, string(ind.IndicatorId))
	if indicatorIndex < 0 {
		panic("unknown data broker")
	}
	newView := IndicatorView{
		IndicatorConfig:    ind,
		dropDownIndicator:  NewDropDown(v.indicatorsList, indicatorIndex),
		propertyTextFields: make(map[string]*component.TextField),
	}
	var colorName string
	for name, value := range colornames.Map {
		// no alpha, directly convert
		if value == color.RGBA(ind.Color) {
			colorName = name
			break
		}
	}
	newView.colorTextField.SetText(colorName)
	for key, value := range ind.Properties {
		t := component.TextField{Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128}}
		t.SetText(value)
		newView.propertyTextFields[key] = &t
		newView.propertyKeys = append(newView.propertyKeys, key)
	}
	sort.Strings(newView.propertyKeys)
	return newView
}

// Call from same goroutine as Layout
func (v *IndicatorsView) ConfirmClicked() bool {
	c := v.confirmed
	v.confirmed = false
	return c
}

func (v *IndicatorsView) Layout(th *material.Theme, gtx layout.Context, plotIndex int) layout.Dimensions {
	v.handleInput(gtx, plotIndex)
	return layoutConfirmationFrame(th, v.Margin, gtx, &v.buttonContinue, nil, &v.buttonClose, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &v.configList).Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
			v.configChildren = v.configChildren[:0]
			v.configChildren = append(v.configChildren,
				layout.Rigid(heading(th, "Indicator Settings").Layout),
			)
			for i := range v.indicatorConfig[plotIndex] {
				v.configChildren = v.appendIndicatorLayout(
					th,
					gtx,
					v.indicatorConfigChild(th, &v.indicatorConfig[plotIndex][i]),
					v.configChildren)
			}
			v.configChildren = v.appendIndicatorLayout(
				th,
				gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: v.ItemMargin, Left: v.ItemMargin}.Layout(gtx, material.Button(th, &v.buttonAdd, "Add").Layout)
						}),
					)
				}),
				v.configChildren)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, v.configChildren...)
		},
		)
	})
}

func (v *IndicatorsView) handleInput(gtx layout.Context, plotIndex int) {
	invalidate := false
	if v.buttonContinue.Clicked(gtx) || v.buttonClose.Clicked(gtx) {
		if v.validate(plotIndex) {
			for i := range v.indicatorConfig[plotIndex] {
				for _, key := range v.indicatorConfig[plotIndex][i].propertyKeys {
					v.indicatorConfig[plotIndex][i].IndicatorConfig.Properties[key] = strings.TrimSpace(v.indicatorConfig[plotIndex][i].propertyTextFields[key].Text())
				}
				var nrgba color.NRGBA
				c, ok := colornames.Map[strings.ToLower(strings.Replace(v.indicatorConfig[plotIndex][i].colorTextField.Text(), " ", "", -1))]
				if ok {
					// no alpha, simply assign.
					nrgba = color.NRGBA(c)
				}
				v.indicatorConfig[plotIndex][i].IndicatorConfig.Color = nrgba
			}
			v.confirmed = true
			invalidate = true
		}
	}
	if v.buttonAdd.Clicked(gtx) {
		defaultData := indicators.Create(indicators.DefaultId, nil, color.NRGBA{})
		newView := v.createIndicator(config.IndicatorConfig{IndicatorId: defaultData.GetId(), Properties: defaultData.GetProperties()})
		v.indicatorConfig[plotIndex] = append(v.indicatorConfig[plotIndex], newView)
		invalidate = true
	}
	for i := range v.indicatorConfig[plotIndex] {
		if v.indicatorConfig[plotIndex][i].buttonRemove.Clicked(gtx) {
			// Remove indicator.
			v.indicatorConfig[plotIndex] = append(v.indicatorConfig[plotIndex][:i], v.indicatorConfig[plotIndex][i+1:]...)
			invalidate = true
			break // we changed the list, ignore further input for this frame
		}
		clickedIndicator := v.indicatorConfig[plotIndex][i].dropDownIndicator.ClickedIndex()
		if clickedIndicator >= 0 {
			newData := indicators.Create(indapi.IndicatorId(v.indicatorsList[clickedIndicator]), nil, color.NRGBA{})
			v.indicatorConfig[plotIndex][i] =
				v.createIndicator(config.IndicatorConfig{IndicatorId: newData.GetId(), Properties: newData.GetProperties()})
			invalidate = true
		}
	}
	if invalidate {
		op.InvalidateOp{}.Add(gtx.Ops)
	}
}

func (v *IndicatorsView) validate(plotIndex int) bool {
	for i := range v.indicatorConfig[plotIndex] {
		if !v.indicatorConfig[plotIndex][i].IsValid() {
			return false
		}
	}
	return true
}

func (v *IndicatorsView) layoutConfigEntry(th *material.Theme, gtx layout.Context, ind *IndicatorView, w layout.Widget) layout.Dimensions {
	return layoutLeftRightWidgets(
		th,
		v.Margin,
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: v.ItemMargin, Right: v.TextMargin, Bottom: v.TextMargin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ind.dropDownIndicator.Layout(th, gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Body1(th, "Color:").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: v.TextMargin, Left: v.TextMargin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ind.colorTextField.Layout(gtx, th, "")
					})
				}),
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: v.TextMargin}.Layout(gtx, w)
		},
	)
}

func (v *IndicatorsView) appendIndicatorLayout(th *material.Theme, gtx layout.Context, configChild layout.FlexChild, children []layout.FlexChild) []layout.FlexChild {
	children = append(children, layout.Rigid(divider(th, v.Margin).Layout))
	children = append(children, configChild)
	return children
}

func (v *IndicatorsView) indicatorConfigChild(th *material.Theme, ind *IndicatorView) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return v.layoutConfigEntry(th, gtx, ind, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					ind.propertyChildren = ind.propertyChildren[:0]
					for _, key := range ind.propertyKeys {
						ind.propertyChildren = append(ind.propertyChildren, v.propertyConfigChild(th, ind, key))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, ind.propertyChildren...)
				}),
				layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
					button := material.Button(th, &ind.buttonRemove, "Remove")
					return button.Layout(gtx)
				}),
			)
		})
	})
}

func (v *IndicatorsView) propertyConfigChild(th *material.Theme, ind *IndicatorView, key string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(th, key+":").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(v.Margin).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return ind.propertyTextFields[key].Layout(gtx, th, "Value")
				})
			}),
		)
	})
}

func (b *IndicatorView) IsValid() bool {
	return true
}
