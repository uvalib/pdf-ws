package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"
)

func downloadHandler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	logger.Printf("%s %s", r.Method, r.RequestURI)
	pid := params.ByName("pid")
	token := r.URL.Query().Get("token")
	workDir := fmt.Sprintf("%s/%s", config.storageDir.value, pid)
	if len(token) > 0 {
		workDir = fmt.Sprintf("%s/%s", config.storageDir.value, token)
	}
	if _, err := os.Stat(workDir); err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
		return
	}

	/* get path of file to send from the done file */
	f, err := os.OpenFile(fmt.Sprintf("%s/done.txt", workDir), os.O_RDONLY, 0666)
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

	pdfFile := strings.TrimSpace(string(b))

	/* seamless conversion from old locations */
	if strings.HasPrefix(pdfFile,"tmp/") {
		logger.Printf("Old location: [%s]",pdfFile)
		pdfFile = strings.Replace(pdfFile,"tmp",config.storageDir.value,1)
		logger.Printf("New location: [%s]",pdfFile)

		/* write out new location */
	}

	/* get file size */

	in, err := os.Open(pdfFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unable to open PDF for this PID")
		return
	}
	defer in.Close()

	stat, staterr := in.Stat()
	if staterr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Unable to stat PDF for this PID")
		return
	}

	logger.Printf("Sending %s to client with size %d", pdfFile, stat.Size())

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", pid))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmt.Sprint(stat.Size()));

	io.Copy(w, in)

	logger.Printf("PDF download for %s completed successfully", pid)
}
