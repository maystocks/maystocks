// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"

	"gioui.org/widget/material"
	"gioui.org/x/component"
)

func NewMenu(th *material.Theme, state *component.MenuState) component.MenuStyle {
	m := component.Menu(th, state)
	m.AmbientColor = th.Palette.ContrastBg
	m.PenumbraColor = color.NRGBA{}
	m.UmbraColor = color.NRGBA{}
	return m
}
