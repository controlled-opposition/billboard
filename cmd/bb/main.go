package main

import (
	"os"

	"github.com/controlled-opposition/billboard/internal/cli"
)

func main() { os.Exit(int(cli.Run(os.Args[1:]))) }
