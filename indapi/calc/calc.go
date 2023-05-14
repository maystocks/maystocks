// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package calc

import (
	"maystocks/indapi"

	"github.com/ericlagergren/decimal"
)

func Min(i0, i1 int) int {
	if i0 < i1 {
		return i0
	}
	return i1
}

func Max(i0, i1 int) int {
	if i0 > i1 {
		return i0
	}
	return i1
}

func Mean(out *decimal.Big, val []indapi.CandleData) *decimal.Big {
	out.SetUint64(0)
	for i := range val {
		out.Add(out, val[i].ClosePrice)
	}
	out.Quo(out, new(decimal.Big).SetUint64(uint64(len(val))))
	return out
}

func StdDev(out *decimal.Big, val []indapi.CandleData) *decimal.Big {
	out.SetUint64(0)
	if len(val) == 0 {
		return out
	}
	m := Mean(new(decimal.Big), val)
	for i := 0; i < len(val); i++ {
		v := new(decimal.Big).Copy(val[i].ClosePrice)
		v.Sub(v, m)
		v.Mul(v, v)
		out.Add(out, v)
	}
	out.Quo(out, new(decimal.Big).SetUint64(uint64(len(val))))
	return out.Context.Sqrt(out, out)
}
