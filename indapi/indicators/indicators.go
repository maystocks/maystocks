// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package indicators

import (
	"image/color"
	"maystocks/indapi"
	"maystocks/indapi/indicators/bollinger"
	"maystocks/indapi/indicators/sma"
	"maystocks/indapi/indicators/stochastics"
	"sort"

	"golang.org/x/exp/maps"
)

const DefaultId = "bollinger"

var IndicatorRegistry map[indapi.IndicatorId]func() indapi.IndicatorData = make(map[indapi.IndicatorId]func() indapi.IndicatorData)

func init() {
	IndicatorRegistry[bollinger.Id] = bollinger.NewIndicator
	IndicatorRegistry[sma.Id] = sma.NewIndicator
	IndicatorRegistry[stochastics.Id] = stochastics.NewIndicator
}

func Create(id indapi.IndicatorId, properties map[string]string, colors []color.NRGBA) indapi.IndicatorData {
	d, ok := IndicatorRegistry[id]
	if !ok {
		panic("invalid indicator name")
	}
	ind := d()
	ind.SetProperties(properties)
	ind.SetColors(colors)
	return ind
}

func GetDefaultProperties(id indapi.IndicatorId) map[string]string {
	d, ok := IndicatorRegistry[id]
	if !ok {
		panic("invalid indicator name")
	}
	return d().GetProperties()
}

func GetSubPlotType(id indapi.IndicatorId) indapi.SubPlotType {
	d, ok := IndicatorRegistry[id]
	if !ok {
		panic("invalid indicator name")
	}
	return d().GetSubPlotType()
}

func GetList() indapi.IndicatorList {
	l := indapi.IndicatorList(maps.Keys(IndicatorRegistry))
	sort.Sort(l)
	return l
}
