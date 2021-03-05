package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// a subset of Solr fields we are interested in
type solrDoc struct {
	ID                 string   `json:"id,omitempty"`
	Title              []string `json:"title_a,omitempty"`
	AuthorFacet        []string `json:"author_facet_a,omitempty"`
	PublishedDaterange []string `json:"published_daterange,omitempty"`
	AlternateID        []string `json:"alternate_id_a,omitempty"`
	RightsWrapper      []string `json:"rights_wrapper_a,omitempty"`
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

func firstElementOf(s []string) string {
	// return first element of slice, or blank string if empty
	val := ""

	if len(s) > 0 {
		val = s[0]
	}

	return val
}

func solrGetInfo(pid string) (*solrInfo, error) {
	url := config.solrURLTemplate.value
	url = strings.Replace(url, "{PID}", pid, -1)

	log.Printf("solr url: [%s]", url)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		log.Printf("NewRequest() failed: %s", reqErr.Error())
		return nil, errors.New("failed to create new solr request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		log.Printf("client.Do() failed: %s", resErr.Error())
		return nil, errors.New("failed to receive solr response")
	}

	defer res.Body.Close()

	// parse json from body

	var solr solrInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &solr); jErr != nil {
		log.Printf("Unmarshal() failed: %s", jErr.Error())
		return nil, fmt.Errorf("failed to unmarshal solr response: [%s]", buf)
	}

	log.Printf("status                 : [%d]", solr.ResponseHeader.Status)
	log.Printf("numFound               : [%d]", solr.Response.NumFound)
	log.Printf("start                  : [%d]", solr.Response.Start)
	log.Printf("len(docs)              : [%d]", len(solr.Response.Docs))

	if solr.Response.NumFound == 0 || len(solr.Response.Docs) == 0 {
		log.Printf("No Solr record found: numFound = %d, len(docs) = %d", solr.Response.NumFound, len(solr.Response.Docs))
		return nil, errors.New("no Solr record found")
	}

	// expecting just one record

	doc := solr.Response.Docs[0]

	log.Printf("id                  : [%s]", doc.ID)
	log.Printf("title_a             : [%s]", firstElementOf(doc.Title))
	log.Printf("author_facet_a      : [%s]", firstElementOf(doc.AuthorFacet))
	log.Printf("published_daterange : [%s]", firstElementOf(doc.PublishedDaterange))
	log.Printf("alternate_id_a      : [%s]", firstElementOf(doc.AlternateID))
	log.Printf("rights_wrapper_a    : [%s]", firstElementOf(doc.RightsWrapper))

	return &solr, nil
}
