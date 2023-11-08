// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package stockval

import (
	"math"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"gioui.org/unit"
	"github.com/ericlagergren/decimal"
)

const DefaultExchange = "US" // TODO support other countries

var IsinRegex = regexp.MustCompile(`^([A-Z]{2})([A-Z0-9]{9})[0-9]$`)

type AquisitionEntry struct {
	Id            string
	Figi          string
	Quantity      int32
	PurchasePrice *decimal.Big
	PurchaseDate  time.Time
}

type AssetData struct {
	Figi                  string
	Symbol                string
	Isin                  string `yaml:",omitempty"`
	Currency              string
	Mic                   string
	CompanyName           string
	CompanyNameNormalized string `yaml:"-"`
	Tradable              bool   `yaml:",omitempty"`
}

type PlotScaling struct {
	Grid      unit.Dp
	ValueGrid float64
}

type SubPlotType int

const (
	SubPlotTypePrice SubPlotType = iota
	SubPlotTypeVolume
	SubPlotTypeIndicator
)

// Limit display name size
var displayNameRegex = regexp.MustCompile(`^.{0,48}`)

var alphanumericRegex = regexp.MustCompile(`[^\p{L}\p{N} ]+`)

func NormalizeAssetName(n string) string {
	return strings.TrimSpace(strings.ToUpper(alphanumericRegex.ReplaceAllString(n, "")))
}

func TruncateDisplayName(n string) string {
	return displayNameRegex.FindString(n)
}

type BrokerId string

// For sorting
type BrokerList []BrokerId

func (x BrokerList) Len() int           { return len(x) }
func (x BrokerList) Less(i, j int) bool { return x[i] < x[j] }
func (x BrokerList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func AtomicSwapFloat64(addr *float64, new float64) float64 {
	return math.Float64frombits(atomic.SwapUint64((*uint64)(unsafe.Pointer(addr)), math.Float64bits(new)))
}

func AtomicStoreFloat64(addr *float64, new float64) {
	atomic.StoreUint64((*uint64)(unsafe.Pointer(addr)), math.Float64bits(new))
}

func CountDigits(v int64) int {
	var count int
	for ; v != 0; v /= 10 {
		count++
	}
	return count
}

func IndexOf[T comparable](s []T, e T) int {
	for i, v := range s {
		if v == e {
			return i
		}
	}
	return -1
}
