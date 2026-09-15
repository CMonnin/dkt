package main

import (
	"os"

	"github.com/CMonnin/dwkt/internal/cli"
)

func main() { os.Exit(cli.Run(os.Args[1:])) }
