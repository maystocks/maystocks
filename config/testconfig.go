// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

type TestConfig struct {
	appConfig AppConfig
}

// Test configurations are not stored and not thread safe.
// Intended only for use in unit tests.
func NewTestConfig() Config {
	return &TestConfig{
		appConfig: NewAppConfig(),
	}
}

func (t *TestConfig) GetAppName() string {
	return "test"
}

func (t *TestConfig) Lock() (*AppConfig, error) {
	return &t.appConfig, nil
}

func (t *TestConfig) Unlock(c *AppConfig) error {
	return nil
}

func (t *TestConfig) Copy() (AppConfig, error) {
	return t.appConfig, nil
}
