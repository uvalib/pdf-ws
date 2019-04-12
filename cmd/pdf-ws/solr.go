package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// a subset of Solr fields we are interested in
type solrDoc struct {
	Id                   string   `json:"id,omitempty"`
	TitleDisplay         []string `json:"title_display,omitempty"`
	AuthorFacet          []string `json:"author_facet,omitempty"`
	PublishedDateDisplay []string `json:"published_date_display,omitempty"`
	AlternateIdFacet     []string `json:"alternate_id_facet,omitempty"`
	RightsWrapperDisplay []string `json:"rights_wrapper_display,omitempty"`
}

type solrResponse struct {
	NumFound int       `json:"numFound,omitempty"`
	Start    int       `json:"start,omitempty"`
	Docs     []solrDoc `json:"docs,omitempty"`
}

type solrResponseHeader struct {
	Status int `json:"status,omitempty"`
}

// top-level json response
type solrInfo struct {
	ResponseHeader solrResponseHeader `json:"responseHeader,omitempty"`
	Response       solrResponse       `json:"response,omitempty"`
}

func solrGetInfo(pid string) (*solrInfo, error) {
	url := config.solrUrlTemplate.value
	url = strings.Replace(url, "{PID}", pid, -1)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		logger.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("Failed to create new solr request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		logger.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("Failed to receive solr response")
	}

	defer res.Body.Close()

	// parse json from body

	var solr solrInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &solr); jErr != nil {
		logger.Printf("Unmarshal() failed: %s", jErr.Error())
		return nil, errors.New(fmt.Sprintf("Failed to unmarshal solr response: [%s]", buf))
	}

	logger.Printf("status                 : [%d]", solr.ResponseHeader.Status)
	logger.Printf("numFound               : [%d]", solr.Response.NumFound)
	logger.Printf("start                  : [%d]", solr.Response.Start)
	logger.Printf("len(docs)              : [%d]", len(solr.Response.Docs))

	if solr.Response.NumFound == 0 || len(solr.Response.Docs) == 0 {
		logger.Printf("No Solr record found: numFound = %d, len(docs) = %d", solr.Response.NumFound, len(solr.Response.Docs))
		return nil, errors.New("No Solr record found")
	}

	// expecting just one record
	logger.Printf("id                     : [%s]", solr.Response.Docs[0].Id)
	logger.Printf("title_display          : [%s]", solr.Response.Docs[0].TitleDisplay[0])
	logger.Printf("author_facet           : [%s]", solr.Response.Docs[0].AuthorFacet[0])
	logger.Printf("published_date_display : [%s]", solr.Response.Docs[0].PublishedDateDisplay[0])
	logger.Printf("alternate_id_facet     : [%s]", solr.Response.Docs[0].AlternateIdFacet[0])
	logger.Printf("rights_wrapper_display : [%s]", solr.Response.Docs[0].RightsWrapperDisplay[0])

	return &solr, nil
}
