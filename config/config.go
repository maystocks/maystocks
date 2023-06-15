// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

type EncryptionSetup interface {
	SetEncryptionPassword(pw string)
	IsEncryptionPassword(pw string) bool
}

type Config interface {
	EncryptionSetup
	GetAppName() string
	Lock() (*AppConfig, error)
	Unlock(c *AppConfig, forceWriting bool) error
	Copy(forceReading bool) (AppConfig, error)
}
