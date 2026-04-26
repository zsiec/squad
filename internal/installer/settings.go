package installer

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type MCPServerSpec struct {
	Command       string
	Args          []string
	Env           map[string]string
	MarkerVersion string
}

func MergeMCPServers(path string, specs map[string]MCPServerSpec) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	var settings map[string]any
	switch {
	case errors.Is(err, fs.ErrNotExist):
		settings = map[string]any{}
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(raw, &settings); err != nil {
			return err
		}
		if settings == nil {
			settings = map[string]any{}
		}
	}

	serversAny, _ := settings["mcpServers"].(map[string]any)
	if serversAny == nil {
		serversAny = map[string]any{}
	}
	for name, spec := range specs {
		entry := map[string]any{
			"command": spec.Command,
			"args":    spec.Args,
			"squad":   spec.MarkerVersion,
		}
		if len(spec.Env) > 0 {
			envOut := map[string]any{}
			for k, v := range spec.Env {
				envOut[k] = v
			}
			entry["env"] = envOut
		}
		serversAny[name] = entry
	}
	settings["mcpServers"] = serversAny

	return atomicWriteJSON(path, settings)
}

// UnmergeMCPServers removes any mcpServers["<name>"] whose `squad` marker
// matches the version we wrote. Hand-edited entries (no marker) are left alone.
func UnmergeMCPServers(path string, names []string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		return err
	}
	servers, _ := settings["mcpServers"].(map[string]any)
	if servers == nil {
		return nil
	}
	for _, name := range names {
		entry, ok := servers[name].(map[string]any)
		if !ok {
			continue
		}
		if _, marked := entry["squad"]; marked {
			delete(servers, name)
		}
	}
	settings["mcpServers"] = servers
	return atomicWriteJSON(path, settings)
}

func atomicWriteJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".squad-settings-*")
	if err != nil {
		return err
	}
	cleanup := tmp.Name()
	defer func() {
		if cleanup != "" {
			os.Remove(cleanup)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	cleanup = ""
	return nil
}
