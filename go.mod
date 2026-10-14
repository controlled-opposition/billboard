module github.com/controlled-opposition/billboard

go 1.26.8

// Planned (not imported by the sketch so it builds offline):
//   github.com/chromedp/chromedp v0.21.0   (pin: 0.21 rewrote the API)
//   github.com/chromedp/cdproto
//   github.com/cli/go-gh/v2                (pkg/jq, pkg/template, pkg/tableprinter, pkg/term)
//   github.com/zalando/go-keyring
//   golang.org/x/net/html                  (attachment extraction from Ultra document bodies)

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
)
