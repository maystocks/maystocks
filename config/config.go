// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

type Config interface {
	GetAppName() string
	SetEncryptionPassword(pw string)
	Lock() (*AppConfig, error)
	Unlock(c *AppConfig, forceWriting bool) error
	Copy(forceReading bool) (AppConfig, error)
}
