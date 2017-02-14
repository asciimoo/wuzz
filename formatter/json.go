package formatter

import (
	"bytes"
	"errors"
	"io"

	"github.com/nwidger/jsoncolor"
)

type jsonFormatter struct {
	textFormatter
}

func (f *jsonFormatter) Format(writer io.Writer, data []byte) error {
	jsonFormatter := jsoncolor.NewFormatter()
	buf := bytes.NewBuffer(make([]byte, 0, len(data)))
	err := jsonFormatter.Format(buf, data)
	if err == nil {
		writer.Write(buf.Bytes())
		return nil
	}
	return errors.New("json formatter error")
}

func (f *jsonFormatter) Title() string {
	return "[json]"
}
