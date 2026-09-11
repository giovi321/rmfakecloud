package ui

import (
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/templates"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// maxTemplateSize is well past the largest template a device ships, and keeps
// an upload from filling the disk.
const maxTemplateSize = 1 << 20

// listTemplates lists the page templates this server can draw.
func (app *ReactAppWrapper) listTemplates(c *gin.Context) {
	names, err := app.templates.List()
	if err != nil {
		log.Error("[ui] cannot list templates: ", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	sort.Strings(names)
	c.JSON(http.StatusOK, gin.H{
		"templates": names,
		"directory": app.templates.Dir(),
	})
}

// createTemplate stores an uploaded template. The same file has to be
// installed on the tablet as well: the device never asks the server for one,
// it only names the template it drew.
func (app *ReactAppWrapper) createTemplate(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		badReq(c, "not multiform")
		return
	}

	files := form.File["file"]
	if len(files) == 0 {
		badReq(c, "no file")
		return
	}

	stored := make([]string, 0, len(files))
	for _, file := range files {
		if file.Size > maxTemplateSize {
			badReq(c, fmt.Sprintf("%s is too big for a template", file.Filename))
			return
		}

		opened, err := file.Open()
		if err != nil {
			log.Error("[ui] ", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		data, err := io.ReadAll(io.LimitReader(opened, maxTemplateSize))
		opened.Close()
		if err != nil {
			log.Error("[ui] ", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		name := common.Sanitize(file.Filename)
		if err := app.templates.Add(file.Filename, data); err != nil {
			log.Warnf("[ui] template %s refused: %v", name, err)
			badReq(c, err.Error())
			return
		}

		log.Infof("[ui] stored template %s", name)
		stored = append(stored, name)
	}

	c.JSON(http.StatusCreated, gin.H{"stored": stored})
}

// deleteTemplate removes a template. Pages that asked for it go back to being
// drawn plain.
func (app *ReactAppWrapper) deleteTemplate(c *gin.Context) {
	name := common.ParamS(templateNameParam, c)
	if name == "" {
		badReq(c, "no template")
		return
	}

	if err := app.templates.Remove(name); err != nil {
		log.Warnf("[ui] cannot remove template %s: %v", name, err)
		badReq(c, err.Error())
		return
	}

	log.Infof("[ui] removed template %s", name)
	c.Status(http.StatusNoContent)
}

// templateSource is what the exporter needs from the store.
var _ interface {
	Template(string) (*templates.Template, error)
} = (*templates.Store)(nil)
