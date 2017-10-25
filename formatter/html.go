package formatter

import (
	"bytes"
	"errors"
	"io"

	"github.com/PuerkitoBio/goquery"
	"github.com/x86kernel/htmlcolor"
)

type htmlFormatter struct {
	parsedBody goquery.Document
	TextFormatter
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

func (f *htmlFormatter) Search(q string, body []byte) ([]string, error) {
	if q == "" {
		buf := bytes.NewBuffer(make([]byte, 0, len(body)))
		err := f.Format(buf, body)
		return []string{buf.String()}, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, 8)
	doc.Find(q).Each(func(_ int, s *goquery.Selection) {
		htmlResult, err := goquery.OuterHtml(s)
		if err == nil {
			results = append(results, htmlResult)
		}
	})

	return results, nil
}
