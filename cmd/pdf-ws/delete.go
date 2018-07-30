package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

func deleteHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	pid := params.ByName("pid")
	token := r.URL.Query().Get("token")
	workDir := fmt.Sprintf("./tmp/%s", pid)
	if len(token) > 0 {
		workDir = fmt.Sprintf("./tmp/%s", token)
	}
	if err := os.RemoveAll(workDir); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "ERROR")
		return
	}
	fmt.Fprintf(w, "DELETED")
}
