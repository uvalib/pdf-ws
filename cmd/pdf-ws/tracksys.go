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
	Id              int    `json:"id,omitempty"`
	Pid             string `json:"pid,omitempty"`
	Type            string `json:"type,omitempty"`
	Title           string `json:"title,omitempty"`
	Filename        string `json:"filename,omitempty"`
	TextSource      string `json:"text_source,omitempty"`
	OcrHint         string `json:"ocr_hint,omitempty"`
	OcrCandidate    bool   `json:"ocr_candidate,omitempty"`
	OcrLanguageHint string `json:"ocr_language_hint,omitempty"`
	imageSource     string
	remoteName      string
}

// holds metadata pid/page info
type tsPidInfo struct {
	Pid       tsGenericPidInfo
	Pages     []tsGenericPidInfo
	isOcrable bool
}

func tsApiUrlForPidUnit(api, pid, unit string) string {
	url := fmt.Sprintf("%s%s", config.tsApiHost.value, api)
	url = strings.Replace(url, "{PID}", pid, -1)

	if unit != "" {
		url = fmt.Sprintf("%s?unit=%s", url, unit)
	}

	//logger.Printf("url: [%s]", url)

	return url
}

func tsGetPagesFromManifest(pid, unit, pages string) ([]tsGenericPidInfo, error) {
	url := tsApiUrlForPidUnit(config.tsApiGetManifestTemplate.value, pid, unit)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		logger.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create new manifest request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		logger.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive manifest response")
	}

	defer res.Body.Close()

	// parse json from body

	var allPages []tsGenericPidInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &allPages); jErr != nil {
		logger.Printf("Unmarshal() failed: %s", jErr.Error())
		return nil, errors.New(fmt.Sprintf("Failed to unmarshal manifest response: [%s]", buf))
	}

	// filter pages, if requested

	var tsPages []tsGenericPidInfo

	if pages == "" {
		tsPages = allPages
	} else {
		pageMap := make(map[int]bool)

		for _, pageId := range strings.Split(pages, ",") {
			if pageId == "" {
				continue
			}
			pageIdVal, _ := strconv.Atoi(pageId)
			pageMap[pageIdVal] = true
		}

		for _, p := range allPages {
			if pageMap[p.Id] {
				tsPages = append(tsPages, p)
			}
		}

		logger.Printf("filtered pages from %d to %d", len(allPages), len(tsPages))
	}

	for i, p := range tsPages {
		logger.Printf("    [page %d / %d]  { [%d]  [%s]  [%s]  [%s]  [%s] }", i+1, len(tsPages), p.Id, p.Pid, p.Filename, p.Title, p.TextSource)
	}

	return tsPages, nil
}

func tsGetPidInfo(pid, unit, pages string) (*tsPidInfo, error) {
	url := tsApiUrlForPidUnit(config.tsApiGetPidTemplate.value, pid, "")

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		logger.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create new pid request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		logger.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive pid response")
	}

	defer res.Body.Close()

	// parse json from body

	var ts tsPidInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &ts.Pid); jErr != nil {
		logger.Printf("Unmarshal() failed: %s", jErr.Error())
		return nil, errors.New(fmt.Sprintf("Failed to unmarshal pid response: [%s]", buf))
	}

	logger.Printf("Type            : [%s]", ts.Pid.Type)
	logger.Printf("TextSource      : [%s]", ts.Pid.TextSource)
	logger.Printf("OcrHint         : [%s]", ts.Pid.OcrHint)
	logger.Printf("OcrCandidate    : [%t]", ts.Pid.OcrCandidate)
	logger.Printf("OcrLanguageHint : [%s]", ts.Pid.OcrLanguageHint)

	switch {
	case ts.Pid.Type == "master_file":
		logger.Printf("    [page 1 / 1]  { [%s]  [%s]  [%s]  [%s] }", ts.Pid.Pid, ts.Pid.Filename, ts.Pid.Title, ts.Pid.TextSource)

		ts.Pages = append(ts.Pages, ts.Pid)
		return &ts, nil

	case strings.Contains(ts.Pid.Type, "metadata"):
		var mfErr error

		ts.Pages, mfErr = tsGetPagesFromManifest(pid, unit, pages)
		if mfErr != nil {
			logger.Printf("tsGetPagesFromManifest() failed: [%s]", mfErr.Error())
			return nil, mfErr
		}

		return &ts, nil
	}

	return nil, errors.New(fmt.Sprintf("Unhandled PID type: [%s]", ts.Pid.Type))
}

func tsGetMetadataPidInfo(pid, unit, pages string) (*tsPidInfo, error) {
	ts, err := tsGetPidInfo(pid, unit, pages)

	if err != nil {
		return nil, err
	}

	if strings.Contains(ts.Pid.Type, "metadata") == false {
		return nil, errors.New(fmt.Sprintf("PID is not a metadata type: [%s]", ts.Pid.Type))
	}

	// ensure there are pages to process
	if len(ts.Pages) == 0 {
		return nil, errors.New("Metadata PID does not have any pages")
	}

	// check if this is ocr-able: FIXME (DCMD-634)
	ts.isOcrable = false
	if ts.Pid.OcrCandidate == true {
		if ts.Pid.TextSource == "" || ts.Pid.TextSource == "ocr" {
			ts.isOcrable = true
		}
	} else {
		// fallback for tracksysdev until it has the new API
		if ts.Pid.OcrHint == "Regular Font" || ts.Pid.OcrHint == "Modern Font" {
			ts.isOcrable = true
		}
	}

	return ts, nil
}
