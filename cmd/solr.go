package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// a subset of solr fields we are interested in
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

func (c *clientContext) solrGetInfo() error {
	url := config.solrURLTemplate.value
	url = strings.Replace(url, "{PID}", c.req.pid, -1)

	c.info("solr url: [%s]", url)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		c.err("NewRequest() failed: %s", reqErr.Error())
		return errors.New("failed to create new solr request")
	}

	res, resErr := client.Do(req)
	if resErr != nil {
		c.err("client.Do() failed: %s", resErr.Error())
		return errors.New("failed to receive solr response")
	}

	defer res.Body.Close()

	// parse json from body

	var solr solrInfo

	buf, _ := ioutil.ReadAll(res.Body)
	if jErr := json.Unmarshal(buf, &solr); jErr != nil {
		c.err("Unmarshal() failed: %s", jErr.Error())
		return fmt.Errorf("failed to unmarshal solr response: [%s]", buf)
	}

	c.info("status                 : [%d]", solr.ResponseHeader.Status)
	c.info("numFound               : [%d]", solr.Response.NumFound)
	c.info("start                  : [%d]", solr.Response.Start)
	c.info("len(docs)              : [%d]", len(solr.Response.Docs))

	if solr.Response.NumFound == 0 || len(solr.Response.Docs) == 0 {
		c.warn("no solr record found: numFound = %d, len(docs) = %d", solr.Response.NumFound, len(solr.Response.Docs))
		return errors.New("no solr record found")
	}

	// expecting just one record

	doc := solr.Response.Docs[0]

	c.info("id                  : [%s]", doc.ID)
	c.info("title_a             : [%s]", firstElementOf(doc.Title))
	c.info("author_facet_a      : [%s]", firstElementOf(doc.AuthorFacet))
	c.info("published_daterange : [%s]", firstElementOf(doc.PublishedDaterange))
	c.info("alternate_id_a      : [%s]", firstElementOf(doc.AlternateID))
	c.info("rights_wrapper_a    : [%s]", firstElementOf(doc.RightsWrapper))

	c.pdf.solr = &solr

	return nil
}
