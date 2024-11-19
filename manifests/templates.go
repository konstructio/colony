package manifests

import (
	"embed"
)

//go:embed templates/*.yaml
var Templates embed.FS

//go:embed colony/*.yaml.tmpl
var Colony embed.FS
