package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (c *clientContext) inProgress() {
	if c.pdf.embed == true {
		c.respondString(http.StatusOK, "ok")
	} else {
		if ajax, err := c.renderAjaxPage(); err != nil {
			c.respondString(http.StatusInternalServerError, err.Error())
		} else {
			c.respondData(http.StatusOK, "text/html; charset=utf-8", []byte(ajax))
		}
	}
}

/**
 * Handle a request for a PDF of page images
 */
func generateHandler(ctx *gin.Context) {
	c := newClientContext(ctx)

	if c.req.pages != "" && c.req.token == "" {
		c.err("request for partial PDF is missing a token")
		c.respondString(http.StatusBadRequest, "Missing token")
		return
	}

	// see if a previous attempt failed; if so, transparently try again
	if c.isFailed() == true {
		c.info("found pdf in failed state; clearing it out and trying again")
		if err := c.removeWorkDir(3, 5); err != nil {
			c.warn("failed to clear out previous failure")
		}
	}

	// See if destination already exists...
	if c.progressInValidState() == true {
		// path already exists; don't start another request, just treat this one
		// as if it was complete (whether successful or not) and render the ajax page
		c.inProgress()
		return
	}

	if res := c.tsGetPidInfo(); res.err != nil {
		switch res.status {
		case http.StatusNotFound:
			c.warn("tracksys API: %s", res.err.Error())
			c.respondString(res.status, fmt.Sprintf("WARNING: Could not retrieve PID info: %s", res.err.Error()))

		default:
			c.err("tracksys API: %s", res.err.Error())
			c.respondString(res.status, fmt.Sprintf("ERROR: Could not retrieve PID info: %s", res.err.Error()))
		}

		return
	}

	if err := c.solrGetInfo(); err != nil {
		c.warn("solr error: %s", err.Error())
		c.warn("generating PDF without a cover page in directory: %s", c.pdf.workDir)
	}

	// Make sure the work directory exists, AND has something recognized by progressInValidState()
	// in case status endpoint is called before everything is set up and in a good state
	if err := os.MkdirAll(c.pdf.workDir, 0755); err != nil {
		c.err("failed to create working directory [%s]: %s", c.pdf.workDir, err.Error())
		c.respondString(http.StatusInternalServerError, "ERROR: failed to initialize PDF process")
		return
	}

	// fudge some numbers for a 0% progress
	c.updateProgress(0, -1)

	// kick the lengthy PDF generation off in a go routine
	go c.generatePdf()

	// Render a simple ok message or kick an ajax polling loop
	c.inProgress()
}

/*
 * Render a simple html page that will poll for status of this PDF, and download it when done
 */
func (c *clientContext) renderAjaxPage() (string, error) {
	varmap := map[string]interface{}{
		"pid":   c.req.pid,
		"token": c.pdf.workSubDir,
	}
	index := fmt.Sprintf("%s/index.html", config.templateDir.value)
	tmpl, _ := template.ParseFiles(index)
	var b bytes.Buffer
	err := tmpl.ExecuteTemplate(&b, "index.html", varmap)
	if err != nil {
		return "", fmt.Errorf("unable to render ajax polling page for %s: %s", c.req.pid, err.Error())
	}
	return b.String(), nil
}

func (c *clientContext) openURL(url string) (io.ReadCloser, error) {
	maxTries := 5
	backoff := 1

	for i := 1; i <= maxTries; i++ {
		h, err := http.Get(url)

		if err != nil {
			return nil, err
		}

		if h.StatusCode == http.StatusOK {
			return h.Body, nil
		}

		if h.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("received http status: %s", h.Status)
		}

		if i == maxTries {
			c.err("open [%s] (try %d/%d): received status: %s; giving up", url, i, maxTries, h.Status)
			return nil, fmt.Errorf("max tries reached")
		}

		c.warn("open [%s] (try %d/%d): received status: %s; will try again in %d seconds...", url, i, maxTries, h.Status, backoff)

		time.Sleep(time.Duration(backoff) * time.Second)
		backoff *= 2
	}

	return nil, fmt.Errorf("max tries reached")
}

func (c *clientContext) downloadJpgFromIiif(pid string) (jpgFileName string, err error) {
	url := config.iiifURLTemplate.value
	url = strings.Replace(url, "{PID}", pid, -1)

	pfx := fmt.Sprintf("[%s] ", pid)

	c.info(pfx+"downloading: %s", url)
	body, err := c.openURL(url)
	if err != nil {
		c.err(pfx+"download failed: %s", err.Error())
		return
	}
	defer body.Close()

	jpgFileName = fmt.Sprintf("%s/%s.jpg", c.pdf.workDir, pid)
	destFile, err := os.Create(jpgFileName)
	if err != nil {
		c.err(pfx+"download failed: %s", err.Error())
		return
	}
	defer destFile.Close()

	s, err := io.Copy(destFile, body)
	if err != nil {
		c.err(pfx+"download failed: %s", err.Error())
		return
	}

	c.info(pfx+"download succeeded: %d bytes", s)
	return
}

func (c *clientContext) updateProgress(step int, steps int) {
	if steps > 0 {
		c.info("%d%% (step %d of %d)", (100*step)/steps, step, steps)
	}

	f, _ := os.OpenFile(fmt.Sprintf("%s/progress.txt", c.pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
	defer f.Close()

	w := bufio.NewWriter(f)

	if _, err := fmt.Fprintf(w, "%d%%", (100*step)/steps); err != nil {
		c.err("unable to write progress file: %s", err.Error())
	}

	w.Flush()
}

func (c *clientContext) getCoverPageArgs() []string {
	args := []string{}

	if c.pdf.solr == nil {
		return args
	}

	// set up arguments

	header := `This resource was made available courtesy of the UVA Library.\n\nNOTICE: This material may be protected by copyright law (Title 17, United States Code)`

	logo := fmt.Sprintf("%s/UVALIB_primary_black_print.png", config.assetsDir.value)

	doc := c.pdf.solr.Response.Docs[0]

	// use first entry for these fields, if available
	title := firstElementOf(doc.Title)
	author := firstElementOf(doc.AuthorFacet)
	year := firstElementOf(doc.PublishedDaterange)
	rightswrapper := firstElementOf(doc.RightsWrapper)

	// filter out catalog link, convert http: to https:, remove period from terms link, and drop any trailing newline
	rights := ""
	for _, line := range strings.Split(rightswrapper, "\n") {
		if strings.Contains(line, "/catalog/") {
			continue
		}

		rights = fmt.Sprintf("%s%s\n", rights, line)
	}
	rights = strings.Replace(rights, "http:", "https:", -1)
	rights = strings.Replace(rights, ".html.", ".html", -1)
	rights = strings.TrimRight(rights, "\n")

	generated := fmt.Sprintf("Generation date: %s", time.Now().Format("2006-01-02"))

	url := strings.Replace(config.virgoURLTemplate.value, "{ID}", doc.ID, -1)

	citation := ""
	if author != "" {
		citation = fmt.Sprintf("%s%s. ", citation, strings.TrimRight(author, "."))
	}
	if year != "" {
		citation = fmt.Sprintf("%s(%s). ", citation, year)
	}
	citation = fmt.Sprintf("%s\"%s\" [PDF document]. Available from %s", citation, title, url)

	// FIXME: rights may not always be present, should we get it elsewhere?
	libraryid := fmt.Sprintf("UVA Library ID Information:\n\n%s", rights)

	footer := fmt.Sprintf("%s\n\n\n%s\n\n\n\n%s", generated, citation, libraryid)

	c.debug("title  : [%s]", title)
	c.debug("author : [%s]", author)
	c.debug("year   : [%s]", year)
	c.debug("verify : [%s] (%s)", c.pdf.workDir, url)

	args = []string{"-c", "-h", header, "-l", logo, "-t", title, "-a", author, "-f", footer}

	return args
}

/**
 * use jp2 or archived tif files to generate a multipage PDF for a PID
 */
func (c *clientContext) generatePdf() {
	// initialize progress reporting:
	// steps include each page download, plus a final conversion step
	// future enhancement: each page download, plus each page as processed by imagemagick (convert -monitor)

	var steps = len(c.pdf.ts.Pages) + 1
	var step = 0
	c.updateProgress(step, steps)

	start := time.Now()

	// iterate over page info and build a list of paths to
	// the image for that page. Older pages may only be stored on an NFS share
	// and newer pages will have a jp2k file available on the iiif server
	var jpgFiles []string
	for _, page := range c.pdf.ts.Pages {
		// if working dir has been removed from under us, abort
		if _, err := os.Stat(c.pdf.workDir); err != nil {
			c.err("working directory [%s] vanished; aborting", c.pdf.workDir)
			return
		}

		// get image from iiif
		jpgFile, jpgErr := c.downloadJpgFromIiif(page.Pid)
		if jpgErr != nil {
			c.warn("no image for %s found on IIIF server; continuing", page.Pid)
			continue
		}
		jpgFiles = append(jpgFiles, jpgFile)

		step++
		c.updateProgress(step, steps)
	}

	// check if we have any jpg files to process

	if len(jpgFiles) == 0 {
		c.err("no jpg files to process")
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", c.pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer ef.Close()
		if _, err := ef.WriteString("No jpg files to process"); err != nil {
			c.err("unable to write error file: %s", err.Error())
		}
		return
	}

	// Now merge all of the files into 1 pdf
	pdfFile := fmt.Sprintf("%s/%s.pdf", c.pdf.workDir, c.req.pid)
	c.info("merging images into single PDF: %s", pdfFile)

	// generate a cover page only if we have solr info
	coverPageArgs := c.getCoverPageArgs()

	// finally build helper script command and argument string
	cmd := fmt.Sprintf("%s/mkpdf.sh", config.scriptDir.value)
	args := []string{"-o", pdfFile, "-n", config.pdfChunkSize.value}
	args = append(args, coverPageArgs...)
	args = append(args, "--")
	args = append(args, jpgFiles...)

	out, convErr := exec.Command(cmd, args...).CombinedOutput()

	cf, _ := os.OpenFile(fmt.Sprintf("%s/convert.txt", c.pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
	defer cf.Close()
	if _, err := cf.WriteString(string(out)); err != nil {
		c.err("unable to write conversion log file: %s", err.Error())
	}

	if convErr != nil {
		c.err("unable to generate merged PDF: %s", convErr.Error())
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", c.pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer ef.Close()
		if _, err := ef.WriteString(convErr.Error()); err != nil {
			c.err("unable to write error file: %s", err.Error())
		}
	} else {
		c.info("generated PDF: %s", pdfFile)
		df, _ := os.OpenFile(fmt.Sprintf("%s/done.txt", c.pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer df.Close()
		if _, err := df.WriteString(pdfFile); err != nil {
			c.err("unable to write done file: %s", err.Error())
		}
	}

	step = steps
	c.updateProgress(step, steps)

	elapsed := time.Since(start).Seconds()

	c.info("DONE: %d pages processed in %0.2f seconds (%0.2f seconds/page)",
		len(jpgFiles), elapsed, elapsed/float64(len(jpgFiles)))
}

func (c *clientContext) isDone() bool {
	if _, err := os.Stat(fmt.Sprintf("%s/done.txt", c.pdf.workDir)); err == nil {
		return true
	}
	return false
}

func (c *clientContext) isFailed() bool {
	if _, err := os.Stat(fmt.Sprintf("%s/fail.txt", c.pdf.workDir)); err == nil {
		return true
	}
	return false
}

func (c *clientContext) isInProgress() bool {
	if _, err := os.Stat(fmt.Sprintf("%s/progress.txt", c.pdf.workDir)); err == nil {
		return true
	}
	return false
}

func (c *clientContext) progressInValidState() bool {
	// valid states being: { in progress, done, failed }

	// returns true if the specified directory exists, and contains
	// at least one of the known progress/completion files.

	// this is a helper to work around a race condition in which the
	// directory exists but is empty, and no pdf is being generated.

	if _, err := os.Stat(c.pdf.workDir); err != nil {
		return false
	}

	if ok := c.isDone(); ok == true {
		return true
	}

	if ok := c.isFailed(); ok == true {
		return true
	}

	if ok := c.isInProgress(); ok == true {
		// at this point, there is the potential for an in-progress generation to
		// have crashed without creating a done.txt or fail.txt file.  we ignore
		// this possibility for now, and just assume the process is chugging along.
		return true
	}

	// the directory might contain other files such as image/pdf data,
	// but no need to keep it if we don't know what state it's in.
	// just remove it.
	if err := c.removeWorkDir(3, 5); err != nil {
		c.info("failed to remove work dir [%s]: %s", c.pdf.workDir, err.Error())
	}

	return false
}

func statusHandler(ctx *gin.Context) {
	c := newClientContext(ctx)

	if c.progressInValidState() == false {
		c.respondString(http.StatusNotFound, "Not found")
		return
	}

	doneFile := fmt.Sprintf("%s/done.txt", c.pdf.workDir)
	if _, err := os.Stat(doneFile); err == nil {
		c.respondString(http.StatusOK, "READY")
		return
	}

	errorFile := fmt.Sprintf("%s/fail.txt", c.pdf.workDir)
	if _, err := os.Stat(errorFile); err == nil {
		c.respondString(http.StatusOK, "FAILED")
		return
	}

	progressFile := fmt.Sprintf("%s/progress.txt", c.pdf.workDir)
	prog, err := ioutil.ReadFile(progressFile)
	if err != nil {
		c.respondString(http.StatusOK, "PROCESSING")
		return
	}

	c.respondString(http.StatusOK, string(prog))
}

func downloadHandler(ctx *gin.Context) {
	c := newClientContext(ctx)

	if c.progressInValidState() == false {
		c.respondString(http.StatusNotFound, "Not found")
		return
	}

	/* get path of file to send from the done file */
	f, err := os.OpenFile(fmt.Sprintf("%s/done.txt", c.pdf.workDir), os.O_RDONLY, 0666)
	if err != nil {
		c.respondString(http.StatusNotFound, "Not found")
		return
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		c.respondString(http.StatusInternalServerError, "Unable to find PDF for this PID")
		return
	}

	pdfFile := strings.TrimSpace(string(b))

	/* seamless conversion from old locations */
	if strings.HasPrefix(pdfFile, "tmp/") {
		pdfFile = strings.Replace(pdfFile, "tmp", config.storageDir.value, 1)
	}

	/* get file size */
	in, err := os.Open(pdfFile)
	if err != nil {
		c.err("failed to open [%s]: %s", pdfFile, err.Error())
		c.respondString(http.StatusInternalServerError, "Unable to open PDF for this PID")
		return
	}
	defer in.Close()

	stat, staterr := in.Stat()
	if staterr != nil {
		c.err("failed to stat [%s]: %s", pdfFile, staterr.Error())
		c.respondString(http.StatusInternalServerError, "Unable to stat PDF for this PID")
		return
	}

	contentLength := stat.Size()
	contentType := "application/pdf"
	fileName := fmt.Sprintf("%s.pdf", c.req.pid)

	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
	}

	c.info("PDF download started: %s (%d bytes)", pdfFile, contentLength)
	c.respondDataFromReader(http.StatusOK, contentLength, contentType, in, extraHeaders)
}

func deleteHandler(ctx *gin.Context) {
	c := newClientContext(ctx)

	// ten attempts over a max of 825 seconds (13.75 minutes) should about do it
	go c.removeWorkDir(10, 15)

	c.respondString(http.StatusOK, "DELETED")
}

func (c *clientContext) removeWorkDir(maxAttempts int, waitBetween int) error {
	// tries to remove the work directory, with arithmetic backoff retry logic.
	// total time before giving up in worst case is:
	// seconds = waitBetween * (maxAttempts * (maxAttempts + 1) / 2)

	// this attempts to work around intermittent NFS "resource busy" errors,
	// increasing the likelihood that the directory is eventually removed.

	wait := 0

	for i := 0; i < maxAttempts; i++ {
		time.Sleep(time.Duration(wait) * time.Second)

		if err := os.RemoveAll(c.pdf.workDir); err != nil {
			c.warn("delete attempt %d/%d for [%s] failed; err: %s", i+1, maxAttempts, c.pdf.workDir, err.Error())
		} else {
			// we are done
			c.info("delete attempt %d/%d for [%s] succeeded", i+1, maxAttempts, c.pdf.workDir)
			return nil
		}

		wait += waitBetween
	}

	c.err("delete FAILED for [%s]: max attempts reached", c.pdf.workDir)
	return errors.New("max attempts reached")
}
