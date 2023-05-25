// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package initapp

import (
	"context"
	"errors"
	"log"
	"maystocks/brokers/alpaca"
	"maystocks/brokers/finnhub"
	"maystocks/brokers/openfigi"
	"maystocks/config"
	"maystocks/stockapi"
	"maystocks/stockval"
	"maystocks/stockviz"
	"maystocks/widgets"
	"os"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type initUiState int

const (
	StateConfirmLicense initUiState = iota
	StateInitialSettings
)

type InitApp struct {
	initWindow       *app.Window
	config           config.Config
	stockRequester   map[stockval.BrokerId]stockapi.StockValueRequester
	defaultBroker    stockval.BrokerId
	licenseConfirmed bool
	uiState          initUiState
	licenseView      *widgets.LicenseView
	configView       *widgets.ConfigView
}

func NewInitApp(c config.Config) *InitApp {
	return &InitApp{
		stockRequester: make(map[stockval.BrokerId]stockapi.StockValueRequester),
		config:         c,
		licenseView:    widgets.NewLicenseView(),
		configView:     widgets.NewConfigView(config.NewBrokerConfigMap()),
	}
}

func (a *InitApp) Initialize(licenseText string) {
	a.licenseView.SetText(licenseText)
}

func (a *InitApp) reloadConfiguration() error {
	appConfig, err := a.config.Copy()
	if err != nil {
		return err
	}
	a.licenseConfirmed = appConfig.LicenseConfirmed

	if !openfigi.IsValidConfig(a.config) {
		return errors.New("missing openfigi configuration")
	}
	figiRequester := openfigi.NewRequester()
	err = figiRequester.ReadConfig(a.config)
	if err != nil {
		return err
	}
	if alpaca.IsValidConfig(a.config) {
		r := alpaca.NewStockRequester(figiRequester)
		err = r.ReadConfig(a.config)
		if err != nil {
			return err
		}
		a.stockRequester[alpaca.GetBrokerId()] = r
		a.defaultBroker = alpaca.GetBrokerId()
	}
	if finnhub.IsValidConfig(a.config) {
		r := finnhub.NewStockRequester(figiRequester)
		err = r.ReadConfig(a.config)
		if err != nil {
			return err
		}
		a.stockRequester[finnhub.GetBrokerId()] = r
		a.defaultBroker = finnhub.GetBrokerId()
	}
	a.configView.SetBrokerConfig(&appConfig)
	a.configView.SetWindowConfig(&appConfig)

	if len(a.stockRequester) == 0 {
		return errors.New("no broker was configured")
	}

	return nil
}

func (a *InitApp) saveConfiguration() error {
	appConfig, err := a.config.Lock()
	if err != nil {
		return err
	}
	appConfig.LicenseConfirmed = a.licenseConfirmed
	a.configView.GetWindowConfig(appConfig)
	a.configView.GetBrokerConfig(appConfig)
	return a.config.Unlock(appConfig)
}

func (a *InitApp) Run(ctx context.Context) {
	err := a.reloadConfiguration()
	// Show initialization window only if initial configuration is missing.
	if err != nil || !a.licenseConfirmed {
		a.createWindows()
		err = a.handleEvents(ctx)
		if err != nil {
			log.Printf("terminating with error: %v", err)
		}
		a.terminate()
		err = a.reloadConfiguration()
		if err != nil {
			log.Fatalf("initialization failed: %v", err)
		}
		if !a.licenseConfirmed {
			log.Fatal("initialization failed: license not confirmed")
		}
	}

	// Start main app after initial configuration.
	s := stockviz.NewStockApp(a.config)
	err = s.Initialize(ctx, a.stockRequester, a.defaultBroker)
	if err != nil {
		log.Fatalf("app initialization failed: %v", err)
	}
	s.Run(ctx)

	os.Exit(0)
}

func (a *InitApp) createWindows() {
	a.initWindow = app.NewWindow(
		app.Title(a.config.GetAppName()),
		app.Size(unit.Dp(640), unit.Dp(480)),
	)
	a.initWindow.Perform(system.ActionCenter)
}

func (a *InitApp) handleEvents(ctx context.Context) error {
	var ops op.Ops
	th := widgets.NewDarkMaterialTheme()

	for e := range a.initWindow.Events() {
		switch e := e.(type) {
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			paint.Fill(gtx.Ops, th.Bg)
			switch a.uiState {
			case StateConfirmLicense:
				a.licenseView.Layout(th, gtx)
				if a.licenseView.ConfirmClicked() {
					a.uiState = StateInitialSettings
				}
				if a.licenseView.CancelClicked() {
					a.initWindow.Perform(system.ActionClose)
				}
			default:
				a.configView.Layout(th, gtx)
				if a.configView.ConfirmClicked() {
					a.licenseConfirmed = true
					a.initWindow.Perform(system.ActionClose)
				}
			}
			e.Frame(gtx.Ops)
		case system.DestroyEvent:
			return e.Err
		}
	}
	return nil
}

func (a *InitApp) terminate() {
	err := a.saveConfiguration()
	if err != nil {
		log.Printf("error saving configuration: %v", err)
	}
}
