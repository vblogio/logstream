package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Built-in tag presets, used when no presets file is found (PRESETS_FILE,
// /data/presets.json by default). See docs/presets.md.
//
//go:embed presets.json
var builtinPresets []byte

// i18nText is a text per language ({"en": "...", "fr": "..."}) or one plain
// string for every language.
type i18nText map[string]string

func (t *i18nText) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*t = i18nText{"en": s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return errors.New("expected a string or an object of strings per language")
	}
	*t = m
	return nil
}

type PresetTag struct {
	Label         i18nText `json:"label,omitempty"`
	Pattern       string   `json:"pattern"`
	Color         string   `json:"color"`
	Regex         *bool    `json:"regex,omitempty"` // regular expression unless false
	WholeWord     bool     `json:"wholeWord,omitempty"`
	CaseSensitive bool     `json:"caseSensitive,omitempty"`
}

type Preset struct {
	ID   string      `json:"id"`
	Name i18nText    `json:"name"`
	Tags []PresetTag `json:"tags"`
}

type PresetGroup struct {
	Group   i18nText `json:"group"`
	Presets []Preset `json:"presets"`
}

// parsePresets decodes and checks a presets file with the same rules as the
// tags, so that every preset can be added as is.
func parsePresets(b []byte) ([]PresetGroup, error) {
	var groups []PresetGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, g := range groups {
		for _, p := range g.Presets {
			if p.ID == "" || seen[p.ID] {
				return nil, fmt.Errorf("preset %q: missing or duplicate id", p.ID)
			}
			seen[p.ID] = true
			if len(p.Tags) == 0 {
				return nil, fmt.Errorf("preset %q: no tags", p.ID)
			}
			for i, pt := range p.Tags {
				t := Tag{Pattern: pt.Pattern, Color: pt.Color, Regex: pt.Regex == nil || *pt.Regex}
				if err := t.validate(); err != nil {
					return nil, fmt.Errorf("preset %q, tag %d: %w", p.ID, i+1, err)
				}
			}
		}
	}
	return groups, nil
}

// presetsResponse is what GET /api/presets returns.
type presetsResponse struct {
	Source string        `json:"source"` // "file" or "builtin"
	File   string        `json:"file"`
	Error  string        `json:"error,omitempty"` // the file is invalid: built-in presets are used
	Groups []PresetGroup `json:"groups"`
}

// loadPresets reads the presets file on every call, so that edits apply
// without a restart, and falls back to the built-in presets.
func loadPresets(path string) presetsResponse {
	res := presetsResponse{Source: "builtin", File: path}
	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			if res.Groups, err = parsePresets(b); err == nil {
				res.Source = "file"
				return res
			}
			res.Error = err.Error()
		case !errors.Is(err, os.ErrNotExist):
			res.Error = err.Error()
		}
	}
	res.Groups, _ = parsePresets(builtinPresets) // checked by the tests
	return res
}
