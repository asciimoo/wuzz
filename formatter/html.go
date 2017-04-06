package formatter

import (
	"bytes"
	"io"

	"github.com/x86kernel/htmlcolor"
)

type htmlFormatter struct {
	textFormatter
}

func (f *htmlFormatter) Format(writer io.Writer, data []byte) error {
	htmlFormatter := htmlcolor.NewFormatter()
	buf := bytes.NewBuffer(make([]byte, 0, len(data)))
	htmlFormatter.Format(buf, data)

	writer.Write(buf.Bytes())

	return nil
}

func (f *htmlFormatter) Title() string {
	return "[html]"
}
