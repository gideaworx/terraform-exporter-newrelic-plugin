package main

import (
	"log"
	"os"

	"github.com/gideaworx/terraform-exporter-newrelic-plugin/plugins/synthetics"
	plugin "github.com/gideaworx/terraform-exporter-plugin-go"
)

var Version = "0.0.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		log.Fatal(Version)
	}

	plugin.ServeCommands(
		plugin.FromString(Version),
		plugin.RPCProtocol,
		synthetics.NewSyntheticExporterCommand(),
	)
}
