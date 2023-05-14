// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package calc

import (
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
		{ClosePrice: decimal.New(300, 0)},
		{ClosePrice: decimal.New(430, 0)},
		{ClosePrice: decimal.New(170, 0)},
		{ClosePrice: decimal.New(470, 0)},
		{ClosePrice: decimal.New(600, 0)},
	}
	out := StdDev(new(decimal.Big), d)
	value, ok := out.Int64()
	assert.True(t, ok)
	assert.Equal(t, int64(147), value)
}
