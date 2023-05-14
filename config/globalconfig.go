// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) Lothar May

package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
)

const AppName = "maystocks"
const configFileName = "globalconfig.yaml"
const configFileVersion = 1

type GlobalConfig struct {
	loaded         bool
	version        VersionConfig
	appConfig      AppConfig
	appConfigMutex sync.Mutex
}

type VersionConfig struct {
	FileVersion int
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
func (g *GlobalConfig) Unlock(c *AppConfig) error {
	var err error
	if !cmp.Equal(g.appConfig, *c) {
		g.appConfig = *c
		err = g.write()
	}
	g.appConfigMutex.Unlock()
	return err
}

func (g *GlobalConfig) Copy() (AppConfig, error) {
	g.appConfigMutex.Lock()
	defer g.appConfigMutex.Unlock()
	if !g.loaded {
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
	err = yaml.Unmarshal(file, &g.appConfig)
	if err != nil {
		return fmt.Errorf("failed to parse app configuration: %v", err)
	}
	g.appConfig.Sanitize()
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
	fileVersion, err := yaml.Marshal(&g.version)
	if err != nil {
		return fmt.Errorf("error generating configuration version: %v", err)
	}
	fileAppConfig, err := yaml.Marshal(&g.appConfig)
	if err != nil {
		return fmt.Errorf("error generating app configuration: %v", err)
	}
	g.appConfig.RestoreDefaults()

	file := append(fileVersion, fileAppConfig...)
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
