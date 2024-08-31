package examples

import "embed"

//go:embed *.yaml
var ExampleConfigs embed.FS
