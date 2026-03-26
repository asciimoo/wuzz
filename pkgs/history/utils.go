package history

import (
	"os"
	"path/filepath"
)

func GetHistoryDBPath() string {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "wuzz", "history.db")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "history.db"
	}

	xdgPath := filepath.Join(homeDir, ".config", "wuzz", "history.db")
	if _, err := os.Stat(filepath.Dir(xdgPath)); err == nil {
		return xdgPath
	}

	return filepath.Join(homeDir, ".wuzz", "history.db")
}
