// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import "maystocks/stockval"

type SubPlotConfig struct {
	Type       stockval.SubPlotType
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
		Type: stockval.SubPlotTypePrice,
		Indicators: []IndicatorConfig{
			{IndicatorId: "bollinger", Properties: make(map[string]string)},
		},
	}
}

func NewSubPlotVolumeConfig() SubPlotConfig {
	return SubPlotConfig{
		Type: stockval.SubPlotTypeVolume,
	}
}
