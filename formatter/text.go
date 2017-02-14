package formatter

import (
	"io"
)

type textFormatter struct {
}

func (f *textFormatter) Format(writer io.Writer, data []byte) error {
	_, err := writer.Write(data)
	return err
}

func (f *textFormatter) Title() string {
	return "[text]"
}

func (f *textFormatter) Searchable() bool {
	return true
}
