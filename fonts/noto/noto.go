// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package noto

import (
	_ "embed"
	"log"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

var once sync.Once
var collection []font.FontFace

//go:embed NotoSans-Regular.ttf
var notoSansRegularTtf []byte

//go:embed NotoSans-Bold.ttf
var notoSansBoldTtf []byte

//go:embed NotoSans-Light.ttf
var notoSansLightTtf []byte

//go:embed NotoSans-Medium.ttf
var notoSansMediumTtf []byte

func Collection() []font.FontFace {
	once.Do(func() {
		register(font.Font{Typeface: "Noto Sans"}, notoSansRegularTtf)
		register(font.Font{Typeface: "Noto Sans", Weight: font.Bold}, notoSansBoldTtf)
		register(font.Font{Typeface: "Noto Sans", Weight: font.Light}, notoSansLightTtf)
		register(font.Font{Typeface: "Noto Sans", Weight: font.Medium}, notoSansMediumTtf)
		n := len(collection)
		collection = collection[:n:n]
	})
	return collection
}

func register(f font.Font, ttf []byte) {
	face, err := opentype.Parse(ttf)
	if err != nil {
		log.Panicf("failed to parse font: %v", err)
	}
	collection = append(collection, font.FontFace{Font: f, Face: face})
}
