// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"
	"maystocks/fonts/noto"

	"gioui.org/widget/material"
)

func NewDarkMaterialTheme() *material.Theme {
	th := material.NewTheme(noto.Collection())
	th.Bg = color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 255} // https://m2.material.io/design/color/dark-theme.html#properties
	th.Fg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	th.ContrastFg = th.Fg
	return th
}

func NewLightMaterialTheme() *material.Theme {
	th := material.NewTheme(noto.Collection())
	return th
}
