# PDF Web Service

This is a web service to generate a PDF from metadata records.
It supports the following endpoints:

* / : returns version information
* /healthcheck : returns a JSON object with details about the health of the service
* /pdf/[PID] : downloads a PDF for the given PID, generating one if necessary
* /pdf/[PID]/status : displays the PDF generation status of the given PID (e.g. nonexistent, progress percentage, failed, complete)
* /pdf/[PID]/download : downloads a PDF for the given PID (does not generate one if it does not exist)
* /pdf/[PID]/delete : removes cached PDF (can be used to reclaim space, or to support regeneration of broken PDFs)

### System Requirements

* GO version 1.9.2 or greater
* DEP (https://golang.github.io/dep/) version 0.4.1 or greater
