package config

var HelpText = `wuzz - Interactive cli tool for HTTP inspection

Usage: wuzz [-H|--header HEADER]... [-d|--data|--data-binary DATA] [-X|--request METHOD] [-t|--timeout MSECS] [URL]

Other command line options:
  -c, --config PATH   Specify custom configuration file
  -h, --help          Show this
  -v, --version       Display version number

Key bindings:
  ctrl+r              Send request
  ctrl+s              Save response
  tab, ctrl+j         Next window
  shift+tab, ctrl+k   Previous window
  ctrl+h, alt+h       Show history
  pageUp              Scroll up the current window
  pageDown            Scroll down the current window`
