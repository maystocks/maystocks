// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"maystocks/indapi/candles"
	"maystocks/stockval"
)

type PlotConfig struct {
	AssetData     stockval.AssetData
	Resolution    candles.CandleResolution
	BrokerId      stockval.BrokerId
	PlotScalingX  stockval.PlotScaling
	SubPlotConfig []SubPlotConfig
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
			Tradable:              true,
			Class:                 stockval.AssetClassEquity,
		},
		Resolution:    candles.CandleOneDay,
		BrokerId:      "alpaca",
		SubPlotConfig: NewSubPlotConfig(),
	}
}

func (p *PlotConfig) sanitize() {
	// Generate normalized name, this is not stored.
	p.AssetData.CompanyNameNormalized = stockval.NormalizeAssetName(p.AssetData.CompanyName)
	if len(p.SubPlotConfig) == 0 {
		p.SubPlotConfig = NewSubPlotConfig()
	}
}
