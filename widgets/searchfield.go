// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"image/color"
	"maystocks/indapi/calc"
	"maystocks/stockval"
	"sync"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type SearchFieldItem struct {
	TitleText string
	DescText  string
	click     widget.Clickable
}

type SearchField struct {
	items                    []SearchFieldItem
	nextItems                []SearchFieldItem
	nextItemsMutex           sync.Mutex
	minItemSizeX             int
	selectedIndex            int
	lastHoveredIndex         int
	textField                component.TextField
	list                     widget.List
	ignoreChangeText         string
	submittedSearchText      string
	submittedSearchTextMutex sync.Mutex
	enteredSearchText        string
	enteredSearchTextMutex   sync.Mutex
}

func NewSearchField(text string) *SearchField {
	f := &SearchField{
		selectedIndex:    -1,
		lastHoveredIndex: -1,
		textField: component.TextField{
			Editor: widget.Editor{Submit: true, SingleLine: true, MaxLen: 128},
		},
		list: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		ignoreChangeText: text,
	}
	f.textField.SetText(text)
	return f
}

// Set items from any goroutine.
func (f *SearchField) SetItems(items []SearchFieldItem) {
	f.nextItemsMutex.Lock()
	defer f.nextItemsMutex.Unlock()
	f.nextItems = items
}

// Retrieve last non-retrieved submitted search text from any goroutine.
func (f *SearchField) SubmittedSearchText() (string, bool) {
	f.submittedSearchTextMutex.Lock()
	defer f.submittedSearchTextMutex.Unlock()
	if len(f.submittedSearchText) > 0 {
		t := f.submittedSearchText
		f.submittedSearchText = ""
		return t, true
	}
	return "", false
}

// Retrieve last non-retrieved entered search text from any goroutine.
func (f *SearchField) EnteredSearchText() (string, bool) {
	f.enteredSearchTextMutex.Lock()
	defer f.enteredSearchTextMutex.Unlock()
	if len(f.enteredSearchText) > 0 {
		t := f.enteredSearchText
		f.enteredSearchText = ""
		return t, true
	}
	return "", false
}

func (f *SearchField) handleEvents(gtx layout.Context) {
	for _, evt := range f.textField.Events() {
		switch evt := evt.(type) {
		case widget.ChangeEvent:
			t := f.textField.Text()
			// SetText also may fire this event. We do not want that.
			if f.ignoreChangeText != "" && f.ignoreChangeText == t {
				f.textField.Focus()
				f.ignoreChangeText = ""
			} else {
				f.textField.Focus()
				if t == "" {
					f.resetItems()
				}
				f.enteredSearchTextMutex.Lock()
				f.enteredSearchText = t
				f.enteredSearchTextMutex.Unlock()
			}
		case widget.SubmitEvent:
			f.submitText(evt.Text)
		}
	}
}

func (f *SearchField) submitText(t string) {
	normalizedText := stockval.NormalizeAssetName(t)
	if normalizedText != f.textField.Text() {
		f.textField.SetText(normalizedText)
		f.ignoreChangeText = normalizedText
	}
	f.textField.SetCaret(0, len(normalizedText))
	f.submittedSearchTextMutex.Lock()
	f.submittedSearchText = normalizedText
	f.submittedSearchTextMutex.Unlock()
}

func (f *SearchField) registerInputOps(gtx layout.Context) {
	// non-default keyboard input
	if f.textField.Focused() {
		key.InputOp{
			Tag:  f,
			Keys: key.NameUpArrow + "|" + key.NameDownArrow + "|" + key.NameEscape,
		}.Add(gtx.Ops)
	}
}

func (f *SearchField) resetItems() {
	f.items = nil
	f.selectedIndex = -1
	f.lastHoveredIndex = -1
}

func (f *SearchField) forceShowItems() {
	if len(f.items) == 0 {
		f.enteredSearchTextMutex.Lock()
		f.enteredSearchText = f.textField.Text()
		f.enteredSearchTextMutex.Unlock()
	}
}

func (f *SearchField) updateTextFromSelection() {
	t := f.items[f.selectedIndex].TitleText
	f.ignoreChangeText = t
	f.textField.SetText(t)
	f.textField.SetCaret(0, len(t))
}

// Call from same goroutine as Layout.
func (f *SearchField) HandleInput(gtx layout.Context) {
	for _, gtxEvent := range gtx.Events(f) {
		switch e := gtxEvent.(type) {
		case key.Event:
			if e.State == key.Press {
				f.HandleKey(e.Name)
			}
		}
	}
}

// Call from same goroutine as Layout.
func (f *SearchField) HandleKey(name string) {
	switch name {
	case key.NameUpArrow:
		if len(f.items) == 0 {
			f.resetItems()
		} else {
			f.selectedIndex--
			if f.selectedIndex < 0 {
				f.selectedIndex = len(f.items) - 1
			}
			f.updateTextFromSelection()
		}
		f.forceShowItems()
	case key.NameDownArrow:
		if len(f.items) == 0 {
			f.resetItems()
		} else {
			f.selectedIndex++
			if f.selectedIndex >= len(f.items) {
				f.selectedIndex = 0
			}
			f.updateTextFromSelection()
		}
		f.forceShowItems()
	case key.NameEscape:
		f.resetItems()
	}
}

// Call from same goroutine as Layout.
func (f *SearchField) HandleFocus(focus bool) {
	if focus {
		f.textField.SetCaret(len(f.textField.Text()), 0)
	}
}

func (f *SearchField) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	f.handleEvents(gtx)
	f.registerInputOps(gtx)
	var nextMinItemSizeX int
	var textFieldDims layout.Dimensions

	flexChildren := [2]layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			textFieldDims = f.textField.Layout(gtx, th, "Symbol")
			return textFieldDims
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{
				Alignment: layout.NW,
			}.Layout(
				gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return Frame{InnerMargin: 5, BorderWidth: 1, BorderColor: th.Palette.ContrastBg, BackgroundColor: th.Palette.Bg}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return f.list.List.Layout(gtx, len(f.items), func(gtx layout.Context, index int) layout.Dimensions {
							item := &f.items[index]
							if item.click.Hovered() && index != f.lastHoveredIndex {
								f.selectedIndex = index
								f.lastHoveredIndex = index
							}
							if item.click.Pressed() {
								f.selectedIndex = index
								f.lastHoveredIndex = index
								f.updateTextFromSelection()
								f.submitText(f.textField.Text())
							}
							isSelected := index == f.selectedIndex
							return item.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								// Correct rendering might be one frame delayed, first frame may have invalid selection size.
								gtx.Constraints.Min.X = calc.Max(gtx.Constraints.Min.X, f.minItemSizeX)
								// Record macro only for the selected entry, because a different background is drawn.
								var macro op.MacroOp
								if isSelected {
									macro = op.Record(gtx.Ops)
								}
								dims := layout.Flex{
									Axis: layout.Vertical,
								}.Layout(
									gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										label := material.Label(th, unit.Sp(24), item.TitleText)
										if isSelected {
											// TODO use theme
											label.Color = color.NRGBA{R: 100, G: 255, B: 100, A: 255}
										}
										return label.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										label := material.Label(th, unit.Sp(18), item.DescText)
										if isSelected {
											// TODO use theme
											label.Color = color.NRGBA{R: 100, G: 255, B: 100, A: 255}
										}
										return label.Layout(gtx)
									}))
								if nextMinItemSizeX < dims.Size.X {
									nextMinItemSizeX = dims.Size.X
								}
								if isSelected {
									call := macro.Stop()
									// TODO use theme
									paint.FillShape(gtx.Ops, color.NRGBA{R: 0x4a, G: 0x4c, B: 0x6b, A: 255}, clip.Rect{Max: dims.Size}.Op())
									call.Add(gtx.Ops)
								}
								return dims
							})
						})
					})
				}),
			)
		}),
	}

	expanded := f.textField.Focused() && len(f.items) > 0

	var macro op.MacroOp
	if expanded {
		macro = op.Record(gtx.Ops)
		layout.Flex{
			Axis: layout.Vertical,
		}.Layout(
			gtx,
			flexChildren[:]...,
		)
	} else {
		layout.Flex{
			Axis: layout.Vertical,
		}.Layout(
			gtx,
			flexChildren[0:1]...,
		)
	}
	if expanded {
		op.Defer(gtx.Ops, macro.Stop())
	}
	f.nextItemsMutex.Lock()
	if f.nextItems != nil {
		f.items = f.nextItems
		f.nextItems = nil
		f.selectedIndex = -1
		f.lastHoveredIndex = -1
	}
	f.nextItemsMutex.Unlock()
	if f.minItemSizeX != nextMinItemSizeX {
		f.minItemSizeX = nextMinItemSizeX
		op.InvalidateOp{}.Add(gtx.Ops)
	}
	return textFieldDims
}
