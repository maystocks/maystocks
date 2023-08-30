// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package mock

import "maystocks/config"

type TestConfig struct {
	appConfig config.AppConfig
}

// Test configurations are not stored and not thread safe.
// Intended only for use in unit tests.
func NewTestConfig() config.Config {
	return &TestConfig{
		appConfig: config.NewAppConfig(),
	}
}

func (t *TestConfig) GetAppName() string {
	return "test"
}

func (g *TestConfig) SetEncryptionPassword(pw string) {
	// Not used with test config
}

func (g *TestConfig) IsEncryptionPassword(pw string) bool {
	// Not used with test config
	return false
}

func (t *TestConfig) Lock() (*config.AppConfig, error) {
	return &t.appConfig, nil
}

func (t *TestConfig) Unlock(c *config.AppConfig, forceWriting bool) error {
	return nil
}

func (t *TestConfig) Copy(forceReading bool) (config.AppConfig, error) {
	return t.appConfig, nil
}
