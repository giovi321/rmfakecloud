package exporter

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"

	"github.com/ddvk/rmfakecloud/internal/templates"

	"github.com/ddvk/rmfakecloud/internal/archive"
	"github.com/ddvk/rmfakecloud/internal/encoding/rm"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	log "github.com/sirupsen/logrus"
)

// stampDescription places an annotation page on the page it belongs to at its
// own size. Both pages are rendered at the same size, so there is nothing to
// scale or move.
const stampDescription = "position:c, scalefactor:1 abs, rotation:0, opacity:1"

// fitDescription fits a stamp to the page it goes on. Used where the stamp has
// already been cut to the same shape as the page.
const fitDescription = "position:c, scalefactor:1 rel, rotation:0, opacity:1"

// PageSize is the size of one output page, in PDF points.
type PageSize struct {
	Width, Height float64
}

// Page is one page of a document, in the order the document declares.
type Page struct {
	// Data is the raw .rm page, or nil for a page the device never wrote.
	// Such a page still needs a place in the output, otherwise every page
	// after it shifts up.
	Data []byte
	// Version is the format of Data.
	Version RmVersion
	// Template is the name of the page template the device drew under the
	// ink, empty for a page that has none.
	Template string
}

// TemplateSource hands out page templates by name. A page whose template is
// not there gets none, which is what every page got before templates were
// read at all.
type TemplateSource interface {
	Template(name string) (*templates.Template, error)
}

// templateArea is where the device canvas lands on an output page, in points.
type templateArea struct {
	X, Y, Width, Height float64
}

// RenderPages renders pages to a PDF, each page through the renderer its own
// format needs, and lays the result over payload when the document has one.
//
// One document can hold more than one format. A notebook started before a
// device upgrade and continued after it holds both v5 and v6 pages, so the
// format has to be decided per page rather than per document.
func RenderPages(pages []Page, payload io.ReadSeeker, source TemplateSource, output io.Writer) error {
	if len(pages) == 0 {
		if payload != nil {
			_, err := io.Copy(output, payload)
			return err
		}
		return fmt.Errorf("the document has no pages")
	}

	sizes, err := payloadPageSizes(payload)
	if err != nil {
		log.Warnf("cannot read the page sizes of the document, annotations will use the device page size: %v", err)
		sizes = nil
	}

	annotations, err := renderAnnotations(pages, sizes, source)
	if err != nil {
		return err
	}

	if len(sizes) == 0 {
		_, err = output.Write(annotations)
		return err
	}

	stamped, err := stampOnPayload(annotations, payload)
	if err != nil {
		return err
	}

	if len(pages) <= len(sizes) {
		_, err = output.Write(stamped)
		return err
	}

	// The device lets pages be added past the end of an imported document.
	// They have nothing to sit on, so they go after it.
	return appendPagesPastTheEnd(stamped, annotations, len(sizes)+1, output)
}

// appendPagesPastTheEnd puts the annotation pages from `from` onwards after
// the stamped document.
func appendPagesPastTheEnd(stamped, annotations []byte, from int, output io.Writer) error {
	conf := model.NewDefaultConfiguration()

	past := &bytes.Buffer{}
	if err := api.Collect(bytes.NewReader(annotations), past, []string{fmt.Sprintf("%d-", from)}, conf); err != nil {
		return fmt.Errorf("failed to read the pages added past the end of the document: %w", err)
	}

	joined := &bytes.Buffer{}
	sources := []io.ReadSeeker{bytes.NewReader(stamped), bytes.NewReader(past.Bytes())}
	if err := api.MergeRaw(sources, joined, false, conf); err != nil {
		return fmt.Errorf("failed to append the pages added past the end of the document: %w", err)
	}

	flat, err := flattenPageTree(joined.Bytes())
	if err != nil {
		return err
	}

	_, err = output.Write(flat)
	return err
}

// flattenPageTree rebuilds the page tree. Joining documents nests it one level
// per document, and past a hundred or so levels readers that cap the depth
// refuse the file.
func flattenPageTree(pdf []byte) ([]byte, error) {
	flat := &bytes.Buffer{}
	conf := model.NewDefaultConfiguration()
	if err := api.Collect(bytes.NewReader(pdf), flat, []string{"1-"}, conf); err != nil {
		return nil, fmt.Errorf("failed to flatten the page tree: %w", err)
	}
	return flat.Bytes(), nil
}

// payloadPageSizes reads the page sizes of the document being annotated.
// It returns nil when there is no payload, or when the payload is not a PDF,
// which is the case for EPUBs.
func payloadPageSizes(payload io.ReadSeeker) ([]PageSize, error) {
	if payload == nil {
		return nil, nil
	}

	if _, err := payload.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	header := make([]byte, 5)
	if _, err := io.ReadFull(payload, header); err != nil {
		return nil, err
	}
	if !bytes.Equal(header, []byte("%PDF-")) {
		log.Info("the payload is not a PDF, exporting the annotations on their own")
		return nil, nil
	}

	if _, err := payload.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	dims, err := api.PageDims(payload, model.NewDefaultConfiguration())
	if err != nil {
		return nil, err
	}

	sizes := make([]PageSize, len(dims))
	for i, dim := range dims {
		sizes[i] = PageSize{Width: dim.Width, Height: dim.Height}
	}
	return sizes, nil
}

// renderAnnotations renders every page to a one page PDF and joins them in
// order. Rendering one page at a time is what lets each page use the renderer
// its own format needs.
func renderAnnotations(pages []Page, sizes []PageSize, source TemplateSource) ([]byte, error) {
	rendered := make([]io.ReadSeeker, 0, len(pages))
	failed := 0

	for i, page := range pages {
		var size PageSize
		if i < len(sizes) {
			size = sizes[i]
		}

		pdf, err := renderPage(page, size, source)
		if err != nil {
			// One unreadable page must not cost the reader the whole
			// document, so leave a blank page in its place and carry on.
			log.Warnf("page %d could not be rendered, leaving it blank: %v", i+1, err)
			failed++
			pdf, err = renderPage(Page{}, size, nil)
			if err != nil {
				return nil, fmt.Errorf("page %d: %w", i+1, err)
			}
		}
		rendered = append(rendered, bytes.NewReader(pdf))
	}

	if failed == len(pages) {
		return nil, fmt.Errorf("none of the %d pages could be rendered", len(pages))
	}

	if len(rendered) == 1 {
		single, err := io.ReadAll(rendered[0])
		return single, err
	}

	joined := &bytes.Buffer{}
	if err := api.MergeRaw(rendered, joined, false, model.NewDefaultConfiguration()); err != nil {
		return nil, fmt.Errorf("failed to join the rendered pages: %w", err)
	}

	return flattenPageTree(joined.Bytes())
}

// renderPage renders one page to a one page PDF. A page with no data renders
// blank, which is what a page the device never wrote should look like.
func renderPage(page Page, size PageSize, source TemplateSource) ([]byte, error) {
	if page.Version == VersionV6 && page.Data != nil {
		buf := &bytes.Buffer{}
		if err := ExportV6ToPdfNative(page.Data, buf); err != nil {
			return nil, err
		}
		if size.Width <= 0 || size.Height <= 0 {
			// A notebook: there is no document to line the ink up with, but
			// there may be a template to draw under it.
			return drawOnTemplate(page, buf.Bytes(), source)
		}
		return cutV6ToPage(page.Data, buf.Bytes(), size)
	}

	single := &MyArchive{Zip: archive.Zip{Pages: []archive.Page{{}}}}
	if page.Data != nil {
		decoded := rm.New()
		if err := decoded.UnmarshalBinary(page.Data); err != nil {
			return nil, err
		}
		single.Pages[0].Data = decoded
	}

	options := PdfGeneratorOptions{AllPages: true}
	if size.Width > 0 && size.Height > 0 {
		options.PageSizes = []PageSize{size}
	}

	buf := &bytes.Buffer{}
	generator := PdfGenerator{}
	if err := generator.Generate(single, buf, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// stampOnPayload lays each annotation page over the document page it belongs
// to, keeping the document itself rather than replacing it with the ink.
func stampOnPayload(annotations []byte, payload io.ReadSeeker) ([]byte, error) {
	stamp, err := api.PDFMultiWatermarkForReadSeeker(
		bytes.NewReader(annotations), 1, 1, stampDescription, true, false, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare the annotation overlay: %w", err)
	}

	if _, err := payload.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	stamped := &bytes.Buffer{}
	if err := api.AddWatermarks(payload, stamped, nil, stamp, model.NewDefaultConfiguration()); err != nil {
		return nil, fmt.Errorf("failed to lay the annotations over the document: %w", err)
	}
	return stamped.Bytes(), nil
}

// The device renders a page at these dimensions, and reMarkable's own screen
// is this many pixels, so ink coordinates are in this space.
const (
	deviceCanvasWidth  = 1404.0
	deviceCanvasHeight = 1872.0
	// devicePointScale is the points per canvas pixel that rmc-go renders at.
	devicePointScale = 72.0 / 226.0
)

// viewBoxPattern pulls the origin out of the SVG rmc-go writes.
var viewBoxPattern = regexp.MustCompile(`viewBox="(-?[0-9.]+) (-?[0-9.]+) (-?[0-9.]+) (-?[0-9.]+)"`)

// cutV6ToPage takes the ink of a v6 page and puts the part of it that sits on
// the document page onto a page of that size, so the ink lands where it was
// written.
//
// rmc-go sizes its output from the ink, not from the device canvas, and the
// canvas can sit anywhere inside it because the device lets writing run off
// the page in any direction. The SVG it writes carries the origin in its
// viewBox, which is the only way to find the canvas from outside the library.
//
// Ink written beside the page rather than on it is left out. It belongs to no
// page of the document.
func cutV6ToPage(rmData, ink []byte, size PageSize) ([]byte, error) {
	area, inkPage, err := v6Canvas(rmData, ink)
	if err != nil {
		log.Warnf("cannot place the ink on the page, leaving it at its own size: %v", err)
		return ink, nil
	}

	// Where the document page sits on the canvas. The device fits the page to
	// the canvas and centres it.
	fit := math.Min(deviceCanvasWidth/size.Width, deviceCanvasHeight/size.Height)
	pageW := size.Width * fit * devicePointScale
	pageH := size.Height * fit * devicePointScale
	pageX := area.X + (deviceCanvasWidth-size.Width*fit)/2*devicePointScale
	pageY := area.Y + (deviceCanvasHeight-size.Height*fit)/2*devicePointScale

	inkHeight := inkPage.Height

	// PDF boxes are measured from the bottom.
	lowerY := inkHeight - pageY - pageH
	boxDefinition := fmt.Sprintf("[%.2f %.2f %.2f %.2f]", pageX, lowerY, pageX+pageW, lowerY+pageH)
	box, err := api.Box(boxDefinition, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("failed to describe the page area of the ink: %w", err)
	}

	conf := model.NewDefaultConfiguration()
	cut := &bytes.Buffer{}
	if err := api.Crop(bytes.NewReader(ink), cut, nil, box, conf); err != nil {
		return nil, fmt.Errorf("failed to cut the ink to the page: %w", err)
	}

	blank, err := renderBlankPage(size)
	if err != nil {
		return nil, err
	}

	// The cut is the same shape as the page, so fitting it is exact.
	stamp, err := api.PDFWatermarkForReadSeeker(bytes.NewReader(cut.Bytes()), 1,
		fitDescription, true, false, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare the ink overlay: %w", err)
	}

	out := &bytes.Buffer{}
	if err := api.AddWatermarks(bytes.NewReader(blank), out, nil, stamp, conf); err != nil {
		return nil, fmt.Errorf("failed to put the ink on the page: %w", err)
	}
	return out.Bytes(), nil
}

// v6InkOrigin reads the top left corner of the rendered ink in its own
// coordinates, from the viewBox of the SVG rmc-go writes for the same page.
func v6InkOrigin(rmData []byte) (minX, minY float64, err error) {
	svg := &bytes.Buffer{}
	if err := ExportV6ToSvgNative(rmData, svg); err != nil {
		return 0, 0, err
	}

	match := viewBoxPattern.FindSubmatch(svg.Bytes())
	if match == nil {
		return 0, 0, fmt.Errorf("the rendered svg has no viewBox")
	}

	if minX, err = strconv.ParseFloat(string(match[1]), 64); err != nil {
		return 0, 0, err
	}
	if minY, err = strconv.ParseFloat(string(match[2]), 64); err != nil {
		return 0, 0, err
	}
	return minX, minY, nil
}

// pdfPageSize reads the size of the first page.
func pdfPageSize(pdf []byte) (PageSize, error) {
	dims, err := api.PageDims(bytes.NewReader(pdf), model.NewDefaultConfiguration())
	if err != nil {
		return PageSize{}, err
	}
	if len(dims) == 0 {
		return PageSize{}, fmt.Errorf("the rendered ink has no pages")
	}
	return PageSize{Width: dims[0].Width, Height: dims[0].Height}, nil
}

// v6Canvas works out where the device canvas sits inside a rendered ink page,
// and how big that page is. rmc-go sizes its output from the ink rather than
// from the canvas, so the canvas can be anywhere inside it.
func v6Canvas(rmData, ink []byte) (templateArea, PageSize, error) {
	minX, minY, err := v6InkOrigin(rmData)
	if err != nil {
		return templateArea{}, PageSize{}, err
	}

	page, err := pdfPageSize(ink)
	if err != nil {
		return templateArea{}, PageSize{}, err
	}

	return templateArea{
		X:      -deviceCanvasWidth/2*devicePointScale - minX,
		Y:      -minY,
		Width:  deviceCanvasWidth * devicePointScale,
		Height: deviceCanvasHeight * devicePointScale,
	}, page, nil
}

// drawOnTemplate puts the page template under the ink of a notebook page. The
// ink keeps its own page, so writing that ran off the template is not lost,
// and the template covers only the part of it the device page occupies.
func drawOnTemplate(page Page, ink []byte, source TemplateSource) ([]byte, error) {
	if source == nil || page.Template == "" {
		return ink, nil
	}

	template, err := source.Template(page.Template)
	if err != nil {
		log.Warnf("template %q could not be read, leaving the page plain: %v", page.Template, err)
		return ink, nil
	}
	if template == nil {
		return ink, nil
	}

	area, inkPage, err := v6Canvas(page.Data, ink)
	if err != nil {
		log.Warnf("cannot place template %q, leaving the page plain: %v", page.Template, err)
		return ink, nil
	}

	shapes, skippedText, err := template.Shapes(deviceCanvasWidth, deviceCanvasHeight)
	if err != nil {
		log.Warnf("template %q could not be drawn, leaving the page plain: %v", page.Template, err)
		return ink, nil
	}
	if skippedText > 0 {
		log.Debugf("template %q has %d text items, which need the device font", page.Template, skippedText)
	}
	if len(shapes) == 0 {
		return ink, nil
	}

	background := &bytes.Buffer{}
	if err := renderTemplate(shapes, inkPage, area, background); err != nil {
		log.Warnf("template %q could not be drawn, leaving the page plain: %v", page.Template, err)
		return ink, nil
	}

	// Both pages are the same size, so laying one on the other lines up.
	stamp, err := api.PDFWatermarkForReadSeeker(bytes.NewReader(ink), 1,
		stampDescription, true, false, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare the ink overlay: %w", err)
	}

	out := &bytes.Buffer{}
	if err := api.AddWatermarks(bytes.NewReader(background.Bytes()), out, nil, stamp,
		model.NewDefaultConfiguration()); err != nil {
		return nil, fmt.Errorf("failed to put the ink on the template: %w", err)
	}
	return out.Bytes(), nil
}

// renderBlankPage makes an empty page of the given size.
func renderBlankPage(size PageSize) ([]byte, error) {
	empty := &MyArchive{Zip: archive.Zip{Pages: []archive.Page{{}}}}
	buf := &bytes.Buffer{}
	generator := PdfGenerator{}
	options := PdfGeneratorOptions{AllPages: true, PageSizes: []PageSize{size}}
	if err := generator.Generate(empty, buf, options); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
