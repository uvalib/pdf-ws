package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

func determinePidType(pid string) (pidType string) {
	var cnt int
	pidType = "invalid"
	qs := "select count(*) as cnt from metadata b where pid=?"
	db.QueryRow(qs, pid).Scan(&cnt)
	if cnt == 1 {
		pidType = "metadata"
		return
	}

	qs = "select count(*) as cnt from master_files b where pid=?"
	db.QueryRow(qs, pid).Scan(&cnt)
	if cnt == 1 {
		pidType = "master_file"
		return
	}

	return
}

/**
 * Handle a request for a PDF of page images
 */
func pdfGenerate(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	pid := params.ByName("pid")

	// pull query string params; embed and unit
	embedStr := r.URL.Query().Get("embed")
	embed := true
	if len(embedStr) == 0 || embedStr == "0" {
		embed = false
	}
	unitID, _ := strconv.Atoi(r.URL.Query().Get("unit"))

	// See if destination already extsts...
	pdfDestPath := fmt.Sprintf("./tmp/%s", pid)
	if _, err := os.Stat(pdfDestPath); err == nil {
		// path already exists; don't start another request, just treat
		// this one is if it was successful and render the ajax page
		if embed {
			fmt.Fprintf(w, "ok")
		} else {
			renderAjaxPage(pid, w)
		}
		return
	}

	// Determine what this pid is. Fail if it can't be determined
	pidType := determinePidType(pid)
	logger.Printf("PID %s is a %s", pid, pidType)
	var pages []pageInfo
	var err error
	if pidType == "metadata" {
		pages, err = getMetadataPages(pid, w, unitID)
		if err != nil {
			return
		}
	} else if pidType == "master_file" {
		pages, err = getMasterFilePages(pid, w)
		if err != nil {
			return
		}
	} else {
		logger.Printf("Unknown PID %s", pid)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "PID %s not found", pid)
		return
	}

	// kick the lengthy PDF generation off in a go routine
	go generatePdf(pid, pages)

	// Render a simple ok message or kick an ajax polling loop
	if embed {
		fmt.Fprintf(w, "ok")
	} else {
		renderAjaxPage(pid, w)
	}
}

func getMasterFilePages(pid string, w http.ResponseWriter) (pages []pageInfo, err error) {
	var pg pageInfo
	var title sql.NullString
	qs := `select pid, filename, title from master_files where pid = ?`
	err = db.QueryRow(qs, pid).Scan(&pg.PID, &pg.Filename, &title)
	if err != nil {
		logger.Printf("Request failed: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unable to PDF: %s", err.Error())
		return
	}
	pg.Title = title.String
	pages = append(pages, pg)
	return
}

// NOTE: when called from Tracksys, the unitID will be set. Honor this and generate a PDF
// of all masterfiles in that unit regardless of published status. When called from Virgo,
// unitID will NOT be set. Run through all units and only include those that are
// in the DL and are publicly visible
//
func getMetadataPages(pid string, w http.ResponseWriter, unitID int) (pages []pageInfo, err error) {
	// Get metadata for the passed PID
	var availability sql.NullInt64
	var metadataID int
	var title string
	qs := "select id, title, availability_policy_id from metadata where pid=?"
	err = db.QueryRow(qs, pid).Scan(&metadataID, &title, &availability)
	if err != nil {
		logger.Printf("Request failed: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Unable to PDF: %s", err.Error())
		return
	}

	// Must have availability set
	if availability.Valid == false && viper.GetBool("allow_unpublished") == false {
		logger.Printf("%s not found", pid)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "%s not found", pid)
		return
	}

	// Get data for all master files from units associated with metadata / unit
	qsID := metadataID
	qs = `select m.pid, m.filename, m.title from master_files m
         inner join units u on u.id = m.unit_id
         where metadata_id = ? and u.include_in_dl = 1`
	if unitID > 0 {
		qs = `select pid, filename, title from master_files where unit_id = ?`
		qsID = unitID
	}
	rows, _ := db.Query(qs, qsID)
	defer rows.Close()
	for rows.Next() {
		var pg pageInfo
		err = rows.Scan(&pg.PID, &pg.Filename, &pg.Title)
		if err != nil {
			logger.Printf("Unable to retreive MasterFile data for PDF generation %s: %s", pid, err.Error())
			continue
		}
		pages = append(pages, pg)
	}
	return
}

/*
 * Render a simple html page that will poll for status of this PDF, and download it when done
 */
func renderAjaxPage(pid string, w http.ResponseWriter) {
	varmap := map[string]interface{}{
		"pid": pid,
	}
	tmpl, _ := template.ParseFiles("index.html")
	err := tmpl.ExecuteTemplate(w, "index.html", varmap)
	if err != nil {
		logger.Printf("Unable to render ajax polling page for %s: %s", pid, err.Error())
		fmt.Fprintf(w, "Unable to render ajax polling page for %s: %s", pid, err.Error())
	}
}

func downloadJpgFromIiif(outPath string, pid string) (jpgFileName string, err error) {
	url := viper.GetString("iiif_url_template")
	url = strings.Replace(url, "$PID", pid, 1)

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
	srcFile := fmt.Sprintf("%s/%s/%s", viper.GetString("archive_mount"), bits[0], tifFile)
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

/**
 * use jp2 or archived tif files to generate a multipage PDF for a PID
 */
func generatePdf(pid string, pages []pageInfo) {
	// Make sure the work directory exists
	outPath := fmt.Sprintf("./tmp/%s", pid)
	os.MkdirAll(outPath, 0777)

	// iterate over page info and build a list of paths to
	// the image for that page. Older pages may only be stored on lib_content44
	// and newer pages will have a jp2k file avialble on the iiif server
	var jpgFiles []string
	for _, val := range pages {
		logger.Printf("Get reduced size jpg for %s %s", val.PID, val.Filename)

		// First, try to get a JPG file from the IIIF server mount
		jpgFile, jpgErr := downloadJpgFromIiif(outPath, val.PID)
		if jpgErr != nil {
			// not found. work from the archival tif file
			logger.Printf("No JPG found on IIIF server; using archival tif...")
			jpgFile, jpgErr = jpgFromTif(outPath, val.PID, val.Filename)
			if jpgErr != nil {
				logger.Printf("Unable to find source image for masterFile %s. Skipping.", val.PID)
				continue
			}
		}
		jpgFiles = append(jpgFiles, jpgFile)
	}

	// Now merge all of the files into 1 pdf
	pdfFile := fmt.Sprintf("tmp/%s/%s.pdf", pid, pid)
	logger.Printf("Merging page PDFs into single PDF %s", pdfFile)
	cmd := "convert"
	args := []string{"-density", "150"}
	args = append(args, jpgFiles...)
	args = append(args, pdfFile)
	convErr := exec.Command(cmd, args...).Run()
	if convErr != nil {
		logger.Printf("Unable to generate merged PDF : %s", convErr.Error())
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", outPath), os.O_CREATE|os.O_RDWR, 0777)
		defer ef.Close()
		if _, err := ef.WriteString(convErr.Error()); err != nil {
			logger.Printf("Unable to write error file : %s", convErr.Error())
		}
	} else {
		logger.Printf("Generated PDF : %s", pdfFile)
		ef, _ := os.OpenFile(fmt.Sprintf("%s/done.txt", outPath), os.O_CREATE|os.O_RDWR, 0777)
		defer ef.Close()
		if _, err := ef.WriteString(pdfFile); err != nil {
			logger.Printf("Unable to write done file : %s", err.Error())
		}
	}

	// Cleanup intermediate jpgFiles
	exec.Command("rm", jpgFiles...).Run()
}
