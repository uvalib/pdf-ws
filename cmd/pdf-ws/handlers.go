package main

import (
	"bufio"
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

	"github.com/julienschmidt/httprouter"
)

type pdfRequest struct {
	pid   string
	unit  string
	pages string
	token string
	embed string
}

type pdfInfo struct {
	req     pdfRequest // values from original request
	ts      *tsPidInfo // values looked up in tracksys
	solr    *solrInfo  // values looked up in solr
	subDir  string
	workDir string
	embed   bool
}

func getWorkDir(pid, unit, token string) string {
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

	return fmt.Sprintf("%s/%s", config.storageDir.value, subDir)
}

/**
 * Handle a request for a PDF of page images
 */
func generateHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)

	pdf := pdfInfo{}

	pdf.req.pid = params.ByName("pid")
	pdf.req.unit = r.URL.Query().Get("unit")
	pdf.req.pages = r.URL.Query().Get("pages")
	pdf.req.token = r.URL.Query().Get("token")
	pdf.req.embed = r.URL.Query().Get("embed")

	pdf.subDir = pdf.req.pid

	token := ""
	if pdf.req.pages != "" {
		if pdf.req.token != "" {
			logger.Printf("Request for partial PDF is missing a token")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Missing token")
			return
		}
		token = pdf.req.token
		logger.Printf("Request for partial PDF including pages: %s", pdf.req.pages)
	}

	pdf.workDir = getWorkDir(pdf.subDir, pdf.req.unit, token)

	pdf.embed = true
	if len(pdf.req.embed) == 0 || pdf.req.embed == "0" {
		pdf.embed = false
	}

	// See if destination already extsts...
	if _, err := os.Stat(pdf.workDir); err == nil {
		// path already exists; don't start another request, just treat
		// this one is if it was successful and render the ajax page
		if pdf.embed {
			fmt.Fprintf(w, "ok")
		} else {
			renderAjaxPage(pdf.workDir, pdf.req.pid, w)
		}
		return
	}

	var apiErr error

	pdf.ts, apiErr = tsGetPidInfo(pdf.req.pid, pdf.req.unit)

	if apiErr != nil {
		logger.Printf("Tracksys API error: [%s]", apiErr.Error())
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, fmt.Sprintf("ERROR: Could not retrieve PID info: [%s]", apiErr.Error()))
		return
	}

	pdf.solr, apiErr = solrGetInfo(pdf.req.pid)

	if apiErr != nil {
		logger.Printf("Solr error: [%s]", apiErr.Error())
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, fmt.Sprintf("ERROR: Could not retrieve Solr info: [%s]", apiErr.Error()))
		return
	}

	// kick the lengthy PDF generation off in a go routine
	go generatePdf(pdf)

	// Render a simple ok message or kick an ajax polling loop
	if pdf.embed {
		fmt.Fprintf(w, "ok")
	} else {
		renderAjaxPage(pdf.workDir, pdf.req.pid, w)
	}
}

/*
 * Render a simple html page that will poll for status of this PDF, and download it when done
 */
func renderAjaxPage(workDir string, pid string, w http.ResponseWriter) {
	varmap := map[string]interface{}{
		"pid":   pid,
		"token": workDir,
	}
	index := fmt.Sprintf("%s/index.html", config.templateDir.value)
	tmpl, _ := template.ParseFiles(index)
	err := tmpl.ExecuteTemplate(w, "index.html", varmap)
	if err != nil {
		logger.Printf("Unable to render ajax polling page for %s: %s", pid, err.Error())
		fmt.Fprintf(w, "Unable to render ajax polling page for %s: %s", pid, err.Error())
	}
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
	// the image for that page. Older pages may only be stored on lib_content44
	// and newer pages will have a jp2k file avialble on the iiif server
	var jpgFiles []string
	for _, page := range pdf.ts.Pages {
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

	// set up arguments

	header := `This book was made available courtesy of the UVA Library.\n\nNOTICE: This material may be protected by copyright law (Title 17, United States Code)`

	logo := fmt.Sprintf("%s/UVALIB_inline_black_web.png", config.assetsDir.value)

	// use first title entry if available
	title := ""
	if len(pdf.solr.Response.Docs[0].TitleDisplay) > 0 {
		title = pdf.solr.Response.Docs[0].TitleDisplay[0]
	}

	// use first author entry if available
	author := ""
	if len(pdf.solr.Response.Docs[0].AuthorFacet) > 0 {
		author = pdf.solr.Response.Docs[0].AuthorFacet[0]
	}

	// use first published date entry if available
	year := ""
	if len(pdf.solr.Response.Docs[0].PublishedDateDisplay) > 0 {
		year = pdf.solr.Response.Docs[0].PublishedDateDisplay[0]
	}

	// use first rights wrapper text entry if available
	rightswrapper := ""
	if len(pdf.solr.Response.Docs[0].RightsWrapperDisplay) > 0 {
		rightswrapper = pdf.solr.Response.Docs[0].RightsWrapperDisplay[0]
	}

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

	url := strings.Replace(config.virgoUrlTemplate.value, "{ID}", pdf.solr.Response.Docs[0].Id, -1)

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

	logger.Printf("header        : [%s]", header)
	logger.Printf("logo          : [%s]", logo)
	logger.Printf("title         : [%s]", title)
	logger.Printf("author        : [%s]", author)
	logger.Printf("year          : [%s]", year)
	logger.Printf("rightswrapper : [%s]", rightswrapper)
	logger.Printf("rights        : [%s]", rights)
	logger.Printf("generated     : [%s]", generated)
	logger.Printf("citation      : [%s]", citation)
	logger.Printf("libraryid     : [%s]", libraryid)
	logger.Printf("footer        : [%s]", footer)

	// finally build helper script command and argument string
	cmd := fmt.Sprintf("%s/mkpdf.sh", config.scriptDir.value)
	args := []string{"-o", pdfFile, "-n", "50", "-h", header, "-l", logo, "-t", title, "-a", author, "-f", footer, "--"}
	args = append(args, jpgFiles...)

	convErr := exec.Command(cmd, args...).Run()

	if convErr != nil {
		logger.Printf("Unable to generate merged PDF : %s", convErr.Error())
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer ef.Close()
		if _, err := ef.WriteString(convErr.Error()); err != nil {
			logger.Printf("Unable to write error file : %s", err.Error())
		}
	} else {
		logger.Printf("Generated PDF : %s", pdfFile)
		ef, _ := os.OpenFile(fmt.Sprintf("%s/done.txt", pdf.workDir), os.O_CREATE|os.O_RDWR, 0666)
		defer ef.Close()
		if _, err := ef.WriteString(pdfFile); err != nil {
			logger.Printf("Unable to write done file : %s", err.Error())
		}
	}

	step++
	updateProgress(pdf.workDir, step, steps)

	elapsed := time.Since(start).Seconds()

	logger.Printf("%d pages processed in %0.2f seconds (%0.2f seconds/page)",
		len(jpgFiles), elapsed, elapsed/float64(len(jpgFiles)))

	// Cleanup intermediate jpgFiles
	exec.Command("rm", jpgFiles...).Run()
}

func statusHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)

	pdf := pdfInfo{}

	pdf.req.pid = params.ByName("pid")
	pdf.req.unit = r.URL.Query().Get("unit")
	pdf.req.token = r.URL.Query().Get("token")

	pdf.subDir = pdf.req.pid
	pdf.workDir = getWorkDir(pdf.subDir, pdf.req.unit, pdf.req.token)

	if _, err := os.Stat(pdf.workDir); err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}

	if _, err := os.Stat(fmt.Sprintf("%s/done.txt", pdf.workDir)); err == nil {
		fmt.Fprintf(w, "READY")
		return
	}

	errorFile := fmt.Sprintf("%s/fail.txt", pdf.workDir)
	if _, err := os.Stat(errorFile); err == nil {
		fmt.Fprintf(w, "FAILED")
		os.RemoveAll(pdf.workDir)
		return
	}

	progressFile := fmt.Sprintf("%s/progress.txt", pdf.workDir)
	prog, err := ioutil.ReadFile(progressFile)
	if err != nil {
		fmt.Fprintf(w, "PROCESSING")
		return
	}

	fmt.Fprintf(w, "%s", string(prog))
}

func downloadHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)

	pdf := pdfInfo{}

	pdf.req.pid = params.ByName("pid")
	pdf.req.unit = r.URL.Query().Get("unit")
	pdf.req.token = r.URL.Query().Get("token")

	pdf.subDir = pdf.req.pid
	pdf.workDir = getWorkDir(pdf.subDir, pdf.req.unit, pdf.req.token)

	if _, err := os.Stat(pdf.workDir); err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}

	/* get path of file to send from the done file */
	f, err := os.OpenFile(fmt.Sprintf("%s/done.txt", pdf.workDir), os.O_RDONLY, 0666)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unable to find PDF for this PID")
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
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unable to open PDF for this PID")
		return
	}
	defer in.Close()

	stat, staterr := in.Stat()
	if staterr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unable to stat PDF for this PID")
		return
	}

	logger.Printf("Sending %s to client with size %d", pdfFile, stat.Size())

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", pdf.req.pid))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmt.Sprint(stat.Size()))

	io.Copy(w, in)

	logger.Printf("PDF download for %s completed successfully", pdf.req.pid)
}

func deleteHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)

	pdf := pdfInfo{}

	pdf.req.pid = params.ByName("pid")
	pdf.req.unit = r.URL.Query().Get("unit")
	pdf.req.token = r.URL.Query().Get("token")

	pdf.subDir = pdf.req.pid
	pdf.workDir = getWorkDir(pdf.subDir, pdf.req.unit, pdf.req.token)

	if err := os.RemoveAll(pdf.workDir); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "ERROR")
		return
	}
	fmt.Fprintf(w, "DELETED")
}
