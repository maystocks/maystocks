// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package widgets

import (
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type DropDownItem struct {
	Text       string
	ItemButton *widget.Clickable
}

type DropDown struct {
	items         []DropDownItem
	selectedIndex int
	clickedIndex  int
	menu          component.MenuState
	button        widget.Clickable
	toggled       bool
}

func NewDropDown(items []string, selectedIndex int) *DropDown {
	d := DropDown{
		selectedIndex: selectedIndex,
		clickedIndex:  -1,
	}
	d.items = make([]DropDownItem, len(items))
	for i, t := range items {
		d.items[i] = DropDownItem{Text: t, ItemButton: new(widget.Clickable)}
	}
	return &d
}

// Retrieve index of the last selected entry. Call from same goroutine as Layout.
// Returns -1 if nothing has been selected since the last call.
func (d *DropDown) ClickedIndex() int {
	c := d.clickedIndex
	d.clickedIndex = -1
	return c
}

// Set the currently selected item. Call from same goroutine as Layout.
func (d *DropDown) SetSelectedIndex(index int) {
	d.selectedIndex = index
}

func (d *DropDown) Layout(th *material.Theme, gtx layout.Context) layout.Dimensions {
	// Handle menu selection.
	d.menu.Options = d.menu.Options[:0]
	for i, m := range d.items {
		if m.ItemButton.Pressed() && d.toggled {
			d.clickedIndex = i
			d.toggled = false
			gtx.Execute(op.InvalidateCmd{})
		}
		d.menu.Options = append(d.menu.Options, component.MenuItem(th, m.ItemButton, m.Text).Layout)
	}
	if d.button.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: &d.button})
		d.toggled = !d.toggled
		gtx.Execute(op.InvalidateCmd{})
	} else {
		focused := gtx.Focused(&d.button)
		if !focused && d.toggled {
			d.toggled = false
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	var buttonDims layout.Dimensions
	flexChildren := [2]layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var buttonText string
			if d.selectedIndex >= 0 && d.selectedIndex < len(d.items) {
				buttonText = d.items[d.selectedIndex].Text
			}
			button := material.Button(th, &d.button, buttonText)
			buttonDims = layout.Inset{Top: 10, Right: 1, Bottom: 0, Left: 1}.Layout(gtx, button.Layout)
			return buttonDims
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{
				Alignment: layout.N,
			}.Layout(
				gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 2}.Layout(gtx, NewMenu(th, &d.menu).Layout)
				}),
			)
		})}
	expanded := d.toggled && len(d.items) > 0

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

	return buttonDims
}
