package main

import (
	"os"

	"github.com/squrious/dockshim/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args, cli.SystemEnv()))
}
