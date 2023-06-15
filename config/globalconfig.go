// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/crypto/argon2"
	"gopkg.in/yaml.v3"
)

const AppName = "maystocks"
const configFileName = "globalconfig.yaml"
const configFileVersion = 2

type GlobalConfig struct {
	loaded            bool
	encryptionPw      string
	encryptionPwMutex sync.Mutex
	version           VersionConfig
	appConfig         AppConfig
	appConfigMutex    sync.Mutex
}

type EncryptedConfig struct {
	IV   string
	Salt string
	Data string
}

type VersionConfig struct {
	FileVersion   int
	Encrypted     bool   `yaml:",omitempty"`
	PlainPassword string `yaml:",omitempty"`
}

func NewGlobalConfig() Config {
	return &GlobalConfig{
		version: VersionConfig{
			FileVersion: configFileVersion,
		},
		appConfig: NewAppConfig(),
	}
}

func (g *GlobalConfig) GetAppName() string {
	return AppName
}

func (g *GlobalConfig) SetEncryptionPassword(pw string) {
	g.encryptionPwMutex.Lock()
	defer g.encryptionPwMutex.Unlock()
	g.encryptionPw = pw
}

func (g *GlobalConfig) IsEncryptionPassword(pw string) bool {
	g.encryptionPwMutex.Lock()
	defer g.encryptionPwMutex.Unlock()
	return g.encryptionPw == pw
}

// Locks access to the configuration and returns a copy which can be modified.
// Unlock needs to be called afterwards, if no error was returned.
func (g *GlobalConfig) Lock() (*AppConfig, error) {
	g.appConfigMutex.Lock()
	if !g.loaded {
		err := g.read()
		if err != nil {
			g.appConfigMutex.Unlock()
			return nil, err
		}
	}
	appConfigCopy := g.appConfig.deepCopy()
	return &appConfigCopy, nil
}

// Update the configuration and unlock access.
// If the configuration was changed, the configuration will be written before unlocking.
func (g *GlobalConfig) Unlock(c *AppConfig, forceWriting bool) error {
	var err error
	if forceWriting || !cmp.Equal(g.appConfig, *c) {
		g.appConfig = *c
		err = g.write()
	}
	g.appConfigMutex.Unlock()
	return err
}

func (g *GlobalConfig) Copy(forceReading bool) (AppConfig, error) {
	g.appConfigMutex.Lock()
	defer g.appConfigMutex.Unlock()
	if forceReading || !g.loaded {
		err := g.read()
		if err != nil {
			return AppConfig{}, err
		}
	}
	return g.appConfig.deepCopy(), nil
}

func (g *GlobalConfig) getAppConfigDir() string {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		// We do not want to run on operating systems without config dir.
		// This is considered to be a fatal error.
		log.Fatalf("unable to determine configuration path: %v", err)
	}
	return filepath.Join(userConfigDir, g.GetAppName())
}

func (g *GlobalConfig) createEncryptionKey(salt []byte) ([]byte, error) {
	g.encryptionPwMutex.Lock()
	defer g.encryptionPwMutex.Unlock()
	if len(g.encryptionPw) == 0 {
		return nil, errors.New("missing encryption password")
	}
	// Parameters used from example at https://pkg.go.dev/golang.org/x/crypto/argon2#IDKey
	return argon2.IDKey([]byte(g.encryptionPw), salt, 1, 64*1024, 4, 32), nil
}

func (g *GlobalConfig) hasEncryptionPw() bool {
	g.encryptionPwMutex.Lock()
	defer g.encryptionPwMutex.Unlock()
	return len(g.encryptionPw) > 0
}

func (g *GlobalConfig) getRandomBytes(size int) ([]byte, error) {
	iv := make([]byte, size)
	_, err := rand.Read(iv)
	if err != nil {
		return nil, errors.New("unable to read random numbers")
	}
	return iv, nil
}

func (g *GlobalConfig) read() error {
	appConfigDir := g.getAppConfigDir()
	fileName := filepath.Join(appConfigDir, configFileName)
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		// It is fine if the configuration file does not yet exist.
		log.Printf("Configuration file \"%s\" does not yet exist, using defaults.", fileName)
		return nil
	}
	file, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %v", err)
	}
	err = yaml.Unmarshal(file, &g.version)
	if err != nil {
		return fmt.Errorf("failed to parse configuration version: %v", err)
	}
	// Avoid removing new unknown settings if an old release is started with a newer config file.
	if g.version.FileVersion > configFileVersion {
		log.Fatalf(
			"Invalid configuration file version %d instead of %d, probably from a newer release.",
			g.version.FileVersion,
			configFileVersion)
	}
	if len(g.version.PlainPassword) > 0 {
		g.SetEncryptionPassword(g.version.PlainPassword)
	}
	if g.version.Encrypted && g.hasEncryptionPw() {
		var encryptedConfig EncryptedConfig
		err = yaml.Unmarshal(file, &encryptedConfig)
		if err != nil {
			return fmt.Errorf("failed to parse encrypted app configuration: %v", err)
		}
		salt, err := base64.StdEncoding.DecodeString(encryptedConfig.Salt)
		if err != nil {
			return fmt.Errorf("failed to decode salt: %v", err)
		}
		key, err := g.createEncryptionKey(salt)
		if err != nil {
			return err
		}
		ciph, err := aes.NewCipher(key)
		if err != nil {
			return err
		}
		gcm, err := cipher.NewGCM(ciph)
		if err != nil {
			return err
		}
		iv, err := base64.StdEncoding.DecodeString(encryptedConfig.IV)
		if err != nil {
			return fmt.Errorf("failed to decode iv: %v", err)
		}
		data, err := base64.StdEncoding.DecodeString(encryptedConfig.Data)
		if err != nil {
			return fmt.Errorf("failed to decode data: %v", err)
		}
		file, err = gcm.Open(nil, iv, data, nil)
		if err != nil {
			return err
		}
	}

	err = yaml.Unmarshal(file, &g.appConfig)
	if err != nil {
		return fmt.Errorf("failed to parse app configuration: %v", err)
	}
	g.appConfig.Sanitize()
	g.appConfig.IsEncrypted = g.version.Encrypted
	g.loaded = true
	return nil
}

func (g *GlobalConfig) write() error {
	appConfigDir := g.getAppConfigDir()
	err := os.Mkdir(appConfigDir, 0700)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create configuration directory: %v", err)
	}
	g.appConfig.Sanitize()
	g.appConfig.RemoveDefaults()
	g.version.FileVersion = configFileVersion
	g.version.Encrypted = true
	fileVersion, err := yaml.Marshal(&g.version)
	if err != nil {
		return fmt.Errorf("error generating configuration version: %v", err)
	}
	fileAppConfig, err := yaml.Marshal(&g.appConfig)
	if err != nil {
		return fmt.Errorf("error generating app configuration: %v", err)
	}
	g.appConfig.RestoreDefaults()

	salt, err := g.getRandomBytes(16)
	if err != nil {
		return err
	}
	key, err := g.createEncryptionKey(salt)
	if err != nil {
		return err
	}
	ciph, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(ciph)
	if err != nil {
		return err
	}
	iv, err := g.getRandomBytes(gcm.NonceSize())
	if err != nil {
		return err
	}
	encryptedData := gcm.Seal(nil, iv, fileAppConfig, nil)
	encryptedConfig := EncryptedConfig{
		IV:   base64.StdEncoding.EncodeToString(iv),
		Salt: base64.StdEncoding.EncodeToString(salt),
		Data: base64.StdEncoding.EncodeToString(encryptedData),
	}
	fileEncryptedConfig, err := yaml.Marshal(&encryptedConfig)
	if err != nil {
		return fmt.Errorf("error generating encrypted app configuration: %v", err)
	}

	file := append(fileVersion, fileEncryptedConfig...)
	fileName := filepath.Join(appConfigDir, configFileName)
	tmpFileName := fileName + ".tmp"
	// Writing may fail, so we write to a temporary file and replace afterwards.
	err = os.WriteFile(tmpFileName, file, 0600)
	if err != nil {
		return fmt.Errorf("failed to write configuration file: %v", err)
	}
	err = os.Rename(tmpFileName, fileName)
	if err != nil {
		return fmt.Errorf("failed to replace configuration file: %v", err)
	}
	return nil
}
