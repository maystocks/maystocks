// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type LicenseView struct {
	license               string
	lines                 []string
	visibleLineCount      int
	lineStates            []widget.Selectable
	textList              widget.List
	buttonPageUp          widget.Clickable
	buttonPageDown        widget.Clickable
	buttonClipboard       widget.Clickable
	buttonCancel          widget.Clickable
	buttonContinue        widget.Clickable
	cbLicenseConfirmed    widget.Bool
	highlightConfirmation bool
	confirmed             bool
	cancelled             bool
	Margin                unit.Dp
}

func NewLicenseView() *LicenseView {
	return &LicenseView{
		textList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		Margin: DefaultMargin,
	}
}

// Call from same goroutine as Layout
func (v *LicenseView) SetText(license string) {
	v.license = license
	v.lines = strings.Split(license, "\n")
	v.lineStates = make([]widget.Selectable, len(v.lines))
}

// Call from same goroutine as Layout
func (v *LicenseView) ConfirmClicked() bool {
	c := v.confirmed
	v.confirmed = false
	return c
}

// Call from same goroutine as Layout
func (v *LicenseView) CancelClicked() bool {
	c := v.cancelled
	v.cancelled = false
	return c
}

func (v *LicenseView) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	// TODO this is all kind of hacked.
	// Placing a single editor in a list will always layout all lines of the editor, which causes slow scrolling.
	// Just using an editor is kind of bad, because there is no scrollbar at this time,
	// and custom scrolling by page is not exposed.

	// Workaround: Layout each line. We also need custom pageup/pagedown handlers.
	// Hopefully, all of this can later be replaced by an editor with an integrated scrollbar.
	key.InputOp{
		Tag:  v,
		Keys: key.NamePageUp + "|" + key.NamePageDown + "|" + key.NameUpArrow + "|" + key.NameDownArrow,
	}.Add(gtx.Ops)

	for _, gtxEvent := range gtx.Events(v) {
		switch e := gtxEvent.(type) {
		case key.Event:
			if e.State == key.Press {
				switch e.Name {
				case key.NamePageUp:
					v.scrollPages(-1)
				case key.NamePageDown:
					v.scrollPages(1)
				case key.NameUpArrow:
					v.textList.ScrollBy(-1)
				case key.NameDownArrow:
					v.textList.ScrollBy(1)
				}
			}
		}
	}

	if v.buttonPageUp.Clicked(gtx) {
		v.scrollPages(-1)
	}
	if v.buttonPageDown.Clicked(gtx) {
		v.scrollPages(1)
	}
	if v.buttonClipboard.Clicked(gtx) {
		clipboard.WriteOp{
			Text: v.license,
		}.Add(gtx.Ops)
	}
	if v.buttonCancel.Clicked(gtx) {
		v.cancelled = true
	}
	if v.cbLicenseConfirmed.Update(gtx) {
		v.highlightConfirmation = false
	}
	if v.buttonContinue.Clicked(gtx) {
		if v.cbLicenseConfirmed.Value {
			v.confirmed = true
		} else {
			v.highlightConfirmation = true
		}
	}

	var visibleLineCount int

	dims := layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return Frame{InnerMargin: v.Margin / 2, OuterMargin: v.Margin, BorderWidth: 1, BorderColor: th.Palette.ContrastBg, BackgroundColor: th.Palette.Bg}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.List(th, &v.textList).Layout(gtx, len(v.lines), func(gtx layout.Context, index int) layout.Dimensions {
					visibleLineCount++
					l := material.Body1(th, v.lines[index])
					l.State = &v.lineStates[index]
					return l.Layout(gtx)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: v.Margin, Bottom: v.Margin, Left: v.Margin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: v.Margin}.Layout(gtx, material.Button(th, &v.buttonPageDown, "Page Down").Layout)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: v.Margin}.Layout(gtx, material.Button(th, &v.buttonPageUp, "Page Up").Layout)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: v.Margin}.Layout(gtx, material.Button(th, &v.buttonClipboard, "Copy to clipboard").Layout)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: v.Margin, Bottom: v.Margin, Left: v.Margin}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Spacing: layout.SpaceBetween,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &v.buttonCancel, "Cancel").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						var borderColor color.NRGBA
						if v.highlightConfirmation {
							borderColor = color.NRGBA{R: 255, A: 255}
						} else {
							borderColor = th.Palette.Bg
						}
						return Frame{InnerMargin: 1, BorderWidth: 1, BorderColor: borderColor}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.CheckBox(th, &v.cbLicenseConfirmed, "I have read and accept the license agreement").Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &v.buttonContinue, "Continue").Layout(gtx)
					}),
				)
			})
		},
		))

	v.visibleLineCount = visibleLineCount
	return dims
}

func (v *LicenseView) scrollPages(pageDelta int) {
	lineDelta := v.visibleLineCount - 4 // Number derived from experimenting.
	if lineDelta < 1 {
		lineDelta = 1
	}
	v.textList.ScrollBy(float32(pageDelta * lineDelta))
}
