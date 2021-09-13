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

type tsResult struct {
	status int   // http status code
	err    error // error, if any
}

// holds metadata pid/page info
type tsPidInfo struct {
	Pid   tsGenericPidInfo
	Pages []tsGenericPidInfo
}

func (c *clientContext) getTsURL(api, pid, unit string) string {
	url := fmt.Sprintf("%s%s/%s", config.tsAPIHost.value, api, pid)
	if unit != "" {
		url = fmt.Sprintf("%s?unit=%s", url, unit)
	}
	c.info("tracksys url: %s", url)
	return url
}

func (c *clientContext) tsGetPagesFromManifest() ([]tsGenericPidInfo, tsResult) {
	var tsPages []tsGenericPidInfo

	url := c.getTsURL("/api/manifest", c.req.pid, c.req.unit)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		c.err("NewRequest() failed: %s", reqErr.Error())
		return tsPages, tsResult{status: http.StatusInternalServerError, err: errors.New("failed to create tracksys manifest request")}
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		c.err("client.Do() failed: %s", resErr.Error())
		return tsPages, tsResult{status: http.StatusInternalServerError, err: errors.New("failed to receive tracksys manifest response")}
	}

	defer res.Body.Close()

	// check for known scenarios before attempting to parse as json

	buf, _ := ioutil.ReadAll(res.Body)
	str := string(buf)
	if str == "no masterfiles found" {
		c.warn("no masterfiles found for pid: [%s]", c.req.pid)
		return tsPages, tsResult{status: http.StatusNotFound, err: errors.New("no pages found for this pid")}
	}

	// parse json from body

	var allPages []tsGenericPidInfo

	if jErr := json.Unmarshal(buf, &allPages); jErr != nil {
		c.err("Unmarshal() failed: %s", jErr.Error())
		return tsPages, tsResult{status: http.StatusInternalServerError, err: fmt.Errorf("failed to unmarshal tracksys manifest response: [%s]", buf)}
	}

	// filter pages, if requested

	if c.req.pages == "" {
		tsPages = allPages
	} else {
		pageMap := make(map[int]bool)

		for _, pageID := range strings.Split(c.req.pages, ",") {
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

		c.info("filtered pages from %d to %d", len(allPages), len(tsPages))
	}

	return tsPages, tsResult{status: http.StatusOK}
}

func (c *clientContext) tsGetPidInfo() tsResult {
	url := c.getTsURL("/api/pid", c.req.pid, "")

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		c.err("NewRequest() failed: %s", reqErr.Error())
		return tsResult{status: http.StatusInternalServerError, err: errors.New("failed to create tracksys pid request")}
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		c.err("client.Do() failed: %s", resErr.Error())
		return tsResult{status: http.StatusInternalServerError, err: errors.New("failed to receive tracksys pid response")}
	}

	defer res.Body.Close()

	// parse json from body

	var ts tsPidInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &ts.Pid); jErr != nil {
		c.err("Unmarshal() failed: %s", jErr.Error())
		return tsResult{status: http.StatusInternalServerError, err: fmt.Errorf("failed to unmarshal pid response: [%s]", buf)}
	}
	c.info("Type            : [%s]", ts.Pid.Type)

	switch {
	case ts.Pid.Type == "master_file":
		ts.Pages = []tsGenericPidInfo{ts.Pid}

	case strings.Contains(ts.Pid.Type, "metadata") || strings.Contains(ts.Pid.Type, "component"):
		tsPages, res := c.tsGetPagesFromManifest()
		if res.err != nil {
			return res
		}
		ts.Pages = tsPages

	default:
		return tsResult{status: http.StatusInternalServerError, err: fmt.Errorf("unhandled PID type: [%s]", ts.Pid.Type)}
	}

	switch len(ts.Pages) {
	case 0:
		c.info("%s pid %s has no pages?", ts.Pid.Type, c.req.pid)

	case 1:
		c.info("%s pid %s has 1 page: { %s }", ts.Pid.Type, c.req.pid, ts.Pid.Pid)

	default:
		c.info("%s pid %s has %d pages: { %s ... %s }", ts.Pid.Type, c.req.pid, len(ts.Pages), ts.Pages[0].Pid, ts.Pages[len(ts.Pages)-1].Pid)
	}

	c.pdf.ts = &ts

	return tsResult{status: http.StatusOK}
}
