// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package cache

import (
	"maystocks/stockval"
	"strings"
)

type AssetList []stockval.AssetData

func (x AssetList) Len() int           { return len(x) }
func (x AssetList) Less(i, j int) bool { return x[i].Symbol < x[j].Symbol }
func (x AssetList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// Searches for corresponding entries with the following priorities:
// Exact symbol or figi matches are always preferred
// Symbol prefix matches are next
// Company name prefix matches are next
// Company name substring matches are last
func (l AssetList) Find(t string, maxNum int, unambiguousLookup bool) AssetList {
	if len(t) == 0 {
		return AssetList{}
	}
	t = stockval.NormalizeAssetName(t)
	var result AssetList
	// Exact id matches
	for _, a := range l {
		if a.Symbol == t || a.Figi == t {
			// exact match should be first result
			result = AssetList{a}
			break
		}
	}
	if unambiguousLookup {
		return result
	}
	// Symbol prefix matches
	for _, a := range l {
		if strings.HasPrefix(a.Symbol, t) {
			result = appendIfNotDuplicate(result, a, maxNum)
		}
	}
	// Company name prefix matches
	for _, a := range l {
		if strings.HasPrefix(a.CompanyNameNormalized, t) {
			result = appendIfNotDuplicate(result, a, maxNum)
		}
	}
	// Company name substring matches
	for _, a := range l {
		if strings.Contains(a.CompanyNameNormalized, t) {
			result = appendIfNotDuplicate(result, a, maxNum)
		}
	}
	return result
}

func appendIfNotDuplicate(l AssetList, a stockval.AssetData, maxNum int) AssetList {
	if maxNum > 0 && len(l) >= maxNum {
		return l
	}
	for _, o := range l {
		if o == a {
			return l
		}
	}
	return append(l, a)
}
