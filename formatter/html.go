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
	err := htmlFormatter.Format(buf, data)

	if err == io.EOF {
		writer.Write(buf.Bytes())
		return nil
	}

	return errors.New("html formatter error")
}

func (f *htmlFormatter) Title() string {
	return "[html]"
}
