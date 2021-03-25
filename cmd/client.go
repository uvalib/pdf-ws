package main

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

type pdfRequest struct {
	pid   string
	unit  string
	pages string
	token string
	embed string
}

type pdfInfo struct {
	ts         *tsPidInfo // values looked up in tracksys
	solr       *solrInfo  // values looked up in solr
	subDir     string
	workSubDir string
	workDir    string
	embed      bool
}

type clientContext struct {
	ctx   *gin.Context
	reqID string     // unique request id for this connection
	ip    string     // client ip address
	req   pdfRequest // values from original request
	pdf   pdfInfo    // values derived while processing request
}

func newClientContext(ctx *gin.Context) *clientContext {
	c := clientContext{}
	c.init(ctx)
	return &c
}

func (c *clientContext) init(ctx *gin.Context) {
	c.ctx = ctx
	c.reqID = fmt.Sprintf("%08x", randomSource.Uint32())
	c.ip = c.ctx.ClientIP()

	c.req.pid = c.ctx.Param("pid")
	c.req.unit = c.ctx.Query("unit")
	c.req.pages = c.ctx.Query("pages")
	c.req.token = c.ctx.Query("token")
	c.req.embed = c.ctx.Query("embed")

	c.pdf.subDir = c.req.pid
	c.pdf.workSubDir = c.getWorkSubDir(c.pdf.subDir, c.req.unit, c.req.token)
	c.pdf.workDir = fmt.Sprintf("%s/%s", config.storageDir.value, c.pdf.workSubDir)

	c.pdf.embed = true
	if len(c.req.embed) == 0 || c.req.embed == "0" {
		c.pdf.embed = false
	}

	c.logRequest()
}

func (c *clientContext) log(format string, args ...interface{}) {
	parts := []string{
		fmt.Sprintf("[ip:%s]", c.ip),
		fmt.Sprintf("[req:%s]", c.reqID),
		fmt.Sprintf(format, args...),
	}

	log.Printf("%s", strings.Join(parts, " "))
}

func (c *clientContext) debug(format string, args ...interface{}) {
	c.log("DEBUG: "+format, args...)
}

func (c *clientContext) info(format string, args ...interface{}) {
	c.log("INFO: "+format, args...)
}

func (c *clientContext) warn(format string, args ...interface{}) {
	c.log("WARNING: "+format, args...)
}

func (c *clientContext) err(format string, args ...interface{}) {
	c.log("ERROR: "+format, args...)
}

func (c *clientContext) logRequest() {
	query := ""
	if c.ctx.Request.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", c.ctx.Request.URL.RawQuery)
	}

	c.log("REQUEST: %s %s%s", c.ctx.Request.Method, c.ctx.Request.URL.Path, query)
}

func (c *clientContext) logResponse(code int, msg string) {
	c.log("RESPONSE: status: %d (%s)", code, msg)
}

func (c *clientContext) respondString(code int, msg string) {
	c.logResponse(code, msg)
	c.ctx.String(code, msg)
}

func (c *clientContext) respondData(code int, contentType string, data []byte) {
	c.logResponse(code, contentType)
	c.ctx.Data(code, contentType, data)
}

func (c *clientContext) respondDataFromReader(code int, contentLength int64, contentType string, reader io.Reader, extraHeaders map[string]string) {
	c.logResponse(code, contentType)
	c.ctx.DataFromReader(code, contentLength, contentType, reader, extraHeaders)
}
