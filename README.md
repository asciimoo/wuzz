# wuzz

Interactive cli tool for HTTP inspection

Wuzz command line arguments are similar to cURL's arguments,
so it can be used to inspect/modify requests copied from the
browser's network inspector with the "copy as cURL" feature.

![wuzz screencast](docs/images/screencast.gif)


## Installation and usage

```
$ go get github.com/asciimoo/wuzz
$ "$GOPATH/bin/wuzz" --help
```

### Commands

Keybinding                              | Description
----------------------------------------|------------------------------------------------------------	gocui.KeyF2: "url",
<kbd>F3</kbd>                           | URL
<kbd>F4</kbd>                           | HTTP method
<kbd>F5</kbd>                           | Request body
<kbd>F6</kbd>                           | Headers
<kbd>F7</kbd>                           | Search
<kbd>F8</kbd>                           | Response headers
<kbd>F9</kbd>                           | Response body
<kbd>Ctrl+R</kbd>                       | Send request.
<kbd>Ret</kbd>                          | Send request from window URL only.
<kbd>Ctrl+C</kbd>                       | Quit.
<kbd>Ctrl+K</kbd>, <kbd>Shift+Tab</kbd> | Previous view.
<kbd>Ctlr+J</kbd>, <kbd>Tab</kbd>       | Next view.
<kbd>Ctrl+H</kbd>, <kbd>Alt+H</kbd>     | Toggle history.
<kbd>Down</kbd>                         | Move down one view line.
<kbd>Up</kbd>                           | Move up one view line.
<kbd>Page down</kbd>                    | Move down one view page.
<kbd>Page up</kbd>                      | Move up one view page.


## TODO

* Colors
* Save response with ctrl+s
* Response specific filters (xpath, etc..)
* Binary respone view
* Better navigation
* File upload
* Autocompletion
* Tests


## Bugs

Bugs or suggestions? Visit the [issue tracker](https://github.com/asciimoo/wuzz/issues).
