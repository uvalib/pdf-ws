package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-zoo/bone"
	"github.com/spf13/viper"
)

var db *sql.DB // global variable to share it between main and the HTTP handler
var logger *log.Logger

type pageInfo struct {
	PID      string
	Filename string
	Title    string
}

/**
 * Main entry point for the web service
 */
func main() {
	// lf, _ := os.OpenFile("service.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	// defer lf.Close()
	// logger = log.New(lf, "service: ", log.LstdFlags)
	logger = log.New(os.Stdout, "logger: ", log.LstdFlags)

	// Load cfg
	logger.Printf("===> pdf-ws staring up <===")
	logger.Printf("Load configuration...")
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("Unable to read config: %s", err.Error())
		os.Exit(1)
	}

	// Init DB connection
	logger.Printf("Init DB connection...")
	connectStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", viper.GetString("db_user"), viper.GetString("db_pass"),
		viper.GetString("db_host"), viper.GetString("db_name"))
	db, err = sql.Open("mysql", connectStr)
	if err != nil {
		fmt.Printf("Database connection failed: %s", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Set routes and start server
	mux := bone.New()
	mux.Get("/pdf/:pid", http.HandlerFunc(pdfHandler))
	mux.Get("/", http.HandlerFunc(rootHandler))
	logger.Printf("Start service on port %s", viper.GetString("port"))
	http.ListenAndServe(":"+viper.GetString("port"), mux)
}

/**
 * Handle a request for /
 */
func rootHandler(rw http.ResponseWriter, req *http.Request) {
	logger.Printf("%s %s", req.Method, req.RequestURI)
	fmt.Fprintf(rw, "PDF service. Usage: ./pdf/[pid]")
}

/**
 * Handle a request for a PDF of page images
 */
func pdfHandler(rw http.ResponseWriter, req *http.Request) {
	logger.Printf("%s %s", req.Method, req.RequestURI)
	pid := bone.GetValue(req, "pid")

	// Get BIBL data for the passed PID
	var availability sql.NullInt64
	var biblID int
	var title, description string
	qs := "select b.id,b.title,b.description,b.availability_policy_id from bibls b where pid=?"
	err := db.QueryRow(qs, pid).Scan(&biblID, &title, &description, &availability)
	switch {
	case err == sql.ErrNoRows:
		logger.Printf("%s not found", pid)
		rw.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rw, "%s not found", pid)
		return
	case err != nil:
		logger.Printf("Request failed: %s", err.Error())
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "Unable to PDF: %s", err.Error())
		return
	}

	// Must have availability set
	if availability.Valid == false {
		logger.Printf("%s not found", pid)
		rw.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rw, "%s not found", pid)
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
			fmt.Fprintf(rw, "Unable to retreive MasterFile data for PDF generation: %s", err.Error())
			return
		}
		pages = append(pages, pg)
	}
	logger.Printf("%s has %d pages. Generating PDF...", pid, len(pages))
	pdfFile, err := generatePdf(pid, pages)
	if err != nil {
		// TODO
		return
	}

	logger.Printf("PDF for %s generated. Sending to client...", pid)
	rw.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.pdf", pid))
	rw.Header().Set("Content-Type", "application/pdf")
	in, err := os.Open(pdfFile)
	if err != nil {
		return
	}
	defer in.Close()
	defer os.Remove(pdfFile)
	io.Copy(rw, in)
	logger.Printf("PDF for %s completed successfully", pid)
}

/**
 * use tif or jp2k image to generate a PDF for a page
 */
func generatePdf(pid string, pages []pageInfo) (pdfFile string, err error) {
	// iterate over page info and build a list of paths to
	// the image for that page. Older pages may only be stored on lib_content44
	// and newer pages will have a jp2k file avialble on the iiif server
	var pdfFiles []string
	for _, val := range pages {
		logger.Printf("PDF for %s", val.Filename)
		var srcFile, outFile string
		if strings.Contains(strings.ToLower(val.PID), "tsm:") {
			// this is a newer masterfile that has jp2k files in iiif data dirs
			//filePath = viper.GetString("jp2k_dir")
			// TODO
		} else {
			// this is an older file. only full scale tif available in archive
			outFile = fmt.Sprintf("tmp/%s", val.Filename)
			outFile = strings.Replace(outFile, ".tif", ".pdf", 1)
			bits := strings.Split(val.Filename, "_")
			srcFile = fmt.Sprintf("%s/%s/%s[0]", viper.GetString("archive_mount"), bits[0], val.Filename)
		}

		// run imagemagick to create pdf
		if len(srcFile) > 0 {
			cmd := "convert"
			args := []string{srcFile, "-compress", "zip", outFile}
			err = exec.Command(cmd, args...).Run()
			if err != nil {
				logger.Printf("Unable to generate PDF for %s", val.Filename)
			} else {
				logger.Printf("GENERATED %s", outFile)
				pdfFiles = append(pdfFiles, outFile)
			}
		}
	}

	// Now merge all of the files into 1 pdf
	// gs -q -dNOPAUSE -dBATCH -sDEVICE=pdfwrite -sOutputFile=merged.pdf test.pdf test2.pdf
	logger.Printf("Merging page PDFs into single PDF")
	pdfFile = fmt.Sprintf("tmp/%s.pdf", pid)
	outParam := fmt.Sprintf("-sOutputFile=%s", pdfFile)
	cmd := "gs"
	args := []string{"-q", "-dNOPAUSE", "-dBATCH", "-sDEVICE=pdfwrite", outParam}
	args = append(args, pdfFiles...)
	err = exec.Command(cmd, args...).Run()
	if err != nil {
		logger.Printf("Unable to generate MERGED PDF : %s", err.Error())
	}

	// Cleanup intermediate PDFs
	exec.Command("rm", pdfFiles...).Run()
	return
}
