package cc

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// HookNames is the full set of CC lifecycle hooks Roost subscribes to.
// We intentionally subscribe to all of them so the classifier sees the full
// state machine; missing UserPromptSubmit (for example) wouldn't affect
// status detection but would degrade transcript-recap quality later.
var HookNames = []string{
	"SessionStart",
	"PreToolUse",
	"PostToolUse",
	"UserPromptSubmit",
	"Stop",
	"Notification",
}

// Install merges a roost-hook entry into each lifecycle hook in CC's
// settings.json. Existing user hooks are preserved. Running twice is a no-op.
func Install(settingsPath, roostHookBin string) error {
	settings, err := load(settingsPath)
	if err != nil {
		return err
	}

	hooks := getOrCreateMap(settings, "hooks")
	for _, name := range HookNames {
		entries := asSlice(hooks[name])
		if alreadyPresent(entries, roostHookBin) {
			continue
		}
		entries = append(entries, map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": roostHookBin,
				},
			},
		})
		hooks[name] = entries
	}
	return save(settingsPath, settings)
}

// Uninstall removes only the roost-hook entries from settings.json, leaving
// user-installed hooks untouched. If a hook ends up empty, the key is dropped.
func Uninstall(settingsPath, roostHookBin string) error {
	settings, err := load(settingsPath)
	if err != nil {
		return err
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil
	}
	for _, name := range HookNames {
		entries := asSlice(hooks[name])
		if len(entries) == 0 {
			continue
		}
		filtered := []any{}
		for _, entry := range entries {
			m, ok := entry.(map[string]any)
			if !ok {
				filtered = append(filtered, entry)
				continue
			}
			inner := asSlice(m["hooks"])
			kept := []any{}
			for _, h := range inner {
				if hm, ok := h.(map[string]any); ok && hm["command"] == roostHookBin {
					continue // drop ours
				}
				kept = append(kept, h)
			}
			if len(kept) > 0 {
				m["hooks"] = kept
				filtered = append(filtered, m)
			}
		}
		if len(filtered) == 0 {
			delete(hooks, name)
		} else {
			hooks[name] = filtered
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	return save(settingsPath, settings)
}

func load(path string) (map[string]any, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return s, nil
}

// save writes atomically via a temp file + rename. If the rename fails the
// original file is untouched.
func save(path string, s map[string]any) error {
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func getOrCreateMap(parent map[string]any, key string) map[string]any {
	if v, ok := parent[key].(map[string]any); ok {
		return v
	}
	m := map[string]any{}
	parent[key] = m
	return m
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func alreadyPresent(entries []any, cmd string) bool {
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range asSlice(m["hooks"]) {
			if hm, ok := h.(map[string]any); ok && hm["command"] == cmd {
				return true
			}
		}
	}
	return false
}
