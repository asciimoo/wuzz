package formatter

type htmlFormatter struct {
	textFormatter
}

func (f *htmlFormatter) Title() string {
	return "[html]"
}
