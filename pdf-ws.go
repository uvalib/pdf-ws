package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
    "github.com/rs/cors"
)

var db *sql.DB
var logger *log.Logger

const version = "1.0.1"

type pageInfo struct {
	PID      string
	Filename string
	Title    string
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
	connectStr := fmt.Sprintf("%s:%s@tcp(%s)/%s?allowOldPasswords=%s", viper.GetString("db_user"), viper.GetString("db_pass"),
		viper.GetString("db_host"), viper.GetString("db_name"), viper.GetString("db_old_passwords"))
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
	logger.Printf("Start service on port %s", viper.GetString("port"))
	log.Fatal(http.ListenAndServe(":"+viper.GetString("port"), cors.Default().Handler(mux)))
}

/**
 * Handle a request for /
 */
func rootHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	fmt.Fprintf(w, "PDF service version %s", version)
}
