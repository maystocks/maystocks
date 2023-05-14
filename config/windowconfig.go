// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"image"
	"maystocks/stockval"
)

type WindowConfig struct {
	NumPlots   image.Point
	Size       image.Point `yaml:",omitempty"`
	PlotConfig []PlotConfig
}

func NewWindowConfig() WindowConfig {
	return WindowConfig{
		NumPlots: image.Point{X: 1, Y: 1},
	}
}

func (w *WindowConfig) sanitize() {
	if w.NumPlots.X <= 0 {
		w.NumPlots.X = 1
	}
	if w.NumPlots.Y <= 0 {
		w.NumPlots.Y = 1
	}
	// Update number of plot configurations according to x/y plot count.
	for w.NumPlots.X*w.NumPlots.Y > len(w.PlotConfig) {
		w.PlotConfig = append(w.PlotConfig, NewPlotConfig())
	}
	w.PlotConfig = w.PlotConfig[:w.NumPlots.X*w.NumPlots.Y]
	// Generate normalized name, this is not stored.
	for j := range w.PlotConfig {
		w.PlotConfig[j].AssetData.CompanyNameNormalized = stockval.NormalizeAssetName(w.PlotConfig[j].AssetData.CompanyName)
	}
}
