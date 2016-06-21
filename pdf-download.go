package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

func downloadHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	pid := params.ByName("pid")
	pidDir := fmt.Sprintf("./tmp/%s", pid)
	if _, err := os.Stat(pidDir); err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}
	f, err := os.OpenFile(fmt.Sprintf("%s/done.txt", pidDir), os.O_RDONLY, 0777)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unable to find PDF for this PID")
		return
	}
	logger.Printf("Sending %s to client", b)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.pdf", pid))
	w.Header().Set("Content-Type", "application/pdf")
	in, err := os.Open(string(b))
	if err != nil {
		return
	}
	defer in.Close()
	defer os.RemoveAll(pidDir)
	io.Copy(w, in)
	logger.Printf("PDF for %s completed successfully", pid)
}
