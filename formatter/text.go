package formatter

import (
	"io"
	"regexp"
)

type TextFormatter struct {
}

func (f *TextFormatter) Format(writer io.Writer, data []byte) error {
	_, err := writer.Write(data)
	return err
}

func (f *TextFormatter) Title() string {
	return "[text]"
}

func (f *TextFormatter) Searchable() bool {
	return true
}

func (f *TextFormatter) Search(q string, body []byte) ([]string, error) {
	search_re, err := regexp.Compile(q)
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0, 16)
	for _, match := range search_re.FindAll(body, 1000) {
		ret = append(ret, string(match))
	}
	return ret, nil
}
