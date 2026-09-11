package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealTemplates resolves whatever templates are in the directory named by
// RMFAKECLOUD_TEMPLATE_DIR. Device templates are not in this repository, so
// point it at a copy of /usr/share/remarkable/templates from a tablet to check
// the parser against the real thing.
func TestRealTemplates(t *testing.T) {
	dir := os.Getenv("RMFAKECLOUD_TEMPLATE_DIR")
	if dir == "" {
		t.Skip("set RMFAKECLOUD_TEMPLATE_DIR to a directory of device templates")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var resolved, withText int
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".template") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}

			template, err := Parse(data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			width, height := 1404.0, 1872.0
			if strings.EqualFold(template.Orientation, "landscape") {
				width, height = height, width
			}

			shapes, skipped, err := template.Shapes(width, height)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if len(shapes) == 0 && skipped == 0 {
				t.Error("resolved to nothing at all")
			}

			resolved++
			if skipped > 0 {
				withText++
			}
			t.Logf("%d shapes, %d text items skipped", len(shapes), skipped)
		})
	}

	t.Logf("resolved %d templates, %d of them contain text", resolved, withText)
}
