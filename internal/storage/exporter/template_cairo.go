//go:build cairo

package exporter

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ddvk/rmfakecloud/internal/templates"
	"github.com/ungerik/go-cairo"
)

// renderTemplate draws a template onto a page of its own, ready for the ink to
// go over it. The template is drawn into the rectangle the device page
// occupies on that page, given in points, so the lines land under the writing
// rather than across the whole sheet.
func renderTemplate(shapes []templates.Shape, page PageSize, area templateArea, output io.Writer) error {
	file, err := os.CreateTemp("", "rmfakecloud-template-*.pdf")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	path := file.Name()
	file.Close()
	defer os.Remove(path)

	surface := cairo.NewPDFSurface(path, page.Width, page.Height, cairo.PDF_VERSION_1_5)

	// Template coordinates are device pixels, the page is points.
	scaleX := area.Width / deviceCanvasWidth
	scaleY := area.Height / deviceCanvasHeight

	for _, shape := range shapes {
		surface.Save()
		surface.NewPath()

		for _, segment := range shape.Segments {
			switch segment.Op {
			case 'M':
				p := segment.Points[0]
				surface.MoveTo(area.X+p.X*scaleX, area.Y+p.Y*scaleY)
			case 'L':
				p := segment.Points[0]
				surface.LineTo(area.X+p.X*scaleX, area.Y+p.Y*scaleY)
			case 'C':
				a, b, c := segment.Points[0], segment.Points[1], segment.Points[2]
				surface.CurveTo(
					area.X+a.X*scaleX, area.Y+a.Y*scaleY,
					area.X+b.X*scaleX, area.Y+b.Y*scaleY,
					area.X+c.X*scaleX, area.Y+c.Y*scaleY)
			case 'Z':
				surface.ClosePath()
			}
		}

		if shape.FillColor != "" {
			r, g, b := parseColor(shape.FillColor)
			surface.SetSourceRGB(r, g, b)
			surface.FillPreserve()
		}

		if shape.StrokeColor != "" || shape.FillColor == "" {
			colour := shape.StrokeColor
			if colour == "" {
				colour = "#000000"
			}
			r, g, b := parseColor(colour)
			surface.SetSourceRGB(r, g, b)
			surface.SetLineWidth(shape.StrokeWidth * scaleX)
			surface.Stroke()
		} else {
			surface.NewPath()
		}

		surface.Restore()
	}

	surface.ShowPage()
	surface.Finish()

	rendered, err := os.Open(path)
	if err != nil {
		return err
	}
	defer rendered.Close()

	_, err = io.Copy(output, rendered)
	return err
}

// parseColor reads #rrggbb. Anything else is black, which is what every
// template line is anyway.
func parseColor(value string) (r, g, b float64) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return 0, 0, 0
	}
	channel := func(part string) float64 {
		n, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return 0
		}
		return float64(n) / 255
	}
	return channel(value[0:2]), channel(value[2:4]), channel(value[4:6])
}
