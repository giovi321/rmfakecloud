package exporter

import (
	"bytes"
	"fmt"
	"io"

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
}

// RenderPages renders pages to a PDF, each page through the renderer its own
// format needs, and lays the result over payload when the document has one.
//
// One document can hold more than one format. A notebook started before a
// device upgrade and continued after it holds both v5 and v6 pages, so the
// format has to be decided per page rather than per document.
func RenderPages(pages []Page, payload io.ReadSeeker, output io.Writer) error {
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

	annotations, err := renderAnnotations(pages, sizes)
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
func renderAnnotations(pages []Page, sizes []PageSize) ([]byte, error) {
	rendered := make([]io.ReadSeeker, 0, len(pages))
	failed := 0

	for i, page := range pages {
		var size PageSize
		if i < len(sizes) {
			size = sizes[i]
		}

		pdf, err := renderPage(page, size)
		if err != nil {
			// One unreadable page must not cost the reader the whole
			// document, so leave a blank page in its place and carry on.
			log.Warnf("page %d could not be rendered, leaving it blank: %v", i+1, err)
			failed++
			pdf, err = renderPage(Page{}, size)
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
func renderPage(page Page, size PageSize) ([]byte, error) {
	if page.Version == VersionV6 && page.Data != nil {
		buf := &bytes.Buffer{}
		if err := ExportV6ToPdfNative(page.Data, buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
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
