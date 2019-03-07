package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

const version = "2.0"

var logger *log.Logger
var client *http.Client

/**
 * Main entry point for the web service
 */
func main() {
	logger = log.New(os.Stdout, "", log.LstdFlags)

	// Load cfg
	logger.Printf("===> pdf-ws staring up <===")
	logger.Printf("Load configuration...")
	getConfigValues()

	// initialize http client
	client = &http.Client{Timeout: 10 * time.Second}

	// Set routes and start server
	mux := httprouter.New()
	mux.GET("/", rootHandler)
	mux.GET("/pdf/:pid", generateHandler)
	mux.GET("/pdf/:pid/status", statusHandler)
	mux.GET("/pdf/:pid/download", downloadHandler)
	mux.GET("/pdf/:pid/delete", deleteHandler)
	logger.Printf("Start service on port %s", config.listenPort.value)

	log.Fatal(http.ListenAndServe(":"+config.listenPort.value, cors.Default().Handler(mux)))
}

/**
 * Handle a request for /
 */
func rootHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	fmt.Fprintf(w, "PDF service version %s", version)
}
