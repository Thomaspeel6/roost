package cc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return s
}

func countRoostHooks(s map[string]any, hookName, bin string) int {
	hooks, ok := s["hooks"].(map[string]any)
	if !ok {
		return 0
	}
	entries, ok := hooks[hookName].([]any)
	if !ok {
		return 0
	}
	count := 0
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			if hm, ok := h.(map[string]any); ok && hm["command"] == bin {
				count++
			}
		}
	}
	return count
}

func TestInstall_FreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := Install(path, "/usr/local/bin/roost-hook"); err != nil {
		t.Fatal(err)
	}
	s := readJSON(t, path)
	for _, name := range HookNames {
		if got := countRoostHooks(s, name, "/usr/local/bin/roost-hook"); got != 1 {
			t.Errorf("hook %s: expected 1 roost-hook entry, got %d", name, got)
		}
	}
}

func TestInstall_PreservesNonHookKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{"theme":"dark","model":"sonnet"}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Install(path, "/x/roost-hook"); err != nil {
		t.Fatal(err)
	}
	s := readJSON(t, path)
	if s["theme"] != "dark" {
		t.Errorf("theme key not preserved: %v", s["theme"])
	}
	if s["model"] != "sonnet" {
		t.Errorf("model key not preserved: %v", s["model"])
	}
}

func TestInstall_PreservesExistingUserHook(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo user-installed-hook"}]}]}}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/x/roost-hook"); err != nil {
		t.Fatal(err)
	}

	s := readJSON(t, path)
	hooks := s["hooks"].(map[string]any)
	ss := hooks["SessionStart"].([]any)
	if len(ss) < 2 {
		t.Errorf("user hook should be preserved alongside roost's; got %d SessionStart entries", len(ss))
	}
	// Confirm both the user's command and ours are present.
	saw := map[string]bool{}
	for _, entry := range ss {
		m := entry.(map[string]any)
		for _, h := range m["hooks"].([]any) {
			saw[h.(map[string]any)["command"].(string)] = true
		}
	}
	if !saw["echo user-installed-hook"] {
		t.Error("user's hook should still be present")
	}
	if !saw["/x/roost-hook"] {
		t.Error("roost-hook should be installed")
	}
}

func TestInstall_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := Install(path, "/x/roost-hook"); err != nil {
		t.Fatal(err)
	}
	if err := Install(path, "/x/roost-hook"); err != nil {
		t.Fatal(err)
	}

	s := readJSON(t, path)
	for _, name := range HookNames {
		if got := countRoostHooks(s, name, "/x/roost-hook"); got != 1 {
			t.Errorf("hook %s after double install: expected 1 roost entry, got %d", name, got)
		}
	}
}

func TestUninstall_RemovesOnlyRoost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo user"}]}]}}`
	os.WriteFile(path, []byte(original), 0644)

	Install(path, "/x/roost-hook")
	if err := Uninstall(path, "/x/roost-hook"); err != nil {
		t.Fatal(err)
	}

	s := readJSON(t, path)
	hooks := s["hooks"].(map[string]any)
	ss := hooks["SessionStart"].([]any)
	for _, entry := range ss {
		inner := entry.(map[string]any)["hooks"].([]any)
		for _, h := range inner {
			if h.(map[string]any)["command"] == "/x/roost-hook" {
				t.Error("roost hook should be removed")
			}
		}
	}
	// User's hook is still there.
	saw := false
	for _, entry := range ss {
		for _, h := range entry.(map[string]any)["hooks"].([]any) {
			if h.(map[string]any)["command"] == "echo user" {
				saw = true
			}
		}
	}
	if !saw {
		t.Error("user's hook should be preserved through uninstall")
	}
}

func TestUninstall_DropsEmptyHookKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	Install(path, "/x/roost-hook")
	if err := Uninstall(path, "/x/roost-hook"); err != nil {
		t.Fatal(err)
	}

	s := readJSON(t, path)
	if hooks, ok := s["hooks"]; ok && len(hooks.(map[string]any)) > 0 {
		t.Errorf("uninstall should drop the hooks map when no hooks remain, got %v", hooks)
	}
}
