package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

// Duration is used to automatically unmarshal timeout strings to
// time.Duration values
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Config struct {
	General GeneralOptions
	Keys    map[string]map[string]string
}

type GeneralOptions struct {
	Timeout                Duration
	FormatJSON             bool
	Insecure               bool
	PreserveScrollPosition bool
	DefaultURLScheme       string
}

var defaultTimeoutDuration, _ = time.ParseDuration("1m")

var DefaultConfig = Config{
	General: GeneralOptions{
		Timeout: Duration{
			defaultTimeoutDuration,
		},
		FormatJSON:             true,
		Insecure:               false,
		PreserveScrollPosition: true,
		DefaultURLScheme:       "https",
	},
}

func LoadConfig(configFile string) (*Config, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, errors.New("Config file does not exist.")
	} else if err != nil {
		return nil, err
	}

	conf := DefaultConfig
	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

func GetDefaultConfigLocation() string {
	var configFolderLocation string
	switch runtime.GOOS {
	case "linux":
		// Use the XDG_CONFIG_HOME variable if it is set, otherwise
		// $HOME/.config/wuzz/config.toml
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome != "" {
			configFolderLocation = xdgConfigHome
		} else {
			configFolderLocation, _ = homedir.Expand("~/.config/wuzz/")
		}

	default:
		// On other platforms we just use $HOME/.wuzz
		configFolderLocation, _ = homedir.Expand("~/.wuzz/")
	}

	return filepath.Join(configFolderLocation, "config.toml")
}
