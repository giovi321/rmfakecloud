package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Extension is what a template file is called on the device.
const Extension = ".template"

// Blank is the template of a page with nothing drawn under it. The device
// ships no file for it.
const Blank = "Blank"

// Store reads templates from a directory, the way the device keeps them.
// A page names the file it wants, without the extension.
type Store struct {
	dir string

	mu     sync.Mutex
	parsed map[string]*Template
	missed map[string]bool
}

// NewStore reads templates from dir. An empty dir, or one that does not
// exist, simply has no templates in it.
func NewStore(dir string) *Store {
	return &Store{
		dir:    dir,
		parsed: make(map[string]*Template),
		missed: make(map[string]bool),
	}
}

// Dir is where this store keeps its templates.
func (s *Store) Dir() string { return s.dir }

// Template returns the template a page asked for. It returns nil, and no
// error, for a page that wants nothing drawn or names a template this server
// does not have: a page with no template underneath is what rmfakecloud has
// always produced, so it is not a failure.
func (s *Store) Template(name string) (*Template, error) {
	name = strings.TrimSpace(name)
	if s == nil || s.dir == "" || name == "" || name == Blank {
		return nil, nil
	}

	// A page names a file. Anything that could climb out of the directory is
	// not a name this server will look up.
	if name != filepath.Base(name) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("%q is not a template name", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if template, known := s.parsed[name]; known {
		return template, nil
	}
	if s.missed[name] {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(s.dir, name+Extension))
	if err != nil {
		if os.IsNotExist(err) {
			s.missed[name] = true
			return nil, nil
		}
		return nil, err
	}

	template, err := Parse(data)
	if err != nil {
		s.missed[name] = true
		return nil, fmt.Errorf("template %q: %w", name, err)
	}

	s.parsed[name] = template
	return template, nil
}

// Missing lists the templates that were asked for and not found, so a page
// that came out plain can be explained.
func (s *Store) Missing() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.missed))
	for name := range s.missed {
		names = append(names, name)
	}
	return names
}

// List names the templates in the store.
func (s *Store) List() ([]string, error) {
	if s == nil || s.dir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), Extension) {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), Extension))
	}
	return names, nil
}

// Add writes a template into the store, after checking it is one.
func (s *Store) Add(name string, data []byte) error {
	name = strings.TrimSpace(strings.TrimSuffix(name, Extension))
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		return fmt.Errorf("%q is not a template name", name)
	}

	if _, err := Parse(data); err != nil {
		return err
	}

	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, name+Extension), data, 0600); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.parsed, name)
	delete(s.missed, name)
	s.mu.Unlock()
	return nil
}

// Remove takes a template out of the store.
func (s *Store) Remove(name string) error {
	name = strings.TrimSpace(strings.TrimSuffix(name, Extension))
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		return fmt.Errorf("%q is not a template name", name)
	}

	if err := os.Remove(filepath.Join(s.dir, name+Extension)); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.parsed, name)
	delete(s.missed, name)
	s.mu.Unlock()
	return nil
}
