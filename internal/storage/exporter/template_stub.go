//go:build !cairo

package exporter

import (
	"errors"
	"io"

	"github.com/ddvk/rmfakecloud/internal/templates"
)

func renderTemplate(shapes []templates.Shape, page PageSize, area templateArea, output io.Writer) error {
	return errors.New("drawing page templates requires building with Cairo support. Build with: go build -tags cairo")
}
