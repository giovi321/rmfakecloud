package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "P Dots"+Extension), []byte(dotsTemplate), 0600); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)

	template, err := store.Template("P Dots")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if template == nil {
		t.Fatal("the template that is there was not found")
	}

	// Blank draws nothing and the device ships no file for it.
	if template, err := store.Template(Blank); err != nil || template != nil {
		t.Errorf("Blank: got %v, %v, want nil, nil", template, err)
	}

	// A template this server does not have leaves the page plain rather than
	// failing the export.
	if template, err := store.Template("Something Else"); err != nil || template != nil {
		t.Errorf("unknown: got %v, %v, want nil, nil", template, err)
	}
	if missing := store.Missing(); len(missing) != 1 || missing[0] != "Something Else" {
		t.Errorf("missing: got %v", missing)
	}

	names, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "P Dots" {
		t.Errorf("list: got %v", names)
	}
}

func TestStoreAddChecksTheFile(t *testing.T) {
	store := NewStore(t.TempDir())

	if _, err := store.Add("Bad", []byte("not a template")); err == nil {
		t.Error("a file that is not a template was accepted")
	}
	stored, err := store.Add("Good"+Extension, []byte(dotsTemplate))
	if err != nil {
		t.Errorf("a real template was refused: %v", err)
	}
	if stored != "Good" {
		t.Errorf("stored as %q, want %q", stored, "Good")
	}
	if template, err := store.Template("Good"); err != nil || template == nil {
		t.Errorf("the added template was not found back: %v, %v", template, err)
	}
}

func TestNilStoreIsHarmless(t *testing.T) {
	var store *Store
	if template, err := store.Template("anything"); err != nil || template != nil {
		t.Errorf("got %v, %v", template, err)
	}
	if names, err := store.List(); err != nil || names != nil {
		t.Errorf("got %v, %v", names, err)
	}
}

func TestNameFromFile(t *testing.T) {
	tests := map[string]string{
		"P Dots large" + Extension: "P Dots large",
		"P Dots large":             "P Dots large",
		"  Spaced  ":               "Spaced",
		"/tmp/Grid" + Extension:    "Grid",
		`C:\Users\me\Grid` + Extension: "Grid",
	}

	for filename, want := range tests {
		if got := NameFromFile(filename); got != want {
			t.Errorf("NameFromFile(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestStoreLooksUpNamesOnly(t *testing.T) {
	store := NewStore(t.TempDir())

	// A page names a file. A page asking for a path is not a page this
	// server will follow.
	for _, name := range []string{"../escape", "sub/dir", `..\escape`} {
		if _, err := store.Template(name); err == nil {
			t.Errorf("Template(%q) was accepted", name)
		}
	}
}

func TestStoreKeepsUploadsInItsOwnDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// A browser is free to send a whole path as the file name. Whatever it
	// sends, the file lands in the store under the last part of it.
	for _, filename := range []string{
		"../escape" + Extension,
		"sub/dir" + Extension,
		`C:\Users\me\Grid` + Extension,
		"/etc/passwd" + Extension,
	} {
		name, err := store.Add(filename, []byte(dotsTemplate))
		if err != nil {
			t.Errorf("Add(%q): %v", filename, err)
			continue
		}
		if name != filepath.Base(name) || strings.Contains(name, "..") {
			t.Errorf("Add(%q) stored as %q", filename, name)
		}
	}

	// Nothing was written anywhere but the store.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("Add created the directory %q", entry.Name())
		}
	}
}
