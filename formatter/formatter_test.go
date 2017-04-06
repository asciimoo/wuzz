package formatter

import (
	"bytes"
	"testing"

	"github.com/asciimoo/wuzz/config"
	"github.com/nwidger/jsoncolor"
	"github.com/x86kernel/htmlcolor"
)

func TestFormat(t *testing.T) {
	var binBuffer bytes.Buffer
	New(configFixture(true), "octet-stream").Format(&binBuffer, []byte("some binary data"))
	if binBuffer.String() != "00000000  73 6f 6d 65 20 62 69 6e  61 72 79 20 64 61 74 61  |some binary data|\n" {
		t.Error("Expected binary to eq " + binBuffer.String())
	}

	var htmlBuffer bytes.Buffer
	New(configFixture(true), "text/html; charset=utf-8").Format(&htmlBuffer, []byte("<html><span>unfomatted</span></html>"))
	var htmltargetBuffer bytes.Buffer
	htmlcolor.NewFormatter().Format(&htmltargetBuffer, []byte("<html><span>unfomatted</span></html>"))
	htmltarget := htmltargetBuffer.String()

	if htmlBuffer.String() != htmltarget {
		t.Error("Expected html to eq " + htmlBuffer.String())
	}

	var jsonEnabledBuffer bytes.Buffer
	New(configFixture(true), "application/json; charset=utf-8").Format(&jsonEnabledBuffer, []byte("{\"json\": \"some value\"}"))
	var jsontargetBuffer bytes.Buffer
	jsoncolor.NewFormatter().Format(&jsontargetBuffer, []byte("{\"json\": \"some value\"}"))
	jsontarget := jsontargetBuffer.String()

	if jsonEnabledBuffer.String() != jsontarget {
		t.Error("Expected json to eq \n" + jsonEnabledBuffer.String() + "\nbut not\n" + jsontarget)
	}

	var jsonDisabledBuffer bytes.Buffer
	New(configFixture(false), "application/json; charset=utf-8").Format(&jsonDisabledBuffer, []byte("{\"json\": \"some value\"}"))
	if jsonDisabledBuffer.String() != "{\"json\": \"some value\"}" {
		t.Error("Expected json to eq " + jsonDisabledBuffer.String())
	}

	var textBuffer bytes.Buffer
	New(configFixture(true), "text/html; charset=utf-8").Format(&textBuffer, []byte("some text"))
	if textBuffer.String() != "some text" {
		t.Error("Expected text to eq " + textBuffer.String())
	}
}

func TestTitle(t *testing.T) {
	//binary
	title := New(configFixture(true), "octet-stream").Title()
	if title != "[binary]" {
		t.Error("for octet-stream content type expected title ", title, "to be [binary]")
	}

	//html
	title = New(configFixture(true), "text/html; charset=utf-8").Title()
	if title != "[html]" {
		t.Error("For text/html content type expected title ", title, " to be [html]")
	}

	//json
	title = New(configFixture(true), "application/json; charset=utf-8").Title()
	if title != "[json]" {
		t.Error("For text/html content type expected title ", title, " to be [json]")
	}

	//text
	title = New(configFixture(true), "text/plain; charset=utf-8").Title()
	if title != "[text]" {
		t.Error("For text/html content type expected title ", title, " to be [text]")
	}
}

func TestSearchable(t *testing.T) {
	if New(configFixture(true), "octet-stream").Searchable() {
		t.Error("binary file can't be searchable")
	}

	if !New(configFixture(true), "text/html").Searchable() {
		t.Error("text/html should be searchable")
	}

	if !New(configFixture(true), "application/json").Searchable() {
		t.Error("application/json should be searchable")
	}
	if !New(configFixture(true), "text/plain").Searchable() {
		t.Error("text/plain should be searchable")
	}

}

func configFixture(jsonEnabled bool) *config.Config {
	return &config.Config{
		General: config.GeneralOptions{
			FormatJSON: jsonEnabled,
		},
	}
}
