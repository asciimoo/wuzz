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

var ContentTypes = map[string]string{
	"json": "application/json",
	"form": "application/x-www-form-urlencoded",
}

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
	Meta    MetaOptions
	Keys    map[string]map[string]string
}

type GeneralOptions struct {
	Timeout                Duration
	FormatJSON             bool
	Insecure               bool
	PreserveScrollPosition bool
	FollowRedirects        bool
	PersistHistory         bool
	DefaultURLScheme       string
	TLSVersionMin          uint16
	TLSVersionMax          uint16
}

type MetaOptions struct {
	ConfigLocation string
}

var defaultTimeoutDuration, _ = time.ParseDuration("1m")

var DefaultKeys = map[string]map[string]string{
	"global": map[string]string{
		"CtrlR": "submit",
		"CtrlC": "quit",
		"CtrlS": "save",
		"CtrlD": "deleteLine",
		"CtrlW": "deleteWord",
		"Tab":   "nextView",
		"CtrlJ": "nextView",
		"CtrlK": "prevView",
		"AltH":  "history",
		"F2":    "focus url",
		"F3":    "focus get",
		"F4":    "focus method",
		"F5":    "focus data",
		"F6":    "focus headers",
		"F7":    "focus search",
		"F8":    "focus response-headers",
		"F9":    "focus response-body",
	},
	"url": map[string]string{
		"Enter": "submit",
	},
	"response-headers": map[string]string{
		"ArrowUp":   "scrollUp",
		"ArrowDown": "scrollDown",
		"PageUp":    "pageUp",
		"PageDown":  "pageDown",
	},
	"response-body": map[string]string{
		"ArrowUp":   "scrollUp",
		"ArrowDown": "scrollDown",
		"PageUp":    "pageUp",
		"PageDown":  "pageDown",
	},
	"help": map[string]string{
		"ArrowUp":   "scrollUp",
		"ArrowDown": "scrollDown",
		"PageUp":    "pageUp",
		"PageDown":  "pageDown",
	},
}

var DefaultConfig = Config{
	General: GeneralOptions{
		Timeout: Duration{
			defaultTimeoutDuration,
		},
		FormatJSON:             true,
		Insecure:               false,
		PreserveScrollPosition: true,
		FollowRedirects:        true,
		PersistHistory:         false,
		DefaultURLScheme:       "https",
	},
	Meta: MetaOptions{
		ConfigLocation: "",
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

	conf.Meta.ConfigLocation = configFile

	if conf.Keys == nil {
		conf.Keys = DefaultKeys
	} else {
		// copy default keys
		for keyCategory, keys := range DefaultKeys {
			confKeys, found := conf.Keys[keyCategory]
			if found {
				for key, action := range keys {
					if _, found := confKeys[key]; !found {
						conf.Keys[keyCategory][key] = action
					}
				}
			} else {
				conf.Keys[keyCategory] = keys
			}
		}
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
