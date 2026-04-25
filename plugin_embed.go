package squad

import (
	"embed"
	"io/fs"
)

//go:embed all:plugin
var pluginAssets embed.FS

func PluginFS() (fs.FS, error) {
	return fs.Sub(pluginAssets, "plugin")
}
