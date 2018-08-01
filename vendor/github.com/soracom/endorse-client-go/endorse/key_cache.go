package endorse

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
)

const (
	magicKeyCache          = 0x1000cace
	envNameKeyStoreKey     = "ENDORSE_KEY_STORE_KEY"
	defaultUnsafePassword  = "!_S0r4C0m_&"
	keyValidDuration       = 3600 * time.Second
	currentKeyCacheVersion = 1
	tagSecretKeyEntry      = 1
)

type keyCache interface {
	findAuthResult(imsi string) (*AuthenticationResult, error)
	saveAuthResult(imsi string, authResult *AuthenticationResult) error
}

func newKeyCache(cfg *Config) keyCache {
	if cfg.KeyCache.Clear {
		err := removeKeyCacheFile()
		if err != nil {
			log("unable to remove key cache file: %+v", err)
		}
	}

	if cfg.KeyCache.Disabled {
		return &noOpKeyCache{}
	}

	kc := newKeyCacheImpl()

	password := loadKeyCachePassword()
	path := getKeyCachePath()
	if doesFileExist(path) {
		err := kc.loadFromFile(path, password)
		if err != nil {
			log("unable to load key cache from file: %+v", err)
		}
	} else {
		err := kc.saveToFile(path, password)
		if err != nil {
			log("unable to create key cache from file: %+v", err)
		}
	}

	return kc
}

func loadKeyCachePassword() []byte {
	pw := os.Getenv(envNameKeyStoreKey)
	if pw != "" {
		return []byte(pw)
	}

	return []byte(defaultUnsafePassword)
}

func getProfileDir() (string, error) {
	profDir := os.Getenv("SORACOM_PROFILE_DIR")
	if profDir == "" {
		dir, err := homedir.Dir()
		if err != nil {
			return "", err
		}
		profDir = filepath.Join(dir, ".soracom")
	}

	return profDir, nil
}

func getKeyCachePath() string {
	pd, err := getProfileDir()
	if err != nil {
		return ""
	}
	return path.Join(pd, ".endorse-client-key-cache")
}

func removeKeyCacheFile() error {
	path := getKeyCachePath()
	return os.Remove(path)
}

func isSupportedKeyCacheVersion(ver uint32) bool {
	return ver <= currentKeyCacheVersion
}
