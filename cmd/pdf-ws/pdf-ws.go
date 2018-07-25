package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

var db *sql.DB
var logger *log.Logger

const version = "1.6"

type pageInfo struct {
	PID      string
	Filename string
	Title    sql.NullString
}

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

	// Init DB connection
	logger.Printf("Init DB connection...")
	connectStr := fmt.Sprintf("%s:%s@tcp(%s)/%s?allowOldPasswords=%s", os.Getenv("db_user"), os.Getenv("db_pass"),
		os.Getenv("db_host"), os.Getenv("db_name"), os.Getenv("db_old_passwords"))
	// need this line, otherwise "undefined: err":
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
	logger.Printf("Start service on port %s", os.Getenv("port"))

	if os.Getenv("https") == "true" {
		crt := os.Getenv("ssl_crt")
		key := os.Getenv("ssl_key")
		log.Fatal(http.ListenAndServeTLS(":"+os.Getenv("port"), crt, key, cors.Default().Handler(mux)))
	} else {
		log.Fatal(http.ListenAndServe(":"+os.Getenv("port"), cors.Default().Handler(mux)))
	}
}

/**
 * Handle a request for /
 */
func rootHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	fmt.Fprintf(w, "PDF service version %s", version)
}
