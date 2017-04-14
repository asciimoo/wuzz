package formatter

import (
	"io"
	"mime"
	"strings"

	"github.com/asciimoo/wuzz/config"
)

type ResponseFormatter interface {
	Format(writer io.Writer, data []byte) error
	Title() string
	Searchable() bool
	Search(string, []byte) ([]string, error)
}

func New(appConfig *config.Config, contentType string) ResponseFormatter {
	ctype, _, err := mime.ParseMediaType(contentType)
	if err == nil && appConfig.General.FormatJSON && (ctype == config.ContentTypes["json"] || strings.HasSuffix(ctype, "+json")) {
		return &jsonFormatter{}
	} else if strings.Contains(contentType, "text/html") {
		return &htmlFormatter{}
	} else if strings.Index(contentType, "text") == -1 && strings.Index(contentType, "application") == -1 {
		return &binaryFormatter{}
	} else {
		return &TextFormatter{}
	}
}
