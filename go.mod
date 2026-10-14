module github.com/controlled-opposition/billboard

go 1.26.8

// Planned (not imported by the sketch so it builds offline):
//   github.com/chromedp/chromedp v0.21.0   (pin: 0.21 rewrote the API)
//   github.com/chromedp/cdproto
//   github.com/cli/go-gh/v2                (pkg/jq, pkg/template, pkg/tableprinter, pkg/term)
//   github.com/zalando/go-keyring
//   golang.org/x/net/html                  (attachment extraction from Ultra document bodies)

require (
	github.com/chromedp/cdproto v0.157.8
	github.com/chromedp/chromedp v0.21.0
	github.com/spf13/cobra v1.10.2
	github.com/zalando/go-keyring v0.2.8
)

require (
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/go-json-experiment/json v0.0.0-20260213210345-44df1a37e875 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
