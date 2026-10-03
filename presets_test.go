package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltinPresets(t *testing.T) {
	groups, err := parsePresets(builtinPresets)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) == 0 {
		t.Fatal("no built-in presets")
	}
}

func TestLoadPresetsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "presets.json")

	if res := loadPresets(path); res.Source != "builtin" || res.Error != "" {
		t.Fatalf("missing file: got %q, error %q", res.Source, res.Error)
	}

	os.WriteFile(path, []byte(`[{"group":"Mine","presets":[{"id":"a","name":{"en":"A","fr":"A fr"},
		"tags":[{"label":"x","pattern":"foo|bar","color":"#112233"},{"pattern":"a.b","color":"#445566","regex":false}]}]}]`), 0o644)
	res := loadPresets(path)
	if res.Source != "file" || res.Error != "" || res.Groups[0].Presets[0].Name["fr"] != "A fr" || res.Groups[0].Group["en"] != "Mine" {
		t.Fatalf("valid file: %+v", res)
	}

	for _, bad := range []string{
		`not json`,
		`[{"group":"G","presets":[{"id":"a","name":"A","tags":[{"pattern":"(","color":"#112233"}]}]}]`,
		`[{"group":"G","presets":[{"id":"a","name":"A","tags":[{"pattern":"x","color":"red"}]}]}]`,
		`[{"group":"G","presets":[{"id":"a","name":"A","tags":[]}]}]`,
		`[{"group":"G","presets":[{"id":"a","name":"A","tags":[{"pattern":"x","color":"#112233"}]},{"id":"a","name":"B","tags":[{"pattern":"y","color":"#112233"}]}]}]`,
	} {
		os.WriteFile(path, []byte(bad), 0o644)
		if res := loadPresets(path); res.Source != "builtin" || res.Error == "" || len(res.Groups) == 0 {
			t.Errorf("%s: got %q, error %q", bad, res.Source, res.Error)
		}
	}
}
