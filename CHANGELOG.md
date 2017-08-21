## 0.4.0 2017.08.21

 - Save/load requests (`-f`/`--file` flags for loading)
 - Multipart form handling (`-F`/`--form` flags)
 - Edit window content in external editor
 - Colorized html output
 - Context specific search (github.com/tidwall/gjson for JSON)
 - More consistency with cURL API (`--data-urlencode` flag added)
 - Update to the latest `gocui` ui library


## 0.3.0 2017.03.07

- Request header autocompletion
- Configurable statusline
- JSON requests with `-j`/`--json` flags
- Allow insecure HTTPS requests (`-k`/`--insecure` flags)
- Socks proxy support (`-x`/`--proxy` flags)
- Disable following redirects (`-R`/`--disable-redirects` flags)
- Enhanced TLS support (`-T`/`--tls`, `-1`/`--tlsv1`, `--tlsv1.0`, `--tlsv1.1`, `--tlsv1.2` flags)
- Commands for line and word deletion
- Home/end navigation fix

## 0.2.0 2017.02.18

- Config file support with configurable keybindings
- Help popup (F1 key)
- Ignore invalid SSL certs with the --insecure flag
- PATCH request support
- Allow JSON request body (--data-binary flag)
- Colorized JSON response
- Parameter encoding bugfix
- Multiple UI bugfixes

## 0.1.0 2017.02.11

Initial release
