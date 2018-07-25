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
	port string
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
	lf, _ := os.OpenFile("service.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0777)
	defer lf.Close()
	logger = log.New(lf, "service: ", log.LstdFlags)
	// use below to log to console....
	//logger = log.New(os.Stdout, "logger: ", log.LstdFlags)

	// Load cfg
	logger.Printf("===> pdf-ws staring up <===")
	logger.Printf("Load configuration...")
	getConfiguration()

	// Init DB connection
	logger.Printf("Init DB connection...")
	connectStr := fmt.Sprintf("%s:%s@tcp(%s)/%s?allowOldPasswords=%s", config.dbUser, config.dbPass,
		config.dbHost, config.dbName, strconv.FormatBool(config.dbAllowOldPasswords))

	db, err := sql.Open("mysql", connectStr)
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
	logger.Printf("Start service on port %s", config.port)

	if config.useHttps == true {
		log.Fatal(http.ListenAndServeTLS(":"+config.port, config.sslCrt, config.sslKey, cors.Default().Handler(mux)))
	} else {
		log.Fatal(http.ListenAndServe(":"+config.port, cors.Default().Handler(mux)))
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
			log.Printf("FATAL: -%s parameter or %s environment variable is required",optFlag,optEnv)
			logger.Printf("FATAL: -%s parameter or %s environment variable is required",optFlag,optEnv)
			os.Exit(1)
		}
	}

	return newValue
}

func getConfiguration() {
	// populate values from the command line first
	flag.StringVar(&config.port, "l", "", "[l]isten port")
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

	// override these with environment variables, if defined
	var env string

	env = os.Getenv("db_old_passwords")
	if env != "" {
		config.dbAllowOldPasswords, _ = strconv.ParseBool(env)
	}

	env = os.Getenv("allow_unpublished")
	if env != "" {
		config.allowUnpublished, _ = strconv.ParseBool(env)
	}

	// fallback to environment variables for these, and exit if not defined
	config.port = ensureDefined(config.port,"l","port")
	config.dbHost = ensureDefined(config.dbHost,"h","db_host")
	config.dbName = ensureDefined(config.dbName,"n","db_name")
	config.dbUser = ensureDefined(config.dbUser,"u","db_user")
	config.dbPass = ensureDefined(config.dbPass,"p","db_pass")
	config.jp2kDir = ensureDefined(config.jp2kDir,"j","jp2k_dir")
	config.archiveDir = ensureDefined(config.archiveDir,"m","archive_mount")
	config.iiifUrlTemplate = ensureDefined(config.iiifUrlTemplate,"i","iiif_url_template")

	if config.useHttps {
		config.sslCrt = ensureDefined(config.sslCrt,"c","ssl_crt")
		config.sslKey = ensureDefined(config.sslKey,"k","ssl_key")
	}
}
