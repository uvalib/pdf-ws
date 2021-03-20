package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// structures

type serviceVersion struct {
	Version   string `json:"version,omitempty"`
	Build     string `json:"build,omitempty"`
	GoVersion string `json:"go_version,omitempty"`
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
		Version:   version,
		Build:     buildVersion,
		GoVersion: fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
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

//
// end of file
//
