// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"image/color"
	"maystocks/indapi"
)

type IndicatorConfig struct {
	IndicatorId indapi.IndicatorId
	Properties  map[string]string
	Colors      []color.NRGBA
}
