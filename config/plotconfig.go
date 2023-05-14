// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"maystocks/indapi/candles"
	"maystocks/stockval"
)

type PlotConfig struct {
	AssetData    stockval.AssetData
	Resolution   candles.CandleResolution
	BrokerId     stockval.BrokerId
	Indicators   []IndicatorConfig
	PlotScalingX stockval.PlotScaling
}

// Returns some valid default plot data. Make sure the broker is available.
func NewPlotConfig() PlotConfig {
	return PlotConfig{
		AssetData: stockval.AssetData{
			Figi:                  "BBG000BDTBL9",
			Symbol:                "SPY",
			Currency:              "USD",
			Mic:                   "ARCX",
			CompanyName:           "SPDR S&P 500 ETF TRUST",
			CompanyNameNormalized: stockval.NormalizeAssetName("SPDR S&P 500 ETF TRUST"),
			Tradable:              false,
		},
		Resolution: candles.CandleOneDay,
		BrokerId:   "finnhub",
		Indicators: []IndicatorConfig{
			{IndicatorId: "bollinger", Properties: make(map[string]string)},
		},
	}
}
