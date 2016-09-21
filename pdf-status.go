package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

func statusHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	pid := params.ByName("pid")
	pidDir := fmt.Sprintf("./tmp/%s", pid)
	if _, err := os.Stat(pidDir); err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}
	if _, err := os.Stat(fmt.Sprintf("%s/done.txt", pidDir)); err == nil {
		fmt.Fprintf(w, "READY")
		return
	}
	errorFile := fmt.Sprintf("%s/fail.txt", pidDir)
	if _, err := os.Stat(errorFile); err == nil {
		fmt.Fprintf(w, "FAILED")
		os.Remove(errorFile)
		os.Remove(pidDir)
		return
	}
	fmt.Fprintf(w, "PROCESSING")
}
