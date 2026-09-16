// Package templates renders the page templates a reMarkable draws under
// handwriting. A template is a small vector document: constants, groups that
// tile across the page, and paths whose coordinates are expressions over the
// page size.
package templates

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// maxRepeats bounds a tiling that would otherwise run away on a bad box size.
const maxRepeats = 4096

// Template is a parsed template file.
type Template struct {
	Name        string           `json:"name"`
	Orientation string           `json:"orientation"`
	Constants   []map[string]any `json:"constants"`
	Items       []Item           `json:"items"`
}

// Item is one element of a template. A group positions and tiles its children,
// a path draws, and text is not drawn: it needs the device font.
type Item struct {
	Type        string  `json:"type"`
	BoundingBox *Box    `json:"boundingBox"`
	Repeat      *Repeat `json:"repeat"`
	Children    []Item  `json:"children"`
	Data        []any   `json:"data"`
	StrokeWidth any     `json:"strokeWidth"`
	FillColor   string  `json:"fillColor"`
	StrokeColor string  `json:"strokeColor"`
	Text        string  `json:"text"`
}

// Box is the position and size of a group, in template coordinates.
type Box struct {
	X      any `json:"x"`
	Y      any `json:"y"`
	Width  any `json:"width"`
	Height any `json:"height"`
}

// Repeat tiles a group. A count is a number, or one of the words the device
// uses: "infinite" to cover the page, or "down", "up", "left" and "right" to
// carry on in one direction only.
type Repeat struct {
	Columns any `json:"columns"`
	Rows    any `json:"rows"`
}

// Point is a resolved coordinate.
type Point struct {
	X, Y float64
}

// Segment is one resolved path command. Op is M, L, C or Z, and Points holds
// as many points as that command takes.
type Segment struct {
	Op     byte
	Points []Point
}

// Shape is a resolved path ready to draw.
type Shape struct {
	Segments    []Segment
	FillColor   string
	StrokeColor string
	StrokeWidth float64
}

// Parse reads a template file.
func Parse(data []byte) (*Template, error) {
	template := &Template{}
	if err := json.Unmarshal(data, template); err != nil {
		return nil, fmt.Errorf("not a template: %w", err)
	}
	if len(template.Items) == 0 {
		return nil, fmt.Errorf("the template draws nothing")
	}
	return template, nil
}

// Shapes resolves the template for a page of this size. Text items are left
// out and counted, because drawing them needs the device font.
func (t *Template) Shapes(width, height float64) (shapes []Shape, skippedText int, err error) {
	names := map[string]float64{
		"templateWidth":  width,
		"templateHeight": height,
		"parentWidth":    width,
		"parentHeight":   height,
		// paperOriginX is the device's own name for the quantity other
		// templates write out as templateWidth / 2 - templateHeight / 2.
		// Read off the two templates that use it rather than documented:
		// taking it this way puts a dot of the small grid exactly on the
		// left edge of the page, which is what the grid is drawn around.
		"paperOriginX": width/2 - height/2,
	}

	for _, constant := range t.Constants {
		for name, value := range constant {
			resolved, err := number(value, names)
			if err != nil {
				return nil, 0, fmt.Errorf("constant %s: %w", name, err)
			}
			names[name] = resolved
		}
	}

	collector := &collector{width: width, height: height}
	if err := collector.items(t.Items, names, 0, 0); err != nil {
		return nil, 0, err
	}
	return collector.shapes, collector.skippedText, nil
}

type collector struct {
	width, height float64
	shapes        []Shape
	skippedText   int
}

func (c *collector) items(items []Item, names map[string]float64, offsetX, offsetY float64) error {
	for _, item := range items {
		switch item.Type {
		case "group":
			if err := c.group(item, names, offsetX, offsetY); err != nil {
				return err
			}
		case "path":
			shape, err := c.path(item, names, offsetX, offsetY)
			if err != nil {
				return err
			}
			if len(shape.Segments) > 0 {
				c.shapes = append(c.shapes, shape)
			}
		case "text":
			c.skippedText++
		}
	}
	return nil
}

func (c *collector) group(item Item, names map[string]float64, offsetX, offsetY float64) error {
	box := item.BoundingBox
	if box == nil {
		return c.items(item.Children, names, offsetX, offsetY)
	}

	x, err := number(box.X, names)
	if err != nil {
		return err
	}
	y, err := number(box.Y, names)
	if err != nil {
		return err
	}
	boxWidth, err := number(box.Width, names)
	if err != nil {
		return err
	}
	boxHeight, err := number(box.Height, names)
	if err != nil {
		return err
	}

	firstColumn, pastColumn, err := c.repeatRange(item.Repeat, true, boxWidth, x, names)
	if err != nil {
		return err
	}
	firstRow, pastRow, err := c.repeatRange(item.Repeat, false, boxHeight, y, names)
	if err != nil {
		return err
	}

	// Children see the group box as their parent.
	inner := make(map[string]float64, len(names)+2)
	for name, value := range names {
		inner[name] = value
	}
	inner["parentWidth"] = boxWidth
	inner["parentHeight"] = boxHeight

	for row := firstRow; row < pastRow; row++ {
		for column := firstColumn; column < pastColumn; column++ {
			tileX := offsetX + x + float64(column)*boxWidth
			tileY := offsetY + y + float64(row)*boxHeight
			if err := c.items(item.Children, inner, tileX, tileY); err != nil {
				return err
			}
		}
	}
	return nil
}

// repeatRange works out which copies of a group to draw along one axis, as a
// half open range of step offsets. A group with no repeat is drawn once, at 0.
func (c *collector) repeatRange(repeat *Repeat, horizontal bool, step, start float64, names map[string]float64) (from, to int, err error) {
	if repeat == nil {
		return 0, 1, nil
	}

	value := repeat.Rows
	extent := c.height
	if horizontal {
		value = repeat.Columns
		extent = c.width
	}
	if value == nil {
		return 0, 1, nil
	}

	// How many steps from the start it takes to reach each edge of the page.
	toStart, toEnd := 0, 1
	if step > 0 {
		toStart = int(math.Floor((0 - start) / step))
		toEnd = int(math.Ceil((extent-start)/step)) + 1
	}

	if word, ok := value.(string); ok {
		switch strings.ToLower(strings.TrimSpace(word)) {
		case "infinite":
			from, to = toStart, toEnd
		case "down", "right":
			from, to = 0, toEnd
		case "up", "left":
			from, to = toStart, 1
		default:
			// Not a word, so it has to be an expression.
			count, err := number(value, names)
			if err != nil {
				return 0, 0, err
			}
			from, to = 0, int(count)
		}
	} else {
		count, err := number(value, names)
		if err != nil {
			return 0, 0, err
		}
		from, to = 0, int(count)
	}

	if to <= from {
		return 0, 1, nil
	}
	if to-from > maxRepeats {
		to = from + maxRepeats
	}
	return from, to, nil
}

// pathOperands is how many points each path command takes.
var pathOperands = map[byte]int{'M': 1, 'L': 1, 'C': 3, 'Z': 0}

func (c *collector) path(item Item, names map[string]float64, offsetX, offsetY float64) (Shape, error) {
	shape := Shape{
		FillColor:   item.FillColor,
		StrokeColor: item.StrokeColor,
		StrokeWidth: 1,
	}

	if item.StrokeWidth != nil {
		width, err := number(item.StrokeWidth, names)
		if err != nil {
			return shape, err
		}
		shape.StrokeWidth = width
	}

	values := item.Data
	for i := 0; i < len(values); {
		command, ok := pathCommand(values[i])
		if !ok {
			return shape, fmt.Errorf("expected a path command, found %v", values[i])
		}
		i++

		points := pathOperands[command]
		segment := Segment{Op: command}

		for p := 0; p < points; p++ {
			if i+1 >= len(values) {
				return shape, fmt.Errorf("path command %q is missing coordinates", string(command))
			}
			x, err := number(values[i], names)
			if err != nil {
				return shape, err
			}
			y, err := number(values[i+1], names)
			if err != nil {
				return shape, err
			}
			i += 2
			segment.Points = append(segment.Points, Point{X: offsetX + x, Y: offsetY + y})
		}

		shape.Segments = append(shape.Segments, segment)
	}

	return shape, nil
}

// pathCommand tells a command apart from a coordinate. Coordinates can be
// expressions, so a string is only a command when it is one of the letters.
func pathCommand(value any) (byte, bool) {
	text, ok := value.(string)
	if !ok || len(text) != 1 {
		return 0, false
	}
	command := text[0]
	if _, known := pathOperands[command]; known {
		return command, true
	}
	return 0, false
}
