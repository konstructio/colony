package templates

import (
	"embed"
)

//go:embed *.yaml
var Templates embed.FS
