//go:build cairo

package exporter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unsafe"

	"github.com/ddvk/rmfakecloud/internal/encoding/rm"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/sirupsen/logrus"
	"github.com/ungerik/go-cairo"
)

/*
#cgo pkg-config: cairo
#include <stdlib.h>
#include <cairo.h>
#include <cairo-pdf.h>
*/
import "C"

const (
	DeviceWidth  = 1404
	DeviceHeight = 1872
)

// rmPageSize is the default page size for blank templates (in PDF points: 1/72 inch)
var rmPageSize = struct{ Width, Height float64 }{445, 594}

type PdfGenerator struct {
	options       PdfGeneratorOptions
	backgroundPDF []byte
	template      bool
}

type PdfGeneratorOptions struct {
	AddPageNumbers  bool
	AllPages        bool
	AnnotationsOnly bool //export the annotations without the background/pdf
	// PageSizes is the size of each output page, indexed by page. Pages past
	// the end of the slice, and entries that are not positive, fall back to
	// the device page size. Set it to the page sizes of the document being
	// annotated so the annotations line up with it.
	PageSizes []PageSize
}

// pageSize is the output size of page index, in PDF points.
func (p *PdfGenerator) pageSize(index int) (width, height float64) {
	if index < len(p.options.PageSizes) {
		size := p.options.PageSizes[index]
		if size.Width > 0 && size.Height > 0 {
			return size.Width, size.Height
		}
	}
	return rmPageSize.Width, rmPageSize.Height
}

// pageTransform maps a point of ink onto the page, in surface coordinates.
//
// A landscape page does not fit a portrait screen the right way up, so the
// device turns it a quarter turn: the page's width then runs along the long
// side of the screen and the ink is written turned with it. Checked against a
// page the tablet exported itself, where this puts the ink within a fraction
// of a percent of where the tablet has it.
type pageTransform struct {
	scale      float64
	pageHeight float64
	turned     bool
}

func (t pageTransform) apply(point rm.Point) (x, y float64) {
	if t.turned {
		return (DeviceHeight - float64(point.Y)) * t.scale, float64(point.X) * t.scale
	}
	return float64(point.X) * t.scale, t.pageHeight - float64(point.Y)*t.scale
}

// transformFor works out how the ink of a page this size is laid onto it.
func transformFor(pageWidth, pageHeight float64) pageTransform {
	if pageWidth > pageHeight {
		return pageTransform{
			scale:      pageWidth / DeviceHeight,
			pageHeight: pageHeight,
			turned:     true,
		}
	}

	scale := pageHeight / DeviceHeight
	if pageHeight/pageWidth < 1.33 {
		scale = pageWidth / DeviceWidth
	}
	return pageTransform{scale: scale, pageHeight: pageHeight}
}

// setPDFPageSize sets the size for the current page in a PDF surface
func setPDFPageSize(surface *cairo.Surface, width, height float64) {
	surfacePtr, _ := surface.Native()
	C.cairo_pdf_surface_set_size((*C.cairo_surface_t)(unsafe.Pointer(surfacePtr)), C.double(width), C.double(height))
}

func (p *PdfGenerator) Generate(zip *MyArchive, output io.Writer, options PdfGeneratorOptions) error {
	p.options = options

	if len(zip.Pages) == 0 {
		if zip.PayloadReader != nil {
			_, err := io.Copy(output, zip.PayloadReader)
			return err
		}
		return fmt.Errorf("the document has no pages")
	}

	if err := p.initBackgroundPages(zip.PayloadReader); err != nil {
		return err
	}

	// If we have a background PDF and not annotations-only mode, we need a two-step process
	if p.backgroundPDF != nil && !p.options.AnnotationsOnly {
		return p.generateWithBackground(zip, output)
	}

	// Otherwise, simple case: just annotations or blank pages
	return p.generateAnnotationsOnly(zip, output)
}

func (p *PdfGenerator) generateAnnotationsOnly(zip *MyArchive, output io.Writer) error {
	// Create a temporary file for PDF output (Cairo requires a file path)
	tmpFile, err := os.CreateTemp("", "rmfakecloud-annotations-*.pdf")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Determine first page dimensions
	firstWidth, firstHeight := p.pageSize(0)

	// Create PDF surface
	pdfSurface := cairo.NewPDFSurface(tmpPath, firstWidth, firstHeight, cairo.PDF_VERSION_1_5)
	defer pdfSurface.Finish()

	pageCount := 0
	for index, pageAnnotations := range zip.Pages {
		hasContent := pageAnnotations.Data != nil

		// Skip pages without content unless AllPages is set
		if !p.options.AllPages && !hasContent {
			continue
		}

		pageCount++

		pageWidth, pageHeight := p.pageSize(index)

		// Set page size (for pages after the first)
		if pageCount > 1 {
			setPDFPageSize(pdfSurface, pageWidth, pageHeight)
		}

		transform := transformFor(pageWidth, pageHeight)

		// Draw annotations if present
		if hasContent {
			if err := p.drawAnnotations(pdfSurface, pageAnnotations.Data, transform); err != nil {
				return err
			}
		}

		// Add page numbers if requested
		if p.options.AddPageNumbers {
			p.drawPageNumber(pdfSurface, pageCount, pageWidth, pageHeight)
		}

		// Show page (prepare for next page)
		if pageCount < len(zip.Pages) || p.options.AllPages {
			pdfSurface.ShowPage()
		}
	}

	pdfSurface.Finish()

	// Copy temp file to output
	tmpFileRead, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer tmpFileRead.Close()

	_, err = io.Copy(output, tmpFileRead)
	return err
}

func (p *PdfGenerator) generateWithBackground(zip *MyArchive, output io.Writer) error {
	// Render the annotation pages at the size of the pages they go over.
	// Anything else and the ink lands in the wrong place.
	dims, err := api.PageDims(bytes.NewReader(p.backgroundPDF), model.NewDefaultConfiguration())
	if err != nil {
		return fmt.Errorf("failed to read the page sizes of the document: %w", err)
	}

	sizes := make([]PageSize, len(dims))
	for i, dim := range dims {
		sizes[i] = PageSize{Width: dim.Width, Height: dim.Height}
	}
	p.options.PageSizes = sizes

	annotations := &bytes.Buffer{}
	if err := p.generateAnnotationsOnly(zip, annotations); err != nil {
		return err
	}

	stamped, err := stampOnPayload(annotations.Bytes(), bytes.NewReader(p.backgroundPDF))
	if err != nil {
		return err
	}

	_, err = output.Write(stamped)
	return err
}

func (p *PdfGenerator) drawAnnotations(surface *cairo.Surface, rmData *rm.Rm, transform pageTransform) error {
	surface.Save()
	defer surface.Restore()

	for _, layer := range rmData.Layers {
		for _, line := range layer.Lines {
			if len(line.Points) < 1 {
				continue
			}
			if line.BrushType == rm.Eraser || line.BrushType == rm.EraseArea {
				continue
			}

			if line.BrushType == rm.HighlighterV5 {
				// Draw highlighter as semi-transparent rectangle
				p.drawHighlighter(surface, line, transform)
			} else {
				// Draw regular stroke
				p.drawStroke(surface, line, transform)
			}
		}
	}

	return nil
}

func (p *PdfGenerator) drawHighlighter(surface *cairo.Surface, line rm.Line, transform pageTransform) {
	if len(line.Points) < 2 {
		return
	}

	last := len(line.Points) - 1
	x1, y1 := transform.apply(line.Points[0])
	x2, y2 := transform.apply(line.Points[last])

	width := transform.scale * 30

	// A highlighter stroke is one straight swipe, so it is drawn flat along
	// whichever way the page runs.
	if transform.turned {
		x2 = x1
	} else {
		y2 = y1
	}

	// Yellow color with 50% opacity
	surface.SetSourceRGBA(1.0, 1.0, 0.0, 0.5)
	surface.SetLineWidth(width)
	surface.SetLineCap(cairo.LINE_CAP_BUTT)

	surface.MoveTo(x1, y1)
	surface.LineTo(x2, y2)
	surface.Stroke()
}

func (p *PdfGenerator) drawStroke(surface *cairo.Surface, line rm.Line, transform pageTransform) {
	if len(line.Points) < 1 {
		return
	}

	// Set stroke color
	var r, g, b float64
	switch line.BrushColor {
	case rm.Black:
		r, g, b = 0.0, 0.0, 0.0
	case rm.White:
		r, g, b = 1.0, 1.0, 1.0
	case rm.Grey:
		r, g, b = 0.5, 0.5, 0.5
	default:
		r, g, b = 0.0, 0.0, 0.0
	}
	surface.SetSourceRGB(r, g, b)

	// Set stroke width
	// Formula from original: line.BrushSize*6.0 - 10.8
	strokeWidth := float64(line.BrushSize)*6.0 - 10.8
	if strokeWidth < 0.5 {
		strokeWidth = 0.5
	}
	surface.SetLineWidth(strokeWidth)

	// Set line cap
	surface.SetLineCap(cairo.LINE_CAP_ROUND)
	surface.SetLineJoin(cairo.LINE_JOIN_ROUND)

	// Draw path
	for i, point := range line.Points {
		x, y := transform.apply(point)

		if i == 0 {
			surface.MoveTo(x, y)
		} else {
			surface.LineTo(x, y)
		}
	}

	surface.Stroke()
}

func (p *PdfGenerator) drawPageNumber(surface *cairo.Surface, pageNum int, pageWidth, pageHeight float64) {
	surface.Save()
	defer surface.Restore()

	surface.SelectFontFace("sans-serif", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	surface.SetFontSize(8.0)
	surface.SetSourceRGB(0, 0, 0)

	text := fmt.Sprintf("%d", pageNum)
	surface.MoveTo(pageWidth-20, pageHeight-10)
	surface.ShowText(text)
}

func (p *PdfGenerator) initBackgroundPages(r io.ReadSeeker) error {
	if r != nil {
		// Read the PDF into memory
		pdfBytes, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("failed to read background PDF: %w", err)
		}

		// Check if PDF is encrypted and handle with pdfcpu
		rs := bytes.NewReader(pdfBytes)
		ctx, err := api.ReadContext(rs, model.NewDefaultConfiguration())
		if err != nil {
			return fmt.Errorf("failed to read PDF: %w", err)
		}

		// Check if encrypted by checking if Encrypt field exists
		if ctx.XRefTable.Encrypt != nil {
			logrus.Info("PDF is encrypted - pdfcpu will handle decryption")
			// pdfcpu's ReadContext already handles decryption with empty password
		}

		p.backgroundPDF = pdfBytes
		p.template = false
		return nil
	}

	logrus.Info("template")
	p.template = true
	return nil
}
