// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockplot

type EventArea int

const (
	EventAreaPlot EventArea = iota
	EventAreaXaxis
	EventAreaYaxis
	EventAreaPlotWindow
)
