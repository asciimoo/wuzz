package history

import (
	"os"
	"path/filepath"

	"github.com/hasithdealwis/wuzz/config"
	"github.com/hasithdealwis/wuzz/formatter"
	"github.com/hasithdealwis/wuzz/pkgs/request"
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

// EntryToRequest converts a history.Entry to a request.Request
func EntryToRequest(entry *Entry, config *config.Config) *request.Request {
	return &request.Request{
		Url:             entry.URL,
		Method:          entry.Method,
		GetParams:       entry.GetParams,
		Data:            entry.Data,
		Headers:         entry.Headers,
		ResponseHeaders: entry.ResponseHeaders,
		RawResponseBody: entry.RawResponseBody,
		ContentType:     entry.ContentType,
		Duration:        entry.Duration,
		Formatter:       formatter.New(config, entry.ContentType),
	}
}
