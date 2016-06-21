package main

import (
	"database/sql"
	"fmt"
	"html/template"
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
	if availability.Valid == false {
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

/**
 * See if a jp2 for the PID exists on the IIIF mount
 */
func getIiifJp2File(pid string) (jp2File string, err error) {
	parts := strings.Split(pid, ":")
	baseName := parts[1]
	digits := strings.Split(baseName, "")
	dirs := []string{parts[0]}
	var buff string
	for i, v := range digits {
		if i > 0 && i%2 == 0 {
			dirs = append(dirs, buff)
			buff = ""
		}
		buff = buff + v
	}
	if len(buff) > 0 {
		dirs = append(dirs, buff)
	}

	jp2File = fmt.Sprintf("%s/%s/%s.jp2", viper.GetString("jp2k_dir"), strings.Join(dirs, "/"), baseName)
	_, err = os.Stat(jp2File)
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
	var pdfFiles []string
	var jp2Files []string
	var errors []string
	for _, val := range pages {
		logger.Printf("Generate PDF for %s", val.Filename)
		outFile := fmt.Sprintf("%s/%s", outPath, val.Filename)
		outFile = strings.Replace(outFile, ".tif", ".pdf", 1)

		// First, try to get a file from the IIIF server mount
		srcFile, j2kErr := getIiifJp2File(val.PID)
		if j2kErr != nil {
			bits := strings.Split(val.Filename, "_")
			srcFile = fmt.Sprintf("%s/%s/%s[0]", viper.GetString("archive_mount"), bits[0], val.Filename)
			logger.Printf("Using archived file as source: %s", srcFile)
		} else {
			logger.Printf("Found IIIF jp2 file: %s", srcFile)
		}

		// run imagemagick to create pdf
		cmd := "convert"
		args := []string{"-resize", "2048", srcFile, "-compress", "jpeg", outFile}
		convErr := exec.Command(cmd, args...).Run()
		if convErr != nil {
			logger.Printf("Unable to generate PDF for %s", val.Filename)
			errors = append(errors, fmt.Sprintf("Could generate page PDF for %s", val.PID))
		} else {
			logger.Printf("Generated %s", outFile)
			pdfFiles = append(pdfFiles, outFile)
		}
	}

	// Now merge all of the files into 1 pdf
	// TODO do something with any errors found in errors array
	pdfFile := fmt.Sprintf("tmp/%s/%s.pdf", pid, pid)
	logger.Printf("Merging page PDFs into single PDF %s", pdfFile)
	outParam := fmt.Sprintf("-sOutputFile=%s", pdfFile)
	cmd := "gs"
	args := []string{"-q", "-dNOPAUSE", "-dBATCH", "-sDEVICE=pdfwrite", outParam}
	args = append(args, pdfFiles...)
	pdfErr := exec.Command(cmd, args...).Run()
	if pdfErr != nil {
		logger.Printf("Unable to generate merged PDF : %s", pdfErr.Error())
		ef, _ := os.OpenFile(fmt.Sprintf("%s/fail.txt", outPath), os.O_CREATE|os.O_RDWR, 0777)
		defer ef.Close()
		if _, err := ef.WriteString(pdfErr.Error()); err != nil {
			logger.Printf("Unable to write error file : %s", pdfErr.Error())
		}
	}

	// Cleanup intermediate PDFs and downloaded jp2s
	exec.Command("rm", pdfFiles...).Run()
	if len(jp2Files) > 0 {
		exec.Command("rm", jp2Files...).Run()
	}

	logger.Printf("Generated PDF : %s", pdfFile)
	ef, _ := os.OpenFile(fmt.Sprintf("%s/done.txt", outPath), os.O_CREATE|os.O_RDWR, 0777)
	defer ef.Close()
	if _, err := ef.WriteString(pdfFile); err != nil {
		logger.Printf("Unable to write done file : %s", pdfErr.Error())
	}
}
