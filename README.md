# Newrelic Plugin for `terraform-exporter`.

This [`terraform-exporter`](https://github.com/gideaworx/terraform-exporter)
plugin provides the ability to export data from [New Relic](https://newrelic.com)
into Hashicorp HCL2 files that can be used to manage existing artifacts in a 
terraform stack.

Currently exportable (more added soon!):
1. Basic Synthetic Monitors
1. Browser-based Step Synthetic Monitors
1. Browser-based Script Synthetic Monitors

## Building

Building requires `go` 1.20 or later.

```bash
$ go build main.go
```

## Installing the plugin into the CLI

```bash
$ go install
$ terraform-exporter install -f "$(go env GOPATH)/bin/terraform-exporter-newrelic-plugin"
```

## Contributing

Pull Requests are welcome! Please open an [issue](/issues/new) before submitting
so it can be discussed beforehand.
