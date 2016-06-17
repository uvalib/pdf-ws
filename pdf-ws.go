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
	lf, _ := os.OpenFile("service.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	defer lf.Close()
	logger = log.New(lf, "service: ", log.LstdFlags)
	// use below to log to console....
	//logger = log.New(os.Stdout, "logger: ", log.LstdFlags)

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
		fmt.Fprintf(rw, "PDF generation failed: %s", err.Error())
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
 * Download jp2k image from fedora
 */
func downloadJp2(pid string) (j2kFileName string, err error) {
	url := fmt.Sprintf("%s/%s/datastreams/content/content", viper.GetString("fedora_url"), pid)
	logger.Printf("Downloading %s", url)

	j2kFileName = fmt.Sprintf("tmp/%s.jp2", pid)
	destFile, err := os.Create(j2kFileName)
	if err != nil {
		return
	}
	defer destFile.Close()

	response, err := http.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()

	s, err := io.Copy(destFile, response.Body)
	if err != nil {
		return
	}

	logger.Printf(fmt.Sprintf("Successful download size: %d", s))
	return
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
 * use jp2 image to generate a PDF for a page
 */
func generatePdf(pid string, pages []pageInfo) (pdfFile string, pdfErr error) {
	// iterate over page info and build a list of paths to
	// the image for that page. Older pages may only be stored on lib_content44
	// and newer pages will have a jp2k file avialble on the iiif server
	var pdfFiles []string
	var jp2Files []string
	var errors []string
	for _, val := range pages {
		logger.Printf("Generate PDF for %s", val.Filename)
		outFile := fmt.Sprintf("tmp/%s", val.Filename)
		outFile = strings.Replace(outFile, ".tif", ".pdf", 1)

		// First, try to get a file from the IIIF server mount
		srcFile, j2kErr := getIiifJp2File(val.PID)
		if j2kErr != nil {
			// Not found, try to pull it from fedora3...
			logger.Printf("%s not found, trying fedora", srcFile)
			srcFile, j2kErr = downloadJp2(val.PID)
			if j2kErr != nil {
				// Can't find anything... give up and move on
				logger.Printf("Unable to download JP2 file for PID %s: %s", val.PID, j2kErr.Error())
				errors = append(errors, fmt.Sprintf("Could not find master file image for %s", val.PID))
				continue
			}
			jp2Files = append(jp2Files, srcFile)
			logger.Printf("Found jp2 file in fedora")
		} else {
			logger.Printf("Found IIIF jp2 file: %s", srcFile)
		}

		// run imagemagick to create pdf
		cmd := "convert"
		args := []string{srcFile, "-compress", "zip", outFile}
		convErr := exec.Command(cmd, args...).Run()
		if convErr != nil {
			logger.Printf("Unable to generate PDF for %s", val.Filename)
			errors = append(errors, fmt.Sprintf("Could generate page PDF for %s", val.PID))
		} else {
			logger.Printf("GENERATED %s", outFile)
			pdfFiles = append(pdfFiles, outFile)
		}
	}

	// Now merge all of the files into 1 pdf
	// TODO do something with any errors found in errors array
	logger.Printf("Merging page PDFs into single PDF")
	pdfFile = fmt.Sprintf("tmp/%s.pdf", pid)
	outParam := fmt.Sprintf("-sOutputFile=%s", pdfFile)
	cmd := "gs"
	args := []string{"-q", "-dNOPAUSE", "-dBATCH", "-sDEVICE=pdfwrite", outParam}
	args = append(args, pdfFiles...)
	pdfErr = exec.Command(cmd, args...).Run()
	if pdfErr != nil {
		logger.Printf("Unable to generate merged PDF : %s", pdfErr.Error())
	}

	// Cleanup intermediate PDFs and downloaded jp2s
	exec.Command("rm", pdfFiles...).Run()
	if len(jp2Files) > 0 {
		exec.Command("rm", jp2Files...).Run()
	}
	return
}
