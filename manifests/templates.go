package manifests

import (
	"embed"
)

//go:embed colony/*.yaml.tmpl
var Colony embed.FS

//go:embed downloads/*.yaml
var Downloads embed.FS

//go:embed ipmi/*.yaml.tmpl
var IPMI embed.FS

//go:embed templates/*.yaml
var Templates embed.FS
