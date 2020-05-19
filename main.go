package main

import (
	"flag"

	"github.com/juniorz/edgemax-exporter/cmd"
)

var (
	configHelp = flag.Bool("help", false, "Displays help")
)

func main() {
	flag.Parse()

	if *configHelp {
		flag.PrintDefaults()
		return
	}

	cmd.Execute()
}
