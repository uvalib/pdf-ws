package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// git commit used for this build; supplied at compile time
var gitCommit string

// structures

type serviceVersion struct {
	BuildVersion string `json:"build,omitempty"`
	GoVersion    string `json:"go_version,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
}

type healthcheckDetails struct {
	Domain healthCheckStatus `json:"pdf_service,omitempty"`
}

type healthCheckStatus struct {
	Healthy bool   `json:"healthy,omitempty"`
	Message string `json:"message,omitempty"`
}

// globals

var versionDetails *serviceVersion

// functions

func initVersion() {
	buildVersion := "unknown"
	files, _ := filepath.Glob("buildtag.*")
	if len(files) == 1 {
		buildVersion = strings.Replace(files[0], "buildtag.", "", 1)
	}

	versionDetails = &serviceVersion{
		BuildVersion: buildVersion,
		GoVersion:    fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		GitCommit:    gitCommit,
	}
}

func firstElementOf(s []string) string {
	// return first element of slice, or blank string if empty
	val := ""

	if len(s) > 0 {
		val = s[0]
	}

	return val
}

func getWorkSubDir(pid, unit, token string) string {
	subDir := pid

	switch {
	case token != "":
		subDir = token

	case unit != "":
		unitID, _ := strconv.Atoi(unit)
		if unitID > 0 {
			subDir = fmt.Sprintf("%s/%d", pid, unitID)
		}
	}

	return subDir
}

//
// end of file
//
