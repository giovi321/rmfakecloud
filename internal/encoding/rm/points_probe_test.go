package rm

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

// TestDumpPoints writes the stroke points of the .rm file named by
// RMFAKECLOUD_RM_FILE to RMFAKECLOUD_POINTS_OUT, one "x y" per line. A probe
// for working out how a page's ink maps onto the document it annotates.
func TestDumpPoints(t *testing.T) {
	path := os.Getenv("RMFAKECLOUD_RM_FILE")
	out := os.Getenv("RMFAKECLOUD_POINTS_OUT")
	if path == "" || out == "" {
		t.Skip("set RMFAKECLOUD_RM_FILE and RMFAKECLOUD_POINTS_OUT")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	page := New()
	if err := page.UnmarshalBinary(data); err != nil {
		t.Fatal(err)
	}

	file, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	defer w.Flush()

	written := 0
	for _, layer := range page.Layers {
		for _, line := range layer.Lines {
			if line.BrushType == Eraser || line.BrushType == EraseArea {
				continue
			}
			for _, p := range line.Points {
				fmt.Fprintf(w, "%.3f %.3f\n", p.X, p.Y)
				written++
			}
		}
	}
	t.Logf("wrote %d points", written)
}
