package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestTagCodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tags.json")
	// File written before codes existed, with a duplicate code.
	old := `[{"id":"a","pattern":"x","color":"#000000"},{"id":"b","code":"07","pattern":"y","color":"#000000"},{"id":"c","code":"07","pattern":"z","color":"#000000"}]`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadTagStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, tg := range s.List() {
		got = append(got, tg.Code)
	}
	if want := []string{"01", "07", "02"}; !slices.Equal(got, want) {
		t.Fatalf("migrated codes = %v, want %v", got, want)
	}

	n, err := s.Create(Tag{Pattern: "w", Color: "#000000"})
	if err != nil || n.Code != "03" {
		t.Fatalf("Create code = %q, %v; want 03", n.Code, err)
	}
	// The client cannot change a code.
	u, err := s.Update("b", Tag{Code: "42", Pattern: "y2", Color: "#000000"})
	if err != nil || u.Code != "07" {
		t.Fatalf("Update code = %q, %v; want 07", u.Code, err)
	}
	// A deleted tag frees its code; the others keep theirs.
	if err := s.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Create(Tag{Pattern: "v", Color: "#000000"}); n.Code != "01" {
		t.Fatalf("code after delete = %q, want 01", n.Code)
	}

	// Codes are saved in the file.
	s2, err := LoadTagStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.List()[0].Code; got != "07" {
		t.Fatalf("reloaded code = %q, want 07", got)
	}
}
