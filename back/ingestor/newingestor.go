package ingestor

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// httpGet: given a url to fetch (assuming JSON response) and a *struct
// decode response into *struct
func httpGet(url string, target any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// Struct representing a single entry from owlrepo's search_item_index.json
type SearchIndexResponse struct {
	TaskId              string `json:"task_id"`
	SearchItemTimestamp string `json:"search_item_timestamp"`
	SearchItem          string `json:"search_item"`
	SearchResults       int    `json:"search_results"`
	P0                  int    `json:"p0"`
	P25                 int    `json:"p25"`
	P50                 int    `json:"p50"`
	P75                 int    `json:"p75"`
	P100                int    `json:"p100"`
	Mean                int    `json:"mean"`
	Std                 int    `json:"std"`
	NOweled             int    `json:"n_owled"`
}

// Entry point to ingestion script, scrapes owlrepo.com for all recent item data
func Ingest() {
	searchIndexUrl := "https://storage.googleapis.com/owlrepo/v1/queries/search_item_index.json"
	searchIndexResults := make([]*SearchIndexResponse, 0)

	err := httpGet(searchIndexUrl, &searchIndexResults)
	panicf(err, "HTTP Get Error - search_item_index.json")
}

// Panic wrapper for critical errors
func panicf(err error, message string) {
	if err != nil {
		panic(fmt.Sprintf("%s \n %e", message, err))
	}
}
