// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"log"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/inkeliz/giohyperlink"
)

type LinkButton struct {
	linkTarget string
	linkText   string
	button     widget.Clickable
}

func (l *LinkButton) SetUrl(url string, text string) {
	l.linkTarget = url
	if len(text) > 0 {
		l.linkText = text
	} else {
		l.linkText = url
	}
	l.linkText += "  »"
}

func (l *LinkButton) Url() string {
	return l.linkTarget
}

func (l *LinkButton) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	if l.button.Clicked() {
		if err := giohyperlink.Open(l.linkTarget); err != nil {
			log.Printf("error: opening link: %v", err)
		}
	}
	if l.button.Hovered() {
		pointer.CursorPointer.Add(gtx.Ops)
	}

	button := material.Button(th, &l.button, l.linkText)
	return layout.Inset{Top: 10, Right: 1, Bottom: 0, Left: 1}.Layout(gtx, button.Layout)

}
