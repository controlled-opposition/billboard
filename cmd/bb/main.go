package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/controlled-opposition/billboard/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, cli.OSEnv(), os.Args[1:])
	stop()
	os.Exit(code)
}
