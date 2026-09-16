//go:build cairo

package exporter

import (
	"fmt"
	"io"
	"math"
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
		points := placeShape(shape, area, scaleX, scaleY)

		surface.Save()
		surface.NewPath()

		at := 0
		for _, segment := range shape.Segments {
			switch segment.Op {
			case 'M':
				surface.MoveTo(points[at].X, points[at].Y)
			case 'L':
				surface.LineTo(points[at].X, points[at].Y)
			case 'C':
				surface.CurveTo(
					points[at].X, points[at].Y,
					points[at+1].X, points[at+1].Y,
					points[at+2].X, points[at+2].Y)
			case 'Z':
				surface.ClosePath()
			}
			at += len(segment.Points)
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

// minVisibleMark is the smallest a filled mark is drawn, in points. The dot
// of a dot grid is one device pixel, which comes to under a third of a point:
// physically the same as on the tablet, but on a screen it lands inside a
// single pixel and all but disappears. Drawn at this size it reads as a dot.
const minVisibleMark = 0.75

// placeShape maps a shape onto the page, and grows a mark too small to see.
func placeShape(shape templates.Shape, area templateArea, scaleX, scaleY float64) []templates.Point {
	var points []templates.Point
	for _, segment := range shape.Segments {
		for _, p := range segment.Points {
			points = append(points, templates.Point{
				X: area.X + p.X*scaleX,
				Y: area.Y + p.Y*scaleY,
			})
		}
	}

	if shape.FillColor == "" || len(points) == 0 {
		return points
	}

	minX, maxX := points[0].X, points[0].X
	minY, maxY := points[0].Y, points[0].Y
	for _, p := range points[1:] {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}

	size := math.Max(maxX-minX, maxY-minY)
	if size <= 0 || size >= minVisibleMark {
		return points
	}

	// Grow it about its own middle, so it stays where it was drawn.
	grow := minVisibleMark / size
	centreX, centreY := (minX+maxX)/2, (minY+maxY)/2
	for i, p := range points {
		points[i] = templates.Point{
			X: centreX + (p.X-centreX)*grow,
			Y: centreY + (p.Y-centreY)*grow,
		}
	}
	return points
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
