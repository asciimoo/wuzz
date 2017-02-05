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


## TODO

* Colors
* Save response with ctrl+s
* Response specific filters (xpath, etc..)
* Binary respone view
* Better navigation
* File upload


## Bugs

Bugs or suggestions? Visit the [issue tracker](https://github.com/asciimoo/wuzz/issues).
