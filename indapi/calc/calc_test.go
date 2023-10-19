// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package calc

import (
	"log"
	"maystocks/indapi"
	"testing"

	"github.com/ericlagergren/decimal"
	"github.com/stretchr/testify/assert"
)

func TestMean(t *testing.T) {
	d := []indapi.CandleData{
		{ClosePrice: decimal.New(5, 0)},
		{ClosePrice: decimal.New(10, 0)},
		{ClosePrice: decimal.New(15, 0)},
	}
	out := Mean(new(decimal.Big), d)
	value, ok := out.Int64()
	assert.True(t, ok)
	assert.Equal(t, int64(10), value)
}

func TestMeanForStdDev(t *testing.T) {
	d := []indapi.CandleData{
		{ClosePrice: decimal.New(300, 0)},
		{ClosePrice: decimal.New(430, 0)},
		{ClosePrice: decimal.New(170, 0)},
		{ClosePrice: decimal.New(470, 0)},
		{ClosePrice: decimal.New(600, 0)},
	}
	out := Mean(new(decimal.Big), d)
	value, ok := out.Int64()
	assert.True(t, ok)
	assert.Equal(t, int64(394), value)
}

func TestStdDev(t *testing.T) {
	d := []indapi.CandleData{
		{ClosePrice: decimal.New(46, 0)},
		{ClosePrice: decimal.New(69, 0)},
		{ClosePrice: decimal.New(32, 0)},
		{ClosePrice: decimal.New(60, 0)},
		{ClosePrice: decimal.New(52, 0)},
		{ClosePrice: decimal.New(41, 0)},
	}
	out := StdDev(new(decimal.Big), d)
	log.Printf("%f", out)
	assert.Equal(t, 0, out.Quantize(2).CmpTotal(decimal.New(1331, 2)))
}

func TestStdDevPrecise(t *testing.T) {
	d := []indapi.CandleData{
		{ClosePrice: decimal.New(247, 2)},
		{ClosePrice: decimal.New(255, 2)},
		{ClosePrice: decimal.New(251, 2)},
		{ClosePrice: decimal.New(239, 2)},
		{ClosePrice: decimal.New(241, 2)},
		{ClosePrice: decimal.New(247, 2)},
		{ClosePrice: decimal.New(244, 2)},
		{ClosePrice: decimal.New(250, 2)},
		{ClosePrice: decimal.New(246, 2)},
		{ClosePrice: decimal.New(255, 2)},
		{ClosePrice: decimal.New(251, 2)},
		{ClosePrice: decimal.New(232, 2)},
		{ClosePrice: decimal.New(250, 2)},
		{ClosePrice: decimal.New(254, 2)},
		{ClosePrice: decimal.New(251, 2)},
	}
	out := StdDev(new(decimal.Big), d)
	assert.Equal(t, 0, out.Quantize(3).CmpTotal(decimal.New(64, 3)))
}
