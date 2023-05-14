// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

type Config interface {
	GetAppName() string
	Lock() (*AppConfig, error)
	Unlock(c *AppConfig) error
	Copy() (AppConfig, error)
}
