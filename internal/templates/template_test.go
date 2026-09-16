package templates

import (
	"math"
	"testing"
)

func TestEvaluate(t *testing.T) {
	names := map[string]float64{"templateWidth": 1404, "templateHeight": 1872, "magicOffset": -1}

	tests := []struct {
		expression string
		want       float64
	}{
		{"12", 12},
		{"19.5", 19.5},
		{"-5", -5},
		{"2 + 3 * 4", 14},
		{"(2 + 3) * 4", 20},
		{"templateWidth / 2", 702},
		{"templateWidth / 2 - templateHeight / 2 + magicOffset", -235},
		{"templateWidth > templateHeight ? 26 : 19.5", 19.5},
		{"templateHeight > templateWidth ? 26 : 19.5", 26},
		{"templateWidth < 1000 || templateHeight < 1000 ? 1 : 2", 2},
		{"templateWidth >= 1404 && templateHeight >= 1872 ? 1 : 2", 1},
		{"templateWidth != 1404 ? 1 : 2", 2},
	}

	for _, tt := range tests {
		got, err := evaluate(tt.expression, names)
		if err != nil {
			t.Errorf("%s: %v", tt.expression, err)
			continue
		}
		if math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("%s: got %v, want %v", tt.expression, got, tt.want)
		}
	}
}

func TestEvaluateRejectsNonsense(t *testing.T) {
	for _, expression := range []string{"2 +", "unknownName", "2 $ 3", "(2", "2 ? 3"} {
		if _, err := evaluate(expression, map[string]float64{}); err == nil {
			t.Errorf("%q was accepted", expression)
		}
	}
}

// dotsTemplate is shaped like the dot grids a reMarkable ships, written here
// rather than copied so the repository carries no device artwork.
const dotsTemplate = `{
	"name": "Dots",
	"orientation": "portrait",
	"constants": [
		{"mobileMaxWidth": 1000},
		{"mobileOffsetX": "templateWidth > templateHeight ? 26 : 19.5"},
		{"magicOffset": -1},
		{"magicOffsetX": "templateWidth / 2 - templateHeight / 2 + magicOffset"},
		{"defaultOffsetX": "templateWidth > templateHeight ? 0 : magicOffsetX"},
		{"offsetX": "templateWidth < mobileMaxWidth || templateHeight < mobileMaxWidth ? mobileOffsetX : defaultOffsetX"}
	],
	"items": [
		{
			"type": "group",
			"boundingBox": {"x": "offsetX - 2", "y": 0, "width": 78, "height": 78},
			"repeat": {"columns": "infinite", "rows": "infinite"},
			"children": [
				{
					"type": "path",
					"strokeWidth": 1,
					"fillColor": "#000000",
					"data": ["M", 0, 0, "L", 1, 0, "L", 1, 1, "L", 0, 1, "Z"]
				}
			]
		}
	]
}`

func TestDotGridTiles(t *testing.T) {
	template, err := Parse([]byte(dotsTemplate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	const width, height, step = 1404.0, 1872.0, 78.0

	shapes, skipped, err := template.Shapes(width, height)
	if err != nil {
		t.Fatalf("shapes: %v", err)
	}
	if skipped != 0 {
		t.Errorf("got %d skipped text items, want 0", skipped)
	}
	if len(shapes) == 0 {
		t.Fatal("the grid is empty")
	}

	// Every dot is the same little square.
	for i, shape := range shapes {
		if len(shape.Segments) != 5 {
			t.Fatalf("dot %d has %d segments, want 5", i, len(shape.Segments))
		}
		if shape.FillColor != "#000000" {
			t.Fatalf("dot %d fill is %q", i, shape.FillColor)
		}
	}

	// The grid has to cover the page, and start no more than one step before
	// it: tiling further out only wastes work on dots nobody sees.
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, shape := range shapes {
		p := shape.Segments[0].Points[0]
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}

	if minX > 0 || minX <= -step {
		t.Errorf("grid starts at x %v, want within one step before 0", minX)
	}
	if minY > 0 || minY <= -step {
		t.Errorf("grid starts at y %v, want within one step before 0", minY)
	}
	if maxX < width-step {
		t.Errorf("grid reaches x %v, short of the page width %v", maxX, width)
	}
	if maxY < height-step {
		t.Errorf("grid reaches y %v, short of the page height %v", maxY, height)
	}

	// Neighbouring dots are one step apart.
	columns := int(math.Round((maxX-minX)/step)) + 1
	if got := shapes[1].Segments[0].Points[0].X - minX; math.Abs(got-step) > 1e-9 {
		t.Errorf("dots are %v apart across, want %v", got, step)
	}
	if got := shapes[columns].Segments[0].Points[0].Y - minY; math.Abs(got-step) > 1e-9 {
		t.Errorf("rows are %v apart, want %v", got, step)
	}
}

func TestTextIsCountedNotDrawn(t *testing.T) {
	const withText = `{
		"name": "Planner",
		"items": [
			{"type": "text", "text": "Monday", "fontSize": 30},
			{"type": "path", "data": ["M", 0, 0, "L", 10, 0]}
		]
	}`

	template, err := Parse([]byte(withText))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	shapes, skipped, err := template.Shapes(1404, 1872)
	if err != nil {
		t.Fatalf("shapes: %v", err)
	}
	if skipped != 1 {
		t.Errorf("got %d skipped, want 1", skipped)
	}
	if len(shapes) != 1 {
		t.Errorf("got %d shapes, want 1", len(shapes))
	}
}

func TestParseRejectsEmpty(t *testing.T) {
	if _, err := Parse([]byte(`{"name":"x"}`)); err == nil {
		t.Error("a template that draws nothing was accepted")
	}
}
