package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Tag highlights a keyword in displayed messages.
type Tag struct {
	ID            string `json:"id"`
	Code          string `json:"code"` // two digits, shown on matching log lines
	Pattern       string `json:"pattern"`
	Label         string `json:"label,omitempty"` // shown instead of the pattern (presets)
	Color         string `json:"color"`
	WholeWord     bool   `json:"wholeWord"`
	CaseSensitive bool   `json:"caseSensitive"`
	Regex         bool   `json:"regex"`
	Enabled       bool   `json:"enabled"`
}

func defaultTags() []Tag {
	return []Tag{
		{ID: "warning", Code: "01", Pattern: "warning", Color: "#fdba74", WholeWord: true, Enabled: true},
		{ID: "error", Code: "02", Pattern: "error", Color: "#fca5a5", WholeWord: true, Enabled: true},
	}
}

var codeRe = regexp.MustCompile(`^[0-9]{2}$`)

// freeCode returns the lowest code from 01 to 99 not used by tags, or "" when
// all are taken. A code stays with its tag until the tag is deleted.
func freeCode(tags []Tag) string {
	used := map[string]bool{}
	for _, t := range tags {
		used[t.Code] = true
	}
	for n := 1; n <= 99; n++ {
		if c := fmt.Sprintf("%02d", n); !used[c] {
			return c
		}
	}
	return ""
}

// assignCodes gives a code to the tags that have none (files written before
// codes existed) or share one with an earlier tag.
func assignCodes(tags []Tag) bool {
	changed := false
	seen := map[string]bool{}
	for i := range tags {
		if codeRe.MatchString(tags[i].Code) && !seen[tags[i].Code] {
			seen[tags[i].Code] = true
			continue
		}
		tags[i].Code = ""
		tags[i].Code = freeCode(tags)
		seen[tags[i].Code] = true
		changed = true
	}
	return changed
}

// Colors of the default tags in earlier versions: still unchanged, they are
// switched to the new pastel defaults when the file is loaded.
var oldDefaultColors = map[string]string{"warning": "#f59e0b", "error": "#ef4444", "ok": "#22c55e"}

func migrateDefaultColors(tags []Tag) bool {
	newColors := map[string]string{}
	for _, t := range defaultTags() {
		newColors[t.ID] = t.Color
	}
	changed := false
	for i := range tags {
		if old, ok := oldDefaultColors[tags[i].ID]; ok && strings.EqualFold(tags[i].Color, old) {
			tags[i].Color = newColors[tags[i].ID]
			changed = true
		}
	}
	return changed
}

// codedError carries a stable code the UI can translate.
type codedError struct {
	code   string
	msg    string
	detail string
}

func (e *codedError) Error() string {
	if e.detail != "" {
		return e.msg + ": " + e.detail
	}
	return e.msg
}

var (
	errTagNotFound = &codedError{code: "tag_not_found", msg: "tag not found"}
	errTooManyTags = &codedError{code: "too_many_tags", msg: "too many tags (99 at most)"}
	colorRe        = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func (t *Tag) validate() error {
	t.Pattern = strings.TrimSpace(t.Pattern)
	t.Label = strings.TrimSpace(t.Label)
	if t.Pattern == "" {
		return &codedError{code: "pattern_required", msg: "the keyword is required"}
	}
	if !colorRe.MatchString(t.Color) {
		return &codedError{code: "invalid_color", msg: "invalid color (expected #rrggbb)"}
	}
	if t.Regex {
		if _, err := regexp.Compile(t.Pattern); err != nil {
			return &codedError{code: "invalid_regex", msg: "invalid regular expression", detail: err.Error()}
		}
	}
	return nil
}

// TagStore keeps the tags in a JSON file (in the /data volume).
type TagStore struct {
	mu   sync.Mutex
	path string
	tags []Tag
}

func LoadTagStore(path string) (*TagStore, error) {
	s := &TagStore{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.tags = defaultTags()
		return s, s.save()
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.tags); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if c1, c2 := migrateDefaultColors(s.tags), assignCodes(s.tags); c1 || c2 {
		if err := s.save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// save writes the file atomically; the caller holds the lock.
func (s *TagStore) save() error {
	b, err := json.MarshalIndent(s.tags, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *TagStore) List() []Tag {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Tag(nil), s.tags...)
}

func (s *TagStore) Create(t Tag) (Tag, error) {
	if err := t.validate(); err != nil {
		return t, err
	}
	t.ID = newID()
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.Code = freeCode(s.tags); t.Code == "" {
		return t, errTooManyTags
	}
	s.tags = append(s.tags, t)
	return t, s.save()
}

func (s *TagStore) Update(id string, t Tag) (Tag, error) {
	if err := t.validate(); err != nil {
		return t, err
	}
	t.ID = id
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tags {
		if s.tags[i].ID == id {
			t.Code = s.tags[i].Code // assigned by the server, never changed
			s.tags[i] = t
			return t, s.save()
		}
	}
	return t, errTagNotFound
}

func (s *TagStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tags {
		if s.tags[i].ID == id {
			s.tags = append(s.tags[:i], s.tags[i+1:]...)
			return s.save()
		}
	}
	return errTagNotFound
}

// tagListError tells which tag of a list (PUT /api/tags) is invalid.
type tagListError struct {
	index int
	err   error
}

func (e *tagListError) Error() string { return e.err.Error() }
func (e *tagListError) Unwrap() error { return e.err }

// Replace saves a whole list at once (the Save button of the settings). A tag
// whose id is known keeps its code; the others get a new id and a free code.
// Nothing is saved when a tag is invalid: the error gives its position.
func (s *TagStore) Replace(in []Tag) ([]Tag, error) {
	if len(in) > 99 {
		return nil, errTooManyTags
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	codes := map[string]string{}
	for _, t := range s.tags {
		codes[t.ID] = t.Code
	}
	out := make([]Tag, len(in))
	seen := map[string]bool{}
	for i, t := range in {
		if err := t.validate(); err != nil {
			return nil, &tagListError{index: i, err: err}
		}
		if c, ok := codes[t.ID]; ok && !seen[t.ID] {
			t.Code = c
		} else {
			t.ID, t.Code = newID(), ""
		}
		seen[t.ID] = true
		out[i] = t
	}
	for i := range out {
		if out[i].Code == "" {
			out[i].Code = freeCode(out)
		}
	}
	s.tags = out
	return append([]Tag(nil), out...), s.save()
}

func (s *TagStore) Reset() ([]Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags = defaultTags()
	return append([]Tag(nil), s.tags...), s.save()
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
