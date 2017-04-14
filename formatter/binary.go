package formatter

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

type binaryFormatter struct {
}

func (f *binaryFormatter) Format(writer io.Writer, data []byte) error {
	fmt.Fprint(writer, hex.Dump(data))
	return nil
}

func (f *binaryFormatter) Title() string {
	return "[binary]"
}

func (f *binaryFormatter) Searchable() bool {
	return false
}

func (f *binaryFormatter) Search(q string, body []byte) ([]string, error) {
	return nil, errors.New("Cannot perform search on binary content type")
}
