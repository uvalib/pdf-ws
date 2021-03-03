package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const version = "2.1.0"

var logger *log.Logger
var client *http.Client

/**
 * Main entry point for the web service
 */
func main() {
	logger = log.New(os.Stdout, "", log.LstdFlags)

	// Load cfg
	logger.Printf("===> pdf-ws staring up <===")
	logger.Printf("Load configuration...")
	getConfigValues()

	// load version details
	initVersion()

	// initialize http client
	client = &http.Client{Timeout: 10 * time.Second}

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
	router.GET("/version", versionHandler)
	router.GET("/healthcheck", healthCheckHandler)

	router.GET("/pdf/:pid", generateHandler)
	router.GET("/pdf/:pid/status", statusHandler)
	router.GET("/pdf/:pid/download", downloadHandler)
	router.GET("/pdf/:pid/delete", deleteHandler)

	portStr := fmt.Sprintf(":%s", config.listenPort.value)
	logger.Printf("Start service on %s", portStr)

	log.Fatal(router.Run(portStr))
}

// Handle a request for /
func rootHandler(c *gin.Context) {
	c.String(http.StatusOK, fmt.Sprintf("PDF service version %s", version))
}

// Handle a request for /version
func versionHandler(c *gin.Context) {
	output, jsonErr := json.Marshal(versionDetails)
	if jsonErr != nil {
		logger.Printf("Failed to serialize output: [%s]", jsonErr.Error())
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
		logger.Printf("Failed to serialize output: [%s]", jsonErr.Error())
		c.String(http.StatusInternalServerError, "")
		return
	}

	c.String(http.StatusOK, string(output))
}

//
// end of file
//
