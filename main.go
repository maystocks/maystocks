// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package main

import (
	"context"
	_ "embed"
	"maystocks/config"
	"maystocks/initapp"

	"gioui.org/app"
)

//go:embed LICENSE
var license string

func main() {
	c := config.NewGlobalConfig()
	a := initapp.NewInitApp(c)
	a.Initialize(license)
	go a.Run(context.Background())
	app.Main()
}
