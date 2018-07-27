package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"flag"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

const version = "1.6"

type pageInfo struct {
	PID      string
	Filename string
	Title    sql.NullString
}

type configData struct {
	listenPort string
	dbHost string
	dbName string
	dbUser string
	dbPass string
	dbAllowOldPasswords bool
	jp2kDir string
	archiveDir string
	allowUnpublished bool
	iiifUrlTemplate string
	useHttps bool
	sslCrt string
	sslKey string
}

var db *sql.DB
var logger *log.Logger
var config configData

/**
 * Main entry point for the web service
 */
func main() {
/*
	lf, _ := os.OpenFile("service.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0777)
	defer lf.Close()
	logger = log.New(lf, "service: ", log.LstdFlags)
*/
	// use below to log to console....
	logger = log.New(os.Stdout, "", log.LstdFlags)

	// Load cfg
	logger.Printf("===> pdf-ws staring up <===")
	logger.Printf("Load configuration...")
	getConfigValues()

	// Init DB connection
	logger.Printf("Init DB connection...")
	connectStr := fmt.Sprintf("%s:%s@tcp(%s)/%s?allowOldPasswords=%s", config.dbUser, config.dbPass,
		config.dbHost, config.dbName, strconv.FormatBool(config.dbAllowOldPasswords))

	var err error
	db, err = sql.Open("mysql", connectStr)
	if err != nil {
		fmt.Printf("Database connection failed: %s", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Set routes and start server
	mux := httprouter.New()
	mux.GET("/", rootHandler)
	mux.GET("/:pid", pdfGenerate)
	mux.GET("/:pid/status", statusHandler)
	mux.GET("/:pid/download", downloadHandler)
	mux.GET("/:pid/delete", deleteHandler)
	logger.Printf("Start service on port %s", config.listenPort)

	if config.useHttps == true {
		log.Fatal(http.ListenAndServeTLS(":"+config.listenPort, config.sslCrt, config.sslKey, cors.Default().Handler(mux)))
	} else {
		log.Fatal(http.ListenAndServe(":"+config.listenPort, cors.Default().Handler(mux)))
	}
}

/**
 * Handle a request for /
 */
func rootHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	fmt.Fprintf(w, "PDF service version %s", version)
}

func ensureDefined(value string, optFlag string, optEnv string) string {
	newValue := value

	if len(newValue) == 0 {
		newValue = os.Getenv(optEnv)

		if len(newValue) == 0 {
			logger.Printf("FATAL: -%s parameter or %s environment variable is required",optFlag,optEnv)
			os.Exit(1)
		}
	}

	return newValue
}

func preferEnvVar(value bool, optEnv string) bool {
	if env := os.Getenv(optEnv); env != "" {
		newValue, _ := strconv.ParseBool(env)
		return newValue
	}

	return value
}

func getConfigValues() {
	// populate values from the command line first
	flag.StringVar(&config.listenPort, "l", "", "[l]isten port")
	flag.StringVar(&config.dbHost, "h", "", "database [h]ost")
	flag.StringVar(&config.dbName, "n", "", "database [n]ame")
	flag.StringVar(&config.dbUser, "u", "", "database [u]ser")
	flag.StringVar(&config.dbPass, "p", "", "database [p]assword")
	flag.BoolVar(&config.dbAllowOldPasswords, "o", false, "allow [o]ld database passwords")
	flag.StringVar(&config.jp2kDir, "j", "", "[j]p2k directory")
	flag.StringVar(&config.archiveDir, "m", "", "archive [m]ount directory")
	flag.BoolVar(&config.allowUnpublished, "a", false, "[a]llow unpublished")
	flag.StringVar(&config.iiifUrlTemplate, "i", "", "[i]iif url template")
	flag.BoolVar(&config.useHttps, "s", false, "use http[s]")
	flag.StringVar(&config.sslCrt, "c", "", "ssl [c]rt")
	flag.StringVar(&config.sslKey, "k", "", "ssl [k]ey")
	flag.Parse()

	// for these, override with environment variables, if defined
	config.dbAllowOldPasswords = preferEnvVar(config.dbAllowOldPasswords,"PDFWS_DB_ALLOW_OLD_PASSWORDS")
	config.allowUnpublished = preferEnvVar(config.allowUnpublished,"PDFWS_ALLOW_UNPUBLISHED")
	config.useHttps = preferEnvVar(config.useHttps,"PDFWS_USE_HTTPS")

	// for these, fall back to environment variables if not given on command line, and exit if empty
	config.listenPort = ensureDefined(config.listenPort,"l","PDFWS_LISTEN_PORT")
	config.dbHost = ensureDefined(config.dbHost,"h","PDFWS_DB_HOST")
	config.dbName = ensureDefined(config.dbName,"n","PDFWS_DB_NAME")
	config.dbUser = ensureDefined(config.dbUser,"u","PDFWS_DB_USER")
	config.dbPass = ensureDefined(config.dbPass,"p","PDFWS_DB_PASS")
	config.jp2kDir = ensureDefined(config.jp2kDir,"j","PDFWS_JP2K_DIR")
	config.archiveDir = ensureDefined(config.archiveDir,"m","PDFWS_ARCHIVE_DIR")
	config.iiifUrlTemplate = ensureDefined(config.iiifUrlTemplate,"i","PDFWS_IIIF_URL_TEMPLATE")
	if config.useHttps {
		config.sslCrt = ensureDefined(config.sslCrt,"c","PDFWS_SSL_CRT")
		config.sslKey = ensureDefined(config.sslKey,"k","PDFWS_SSL_KEY")
	}

	logger.Printf("CONFIG: listenPort          = [%s]",config.listenPort)
	logger.Printf("CONFIG: dbHost              = [%s]",config.dbHost)
	logger.Printf("CONFIG: dbName              = [%s]",config.dbName)
	logger.Printf("CONFIG: dbUser              = [%s]",config.dbUser)
	logger.Printf("CONFIG: dbPass              = [REDACTED]");
	logger.Printf("CONFIG: dbAllowOldPasswords = [%s]",strconv.FormatBool(config.dbAllowOldPasswords))
	logger.Printf("CONFIG: jp2kDir             = [%s]",config.jp2kDir)
	logger.Printf("CONFIG: archiveDir          = [%s]",config.archiveDir)
	logger.Printf("CONFIG: allowUnpublished    = [%s]",strconv.FormatBool(config.allowUnpublished))
	logger.Printf("CONFIG: iiifUrlTemplate     = [%s]",config.iiifUrlTemplate)
	logger.Printf("CONFIG: useHttps            = [%s]",strconv.FormatBool(config.useHttps))
	logger.Printf("CONFIG: sslCrt              = [%s]",config.sslCrt)
	logger.Printf("CONFIG: sslKey              = [%s]",config.sslKey)
}
