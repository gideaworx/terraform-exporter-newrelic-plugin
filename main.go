package main

import (
	"github.com/gideaworx/terraform-exporter-newrelic-plugin/plugins/synthetics"
	"github.com/gideaworx/terraform-exporter-plugin/go-plugin"
)

var Version = "0.0.0"

func main() {
	plugin.ServeCommands(
		plugin.FromString(Version),
		plugin.RPCProtocol,
		synthetics.NewSyntheticExporterCommand(),
	)
}
