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
	Pattern       string `json:"pattern"`
	Color         string `json:"color"`
	WholeWord     bool   `json:"wholeWord"`
	CaseSensitive bool   `json:"caseSensitive"`
	Regex         bool   `json:"regex"`
	Enabled       bool   `json:"enabled"`
}

func defaultTags() []Tag {
	return []Tag{
		{ID: "warning", Pattern: "warning", Color: "#f59e0b", WholeWord: true, Enabled: true},
		{ID: "error", Pattern: "error", Color: "#ef4444", WholeWord: true, Enabled: true},
		{ID: "ok", Pattern: "ok", Color: "#22c55e", WholeWord: true, Enabled: true},
	}
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
	colorRe        = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func (t *Tag) validate() error {
	t.Pattern = strings.TrimSpace(t.Pattern)
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
