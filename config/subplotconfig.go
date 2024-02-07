// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"maystocks/indapi"
)

type SubPlotConfig struct {
	Type       indapi.SubPlotType
	Indicators []IndicatorConfig
}

func NewSubPlotConfig() []SubPlotConfig {
	return []SubPlotConfig{
		NewSubPlotPriceConfig(),
		NewSubPlotVolumeConfig(),
	}
}

// Returns some valid default plot data.
func NewSubPlotPriceConfig() SubPlotConfig {
	return SubPlotConfig{
		Type: indapi.SubPlotTypePrice,
		Indicators: []IndicatorConfig{
			{IndicatorId: "bollinger", Properties: make(map[string]string)},
		},
	}
}

func NewSubPlotVolumeConfig() SubPlotConfig {
	return SubPlotConfig{
		Type: indapi.SubPlotTypeVolume,
	}
}

func NewSubPlotIndicatorConfig() SubPlotConfig {
	return SubPlotConfig{
		Type: indapi.SubPlotTypeIndicator,
	}
}
