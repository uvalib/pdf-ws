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
	"strconv"
	"strings"
	"time"

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
	req        pdfRequest // values from original request
	ts         *tsPidInfo // values looked up in tracksys
	solr       *solrInfo  // values looked up in solr
	subDir     string
	workSubDir string
	workDir    string
	embed      bool
}

func getWorkSubDir(pid, unit, token string) string {
	subDir := pid

	switch {
	case token != "":
		subDir = token

	case unit != "":
		unitID, _ := strconv.Atoi(unit)
		if unitID > 0 {
			subDir = fmt.Sprintf("%s/%d", pid, unitID)
		}
	}

	return subDir
}

func getWorkDir(subDir string) string {
	return fmt.Sprintf("%s/%s", config.storageDir.value, subDir)
}

/**
 * Handle a request for a PDF of page images
 */
func generateHandler(c *gin.Context) {
	pdf := pdfInfo{}

	pdf.req.pid = c.Param("pid")
	pdf.req.unit = c.Query("unit")
	pdf.req.pages = c.Query("pages")
	pdf.req.token = c.Query("token")
	pdf.req.embed = c.Query("embed")

	pdf.subDir = pdf.req.pid

	token := ""
	if pdf.req.pages != "" {
		if pdf.req.token == "" {
			logger.Printf("Request for partial PDF is missing a token")
			c.String(http.StatusBadRequest, "Missing token")
			return
		}
		token = pdf.req.token
		logger.Printf("Request for partial PDF including pages: %s", pdf.req.pages)
	}

	pdf.workSubDir = getWorkSubDir(pdf.subDir, pdf.req.unit, token)
	pdf.workDir = getWorkDir(pdf.workSubDir)

	pdf.embed = true
	if len(pdf.req.embed) == 0 || pdf.req.embed == "0" {
		pdf.embed = false
	}

	// See if destination already extsts...
	if progressInValidState(pdf.workDir) == true {
		// path already exists; don't start another request, just treat
		// this one is if it was successful and render the ajax page
		if pdf.embed {
			c.String(http.StatusOK, "ok")
		} else {
			if ajax, err := renderAjaxPage(pdf.workSubDir, pdf.req.pid); err != nil {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(ajax))
			}
		}
		return
	}

	var apiErr error

	pdf.ts, apiErr = tsGetPidInfo(pdf.req.pid, pdf.req.unit, pdf.req.pages)

	if apiErr != nil {
		logger.Printf("Tracksys API error: [%s]", apiErr.Error())
		c.String(http.StatusNotFound, fmt.Sprintf("ERROR: Could not retrieve PID info: [%s]", apiErr.Error()))
		return
	}

	pdf.solr, apiErr = solrGetInfo(pdf.req.pid)

	if apiErr != nil {
		logger.Printf("WARNING: [%s] Solr error: [%s]", pdf.req.pid, apiErr.Error())

		if pdf.workDir == pdf.req.pid {
			logger.Printf("WARNING: [%s] generating PDF without a cover page", pdf.req.pid)
		} else {
			logger.Printf("WARNING: [%s] generating PDF without a cover page in directory: %s", pdf.req.pid, pdf.workDir)
		}
	}

	// kick the lengthy PDF generation off in a go routine
	go generatePdf(pdf)

	// Render a simple ok message or kick an ajax polling loop
	if pdf.embed {
		c.String(http.StatusOK, "ok")
	} else {
		if ajax, err := renderAjaxPage(pdf.workSubDir, pdf.req.pid); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(ajax))
		}
	}
}

/*
 * Render a simple html page that will poll for status of this PDF, and download it when done
 */
func renderAjaxPage(workSubDir string, pid string) (string, error) {
	varmap := map[string]interface{}{
		"pid":   pid,
		"token": workSubDir,
	}
	index := fmt.Sprintf("%s/index.html", config.templateDir.value)
	tmpl, _ := template.ParseFiles(index)
	var b bytes.Buffer
	err := tmpl.ExecuteTemplate(&b, "index.html", varmap)
	if err != nil {
		return "", fmt.Errorf("Unable to render ajax polling page for %s: %s", pid, err.Error())
	}
	return b.String(), nil
}

func downloadJpgFromIiif(outPath string, pid string) (jpgFileName string, err error) {
	url := config.iiifUrlTemplate.value
	url = strings.Replace(url, "{PID}", pid, -1)

	logger.Printf("Downloading JPG from: %s", url)
	response, err := http.Get(url)
	if err != nil || response.StatusCode != 200 {
		err = errors.New("not found")
		return
	}
	defer response.Body.Close()

	jpgFileName = fmt.Sprintf("%s/%s.jpg", outPath, pid)
	destFile, err := os.Create(jpgFileName)
	if err != nil {
		return
	}
	defer destFile.Close()

	s, err := io.Copy(destFile, response.Body)
	if err != nil {
		return
	}

	logger.Printf(fmt.Sprintf("Successful download size: %d", s))
	return
}

func jpgFromTif(outPath string, pid string, tifFile string) (jpgFileName string, err error) {
	jpgFileName = fmt.Sprintf("%s/%s.jpg", outPath, pid)
	bits := strings.Split(tifFile, "_")
	srcFile := fmt.Sprintf("%s/%s/%s", config.archiveDir.value, bits[0], tifFile)
	logger.Printf("Using archived file as source: %s", srcFile)
	_, err = os.Stat(srcFile)
	if err != nil {
		logger.Printf("ERROR %s", err.Error())
		return
	}

	// run imagemagick to create scaled down jpg
	cmd := "convert"
	args := []string{fmt.Sprintf("%s[0]", srcFile), "-resize", "1024", jpgFileName}
	convErr := exec.Command(cmd, args...).Run()
	if convErr != nil {
		logger.Printf("Unable to generate JPG for %s", tifFile)
	} else {
		logger.Printf("Generated %s", jpgFileName)
	}
	return
}

func updateProgress(outPath string, step int, steps int) {
	logger.Printf("%d%% (step %d of %d)", (100*step)/steps, step, steps)

	f, _ := os.OpenFile(fmt.Sprintf("%s/progress.txt", outPath), os.O_CREATE|os.O_RDWR, 0666)
	defer f.Close()

	w := bufio.NewWriter(f)

	if _, err := fmt.Fprintf(w, "%d%%", (100*step)/steps); err != nil {
		logger.Printf("Unable to write progress file : %s", err.Error())
	}

	w.Flush()
}

func getCoverPageArgs(pdf pdfInfo) []string {
	args := []string{}

	if pdf.solr == nil {
		return args
	}

	// set up arguments

	header := `This resource was made available courtesy of the UVA Library.\n\nNOTICE: This material may be protected by copyright law (Title 17, United States Code)`

	logo := fmt.Sprintf("%s/UVALIB_primary_black_web.png", config.assetsDir.value)

	doc := pdf.solr.Response.Docs[0]

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

	url := strings.Replace(config.virgoUrlTemplate.value, "{ID}", doc.Id, -1)

	citation := ""
	if author != "" {
		citation = fmt.Sprintf("%s%s. ", citation, strings.TrimRight(author, "."))
	}
	if year != "" {
		citation = fmt.Sprintf("%s(%s). ", citation, year)
	}
	citation = fmt.Sprintf("%s\"%s\" [PDF document]. Available from %s", citation, title, url)

	libraryid := fmt.Sprintf("UVA Library ID Information:\n\n%s", rights)

	footer := fmt.Sprintf("%s\n\n\n%s\n\n\n\n%s", generated, citation, libraryid)

	logger.Printf("title  : [%s]", title)
	logger.Printf("author : [%s]", author)
	logger.Printf("year   : [%s]", year)
	logger.Printf("verify : [%s] (%s)", pdf.workDir, url)

	args = []string{"-c", "-h", header, "-l", logo, "-t", title, "-a", author, "-f", footer}

	return args
}

/**
 * use jp2 or archived tif files to generate a multipage PDF for a PID
 */
func generatePdf(pdf pdfInfo) {
	// Make sure the work directory exists
	os.MkdirAll(pdf.workDir, 0777)

	// initialize progress reporting:
	// steps include each page download, plus a final conversion step
	// future enhancement: each page download, plus each page as processed by imagemagick (convert -monitor)

	var steps = len(pdf.ts.Pages) + 1
	var step = 0

	start := time.Now()

	// iterate over page info and build a list of paths to
	// the image for that page. Older pages may only be stored on an NFS share
	// and newer pages will have a jp2k file available on the iiif server
	var jpgFiles []string
	for _, page := range pdf.ts.Pages {
		// if working dir has been removed from under us, abort
		if _, err := os.Stat(pdf.workDir); err != nil {
			logger.Printf("working directory [%s] vanished; aborting", pdf.workDir)
			return
		}

		logger.Printf("Get reduced size jpg for %s %s", page.Pid, page.Filename)

		step++
		updateProgress(pdf.workDir, step, steps)

		// First, try to get a JPG file from the IIIF server
		jpgFile, jpgErr := downloadJpgFromIiif(pdf.workDir, page.Pid)
		if jpgErr != nil {
			// not found. work from the archival tif file
			logger.Printf("No JPG found on IIIF server; using archival tif...")
			jpgFile, jpgErr = jpgFromTif(pdf.workDir, page.Pid, page.Filename)
			if jpgErr != nil {
				logger.Printf("Unable to find source image for masterFile %s. Skipping.", page.Pid)
				continue
			}
		}
		jpgFiles = append(jpgFiles, jpgFile)
	}

	// check if we have any jpg files to process

	if len(jpgFiles) == 0 {
		logger.Printf("No jpg files to process")
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer ef.Close()
		if _, err := ef.WriteString("No jpg files to process"); err != nil {
			logger.Printf("Unable to write error file : %s", err.Error())
		}
		return
	}

	// Now merge all of the files into 1 pdf
	pdfFile := fmt.Sprintf("%s/%s.pdf", pdf.workDir, pdf.req.pid)
	logger.Printf("Merging pages into single PDF %s", pdfFile)

	// generate a cover page only if we have Solr info

	coverPageArgs := getCoverPageArgs(pdf)

	// finally build helper script command and argument string
	cmd := fmt.Sprintf("%s/mkpdf.sh", config.scriptDir.value)
	args := []string{"-o", pdfFile, "-n", "50"}
	args = append(args, coverPageArgs...)
	args = append(args, "--")
	args = append(args, jpgFiles...)

	out, convErr := exec.Command(cmd, args...).CombinedOutput()

	cf, _ := os.OpenFile(fmt.Sprintf("%s/convert.txt", pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
	defer cf.Close()
	if _, err := cf.WriteString(string(out)); err != nil {
		logger.Printf("Unable to write conversion log file : %s", err.Error())
	}

	if convErr != nil {
		logger.Printf("Unable to generate merged PDF : %s", convErr.Error())
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer ef.Close()
		if _, err := ef.WriteString(convErr.Error()); err != nil {
			logger.Printf("Unable to write error file : %s", err.Error())
		}
	} else {
		logger.Printf("Generated PDF : %s", pdfFile)
		df, _ := os.OpenFile(fmt.Sprintf("%s/done.txt", pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer df.Close()
		if _, err := df.WriteString(pdfFile); err != nil {
			logger.Printf("Unable to write done file : %s", err.Error())
		}
	}

	step = steps
	updateProgress(pdf.workDir, step, steps)

	elapsed := time.Since(start).Seconds()

	logger.Printf("%d pages processed in %0.2f seconds (%0.2f seconds/page)",
		len(jpgFiles), elapsed, elapsed/float64(len(jpgFiles)))

	// Cleanup intermediate jpgFiles
	exec.Command("rm", jpgFiles...).Run()
}

func progressInValidState(dir string) bool {
	// valid states being: { in progress, done, failed }

	// returns true if the specified directory exists, and contains
	// at least one of the known progress/completion files.

	// this is a helper to work around a race condition in which the
	// directory exists but is empty, and no pdf is being generated.

	if _, err := os.Stat(dir); err != nil {
		return false
	}

	if _, err := os.Stat(fmt.Sprintf("%s/done.txt", dir)); err == nil {
		return true
	}

	if _, err := os.Stat(fmt.Sprintf("%s/fail.txt", dir)); err == nil {
		return true
	}

	if _, err := os.Stat(fmt.Sprintf("%s/progress.txt", dir)); err == nil {
		// at this point, there is the potential for an in-progress generation to
		// have crashed without creating a done.txt or fail.txt file.  we ignore
		// this possibility for now, and just assume the process is chugging along.
		return true
	}

	// the directory might contain other files such as image/pdf data,
	// but no need to keep it if we don't know what state it's in.
	// just remove it.

	if err := os.RemoveAll(dir); err != nil {
		logger.Printf("progressInValidState(): RemoveAll() failed for [%s]: %s", dir, err.Error())
	}

	return false
}

func statusHandler(c *gin.Context) {
	pdf := pdfInfo{}

	pdf.req.pid = c.Param("pid")
	pdf.req.unit = c.Query("unit")
	pdf.req.token = c.Query("token")

	pdf.subDir = pdf.req.pid

	pdf.workSubDir = getWorkSubDir(pdf.subDir, pdf.req.unit, pdf.req.token)
	pdf.workDir = getWorkDir(pdf.workSubDir)

	if progressInValidState(pdf.workDir) == false {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	doneFile := fmt.Sprintf("%s/done.txt", pdf.workDir)
	if _, err := os.Stat(doneFile); err == nil {
		c.String(http.StatusOK, "READY")
		return
	}

	errorFile := fmt.Sprintf("%s/fail.txt", pdf.workDir)
	if _, err := os.Stat(errorFile); err == nil {
		c.String(http.StatusOK, "FAILED")
		return
	}

	progressFile := fmt.Sprintf("%s/progress.txt", pdf.workDir)
	prog, err := ioutil.ReadFile(progressFile)
	if err != nil {
		c.String(http.StatusOK, "PROCESSING")
		return
	}

	c.String(http.StatusOK, string(prog))
}

func downloadHandler(c *gin.Context) {
	pdf := pdfInfo{}

	pdf.req.pid = c.Param("pid")
	pdf.req.unit = c.Query("unit")
	pdf.req.token = c.Query("token")

	pdf.subDir = pdf.req.pid

	pdf.workSubDir = getWorkSubDir(pdf.subDir, pdf.req.unit, pdf.req.token)
	pdf.workDir = getWorkDir(pdf.workSubDir)

	if progressInValidState(pdf.workDir) == false {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	/* get path of file to send from the done file */
	f, err := os.OpenFile(fmt.Sprintf("%s/done.txt", pdf.workDir), os.O_RDONLY, 0666)
	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		c.String(http.StatusInternalServerError, "Unable to find PDF for this PID")
		return
	}

	pdfFile := strings.TrimSpace(string(b))

	/* seamless conversion from old locations */
	if strings.HasPrefix(pdfFile, "tmp/") {
		logger.Printf("Old location: [%s]", pdfFile)
		pdfFile = strings.Replace(pdfFile, "tmp", config.storageDir.value, 1)
		logger.Printf("New location: [%s]", pdfFile)

		/* write out new location */
	}

	/* get file size */

	in, err := os.Open(pdfFile)
	if err != nil {
		c.String(http.StatusInternalServerError, "Unable to open PDF for this PID")
		return
	}
	defer in.Close()

	stat, staterr := in.Stat()
	if staterr != nil {
		c.String(http.StatusInternalServerError, "Unable to stat PDF for this PID")
		return
	}

	logger.Printf("Sending %s to client with size %d", pdfFile, stat.Size())

	contentLength := stat.Size()
	contentType := "application/pdf"
	fileName := fmt.Sprintf("%s.pdf", pdf.req.pid)

	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, fileName),
	}

	c.DataFromReader(http.StatusOK, contentLength, contentType, in, extraHeaders)

	logger.Printf("PDF download for %s completed successfully", pdf.req.pid)
}

func deleteHandler(c *gin.Context) {
	pdf := pdfInfo{}

	pdf.req.pid = c.Param("pid")
	pdf.req.unit = c.Query("unit")
	pdf.req.token = c.Query("token")

	pdf.subDir = pdf.req.pid

	pdf.workSubDir = getWorkSubDir(pdf.subDir, pdf.req.unit, pdf.req.token)
	pdf.workDir = getWorkDir(pdf.workSubDir)

	// ten attempts over a max of 825 seconds (13.75 minutes) should about do it
	go removeDirectory(pdf.workDir, 10, 15)

	c.String(http.StatusOK, "DELETED")
}

func removeDirectory(dir string, maxAttempts int, waitBetween int) {
	// tries to remove the given directory, with arithmetic backoff retry logic.
	// total time before giving up in worst case is:
	// seconds = waitBetween * (maxAttempts * (maxAttempts + 1) / 2)

	// this attempts to work around intermittent NFS "resource busy" errors,
	// increasing the likelihood that the directory is eventually removed.

	wait := 0

	for i := 0; i < maxAttempts; i++ {
		time.Sleep(time.Duration(wait) * time.Second)

		if err := os.RemoveAll(dir); err != nil {
			logger.Printf("delete attempt %d/%d for [%s] failed; err: %s", i+1, maxAttempts, dir, err.Error())
		} else {
			// we are done
			logger.Printf("delete attempt %d/%d for [%s] succeeded", i+1, maxAttempts, dir)
			return
		}

		wait += waitBetween
	}

	logger.Printf("delete FAILED for [%s]: max attempts reached", dir)
}
