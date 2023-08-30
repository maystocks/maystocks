// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package cache

import (
	"context"
	"maystocks/stockval"
)

type AssetCache interface {
	GetAssetList(ctx context.Context, req func(ctx context.Context) ([]stockval.AssetData, error)) AssetList
}
