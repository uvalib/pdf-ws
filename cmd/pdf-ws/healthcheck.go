package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

func healthCheckHandler(rw http.ResponseWriter, req *http.Request, params httprouter.Params) {
	rw.Header().Set("Content-Type", "application/json")

	// check tracksys database connection
	log.Printf("Checking Tracksys...")

	tsPid := "tsb:18139"
	cnt, expected_cnt := 0, 1
	tsStatus := true

	qs := "select count(*) as cnt from metadata b where pid=?"
	db.QueryRow(qs, tsPid).Scan(&cnt)

	if cnt != expected_cnt {
		log.Printf("ERROR: Tracksys database: count mismatch: expected %d, got %d", expected_cnt, cnt)
		tsStatus = false
	}

	// check IIIF server
	log.Printf("Checking IIIF...")

	iiifPid := "tsm:1250722"
	size, expected_size := 0, 154948
	iiifStatus := true

	url := config.iiifUrlTemplate
	url = strings.Replace(url, "$PID", iiifPid, 1)

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("ERROR: IIIF service: Get: (%s)", err)
		iiifStatus = false
	} else {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("ERROR: IIIF service: ReadAll: (%s)", err)
			iiifStatus = false
		} else {
			resp.Body.Close()
			size = len(b)
			if size != expected_size {
				log.Printf("ERROR: IIIF service: size mismatch: expected %d, got %d", expected_size, size)
				iiifStatus = false
			}
		}
	}

	out := fmt.Sprintf(`{"alive": true, "iiif": %t, "tracksys": %t}`, iiifStatus, tsStatus)

	if iiifStatus == false {
		http.Error(rw, out, http.StatusInternalServerError)
	} else {
		fmt.Fprintf(rw, out)
	}
}
