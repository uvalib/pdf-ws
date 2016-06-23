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
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

/**
 * Handle a request for a PDF of page images
 */
func pdfGenerate(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	pid := params.ByName("pid")
	embedStr := r.URL.Query().Get("embed")
	embed := true
	if len(embedStr) == 0 || embedStr == "0" {
		embed = false
	}
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

	// Get BIBL data for the passed PID
	var availability sql.NullInt64
	var biblID int
	var title string
	qs := "select b.id, b.title, b.availability_policy_id from bibls b where pid=?"
	err := db.QueryRow(qs, pid).Scan(&biblID, &title, &availability)
	switch {
	case err == sql.ErrNoRows:
		logger.Printf("%s not found", pid)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "%s not found", pid)
		return
	case err != nil:
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

	// Get data for all master files from units associated with bibl
	qs = `select m.pid, m.filename, m.title from master_files m
	      inner join units u on u.id=m.unit_id where u.bibl_id = ?`
	rows, _ := db.Query(qs, biblID)
	defer rows.Close()
	var pages []pageInfo
	for rows.Next() {
		var pg pageInfo
		err = rows.Scan(&pg.PID, &pg.Filename, &pg.Title)
		if err != nil {
			logger.Printf("Unable to retreive MasterFile data for PDF generation %s: %s", pid, err.Error())
			continue
		}
		pages = append(pages, pg)
	}

	// kick the lengthy PDF generation off in a go routine
	go generatePdf(pid, pages)

	if embed {
		fmt.Fprintf(w, "ok")
	} else {
		renderAjaxPage(pid, w)
	}
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
	var args []string
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
