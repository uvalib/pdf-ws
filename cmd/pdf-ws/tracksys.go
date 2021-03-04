package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// the line between metadata/masterfile fields is getting blurry; just lump them together
type tsGenericPidInfo struct {
	ID       int    `json:"id,omitempty"`
	Pid      string `json:"pid,omitempty"`
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// holds metadata pid/page info
type tsPidInfo struct {
	Pid   tsGenericPidInfo
	Pages []tsGenericPidInfo
}

func getTsURL(api string, pid string, unit string) string {
	url := fmt.Sprintf("%s%s/%s", config.tsAPIHost.value, api, pid)
	if unit != "" {
		url = fmt.Sprintf("%s?unit=%s", url, unit)
	}
	return url
}

func tsGetPagesFromManifest(pid, unit, pages string) ([]tsGenericPidInfo, error) {
	url := getTsURL("/api/manifest", pid, unit)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		logger.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("failed to create new manifest request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		logger.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("failed to receive manifest response")
	}

	defer res.Body.Close()

	// parse json from body

	var allPages []tsGenericPidInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &allPages); jErr != nil {
		logger.Printf("Unmarshal() failed: %s", jErr.Error())
		return nil, fmt.Errorf("failed to unmarshal manifest response: [%s]", buf)
	}

	// filter pages, if requested

	var tsPages []tsGenericPidInfo

	if pages == "" {
		tsPages = allPages
	} else {
		pageMap := make(map[int]bool)

		for _, pageID := range strings.Split(pages, ",") {
			if pageID == "" {
				continue
			}
			pageIDVal, _ := strconv.Atoi(pageID)
			pageMap[pageIDVal] = true
		}

		for _, p := range allPages {
			if pageMap[p.ID] {
				tsPages = append(tsPages, p)
			}
		}

		logger.Printf("filtered pages from %d to %d", len(allPages), len(tsPages))
	}

	for i, p := range tsPages {
		logger.Printf("    [page %d / %d]  { [%d]  [%s]  [%s]  [%s] }", i+1, len(tsPages), p.ID, p.Pid, p.Filename, p.Title)
	}

	return tsPages, nil
}

func tsGetPidInfo(pid, unit, pages string) (*tsPidInfo, error) {
	url := getTsURL("/api/pid", pid, "")

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		logger.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("failed to create new pid request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		logger.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("failed to receive pid response")
	}

	defer res.Body.Close()

	// parse json from body

	var ts tsPidInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &ts.Pid); jErr != nil {
		logger.Printf("Unmarshal() failed: %s", jErr.Error())
		return nil, fmt.Errorf("failed to unmarshal pid response: [%s]", buf)
	}
	logger.Printf("Type            : [%s]", ts.Pid.Type)

	if ts.Pid.Type == "master_file" {
		logger.Printf("    [page 1 / 1]  { [%s]  [%s]  [%s] }", ts.Pid.Pid, ts.Pid.Filename, ts.Pid.Title)
		ts.Pages = append(ts.Pages, ts.Pid)
		return &ts, nil
	}

	if strings.Contains(ts.Pid.Type, "metadata") || strings.Contains(ts.Pid.Type, "component") {
		var mfErr error
		ts.Pages, mfErr = tsGetPagesFromManifest(pid, unit, pages)
		if mfErr != nil {
			logger.Printf("tsGetPagesFromManifest() failed: [%s]", mfErr.Error())
			return nil, mfErr
		}
		return &ts, nil
	}

	return nil, fmt.Errorf("unhandled PID type: [%s]", ts.Pid.Type)
}
