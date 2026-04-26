package squad

import (
	"embed"
	"io/fs"
)

// Explicit patterns rather than `all:plugin` so the Go source under
// plugin/hooks/ (the embed registry + tests) does not ship to user installs.
// `all:` prefix on plugin/commands so dotfile placeholders survive embed.
//
//go:embed plugin/.claude-plugin/plugin.json
//go:embed plugin/skills
//go:embed all:plugin/commands
//go:embed plugin/hooks/*.sh
var pluginAssets embed.FS

func PluginFS() (fs.FS, error) {
	return fs.Sub(pluginAssets, "plugin")
}
