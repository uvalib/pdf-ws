package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const version = "2.3.0"

var client *http.Client
var randomSource *rand.Rand

/**
 * Main entry point for the web service
 */
func main() {
	log.Printf("===> pdf-ws starting up <===")
	log.Printf("Load configuration...")
	getConfigValues()

	// load version details
	initVersion()

	// initialize http client and random source
	client = &http.Client{Timeout: 10 * time.Second}
	randomSource = rand.New(rand.NewSource(time.Now().UnixNano()))

	// Set routes and start server
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	router := gin.Default()

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowCredentials = true
	corsCfg.AddAllowHeaders("Authorization")
	router.Use(cors.New(corsCfg))

	// Set routes and start server
	router.GET("/", rootHandler)
	router.GET("/robots.txt", robotsHandler)
	router.GET("/favicon.ico", ignoreHandler)
	router.GET("/version", versionHandler)
	router.GET("/healthcheck", healthCheckHandler)

	router.GET("/pdf/:pid", generateHandler)
	router.GET("/pdf/:pid/status", statusHandler)
	router.GET("/pdf/:pid/download", downloadHandler)
	router.GET("/pdf/:pid/delete", deleteHandler)

	portStr := fmt.Sprintf(":%s", config.listenPort.value)
	log.Printf("Start service on %s", portStr)

	log.Fatal(router.Run(portStr))
}

// Handle a request for /
func rootHandler(c *gin.Context) {
	c.String(http.StatusOK, fmt.Sprintf("PDF service version %s", version))
}

// Handle a request for /robots.txt
func robotsHandler(c *gin.Context) {
	c.String(http.StatusOK, "User-agent: *\nDisallow: /\n")
}

// Handle a request for /favicon.ico
func ignoreHandler(c *gin.Context) {
}

// Handle a request for /version
func versionHandler(c *gin.Context) {
	output, jsonErr := json.Marshal(versionDetails)
	if jsonErr != nil {
		log.Printf("ERROR: failed to serialize output: [%s]", jsonErr.Error())
		c.String(http.StatusInternalServerError, "")
		return
	}

	c.String(http.StatusOK, string(output))
}

// Handle a request for /healthcheck
func healthCheckHandler(c *gin.Context) {
	health := healthcheckDetails{healthCheckStatus{Healthy: true, Message: "Not implemented"}}

	output, jsonErr := json.Marshal(health)
	if jsonErr != nil {
		log.Printf("ERROR: failed to serialize output: [%s]", jsonErr.Error())
		c.String(http.StatusInternalServerError, "")
		return
	}

	c.String(http.StatusOK, string(output))
}

//
// end of file
//
