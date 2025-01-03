package ingestor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"owlrepo/pg"
	"sync"
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
// This holds the meta-data for an item however does not provide history
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

// Response from a given TaskId. These are batches and may contain data for
// multiple items per set
type TaskIdResponse struct {
	Payload []TaskIdMeta `json:"payload"`
}

// Contains the metadata (and underlying body) for a TaskId response
type TaskIdMeta struct {
	Screenshot struct {
		Timestamp string `json:"timestamp"`
	} `json:"screenshot"`
	Search struct {
		Item    string `json:"item"`
		Results int    `json:"results"`
	} `json:"search"`
	Body struct {
		Entries []TaskIdEntry
	} `json:"body"`
}

// Contains information for a TaskId items response; price, location, and owner
type TaskIdEntry struct {
	Id        string `json:"id"`
	StoreName string `json:"store_name"`
	Bundle    int    `json:"bundle"`
	Price     int    `json:"price"`
	Quantity  int    `json:"quantity"`
}

// Entry point to ingestion script, scrapes owlrepo.com for all recent item data
func Ingest() {
	db := pg.NewPG()
	// Step 1 - Fetch search_item_index (list of all items and meta-data)
	searchIndexUrl := "https://storage.googleapis.com/owlrepo/v1/queries/search_item_index.json"
	searchIndexResults := make([]SearchIndexResponse, 0)

	err := httpGet(searchIndexUrl, &searchIndexResults)
	panicf(err, "HTTP Get Error - search_item_index.json")
	sirLen := len(searchIndexResults)

	// Step 2 - Fetch historical data via a TaskId on an item. (TODO: This may not actually be historically complete...)
	var wg sync.WaitGroup
	sem := make(chan int, 20)
	taskIdResponses := make(chan TaskIdResponse, sirLen)
	taskIdErrors := make(chan error, sirLen*2)

	for i, sir := range searchIndexResults {
		fmt.Printf("\r Fetching slim.json from search_item_index --- [ %d / %d ]            ", i, sirLen)

		wg.Add(1)
		sem <- 1
		go ProcessSearchIndexResult(&wg, sem, taskIdErrors, taskIdResponses, db, sir)
	}

	wg.Wait()
	close(taskIdResponses)

	// TODO: Do something with these errors
	close(taskIdErrors)
	if len(taskIdErrors) > (sirLen / 10) {
		panic("Error rate from index > 10% time to debug")
	}

	taskMetaErrors := make(chan error, len(taskIdResponses))
	for tir := range taskIdResponses {
		for _, tim := range tir.Payload {
			wg.Add(1)
			sem <- 1
			go ProcessTaskIdMeta(&wg, sem, taskMetaErrors, db, tim)
		}
	}

	wg.Wait()
	close(taskMetaErrors)
	// TODO: Do something with these errors
	if len(taskMetaErrors) > (len(taskIdResponses) / 10) {
		panic("Error rate from processing TaskIdMeta > 10% time to debug")
	}
}

// ProcessTaskIdMeta: Used to store data from each items meta provided slim.json
func ProcessTaskIdMeta(wg *sync.WaitGroup, sem <-chan int, errorc chan<- error, db *pg.PG, tim TaskIdMeta) {
	defer wg.Done()
	defer func() { <-sem }()

	// Upsert items to item_hist
	err := db.Exec("insert_item_hist", tim.Search.Item, tim.Screenshot.Timestamp)
	if err != nil {
		fmt.Println(err)
		errorc <- err
	} else {
		fmt.Println(tim.Search.Item)
	}

	/*
				    *   FIXME: okay so at this point there is not a guarentee the related item (id) exists...
				*      this is kind of odd and a problem... maybe they dont have every item in the search_index?
				*      TODO: deque these into some kind of storage and process afterward...
		*               NOTE: this could also just be a string formatting issue.
	*/
}

// ProcessSearchIndexResult: Used inside main loop over searchIndexResults to dispatch multiple tasks
// 1) upsert current SearchIndexResult to database
// 2) retrieve slim.json from given items TaskID and send to chan
func ProcessSearchIndexResult(wg *sync.WaitGroup, sem <-chan int, errorc chan<- error, taskIdRespc chan<- TaskIdResponse, db *pg.PG, sir SearchIndexResponse) {
	defer wg.Done()
	defer func() { <-sem }()

	// Upsert current SearchIndexResponse to db
	err := db.Exec("upsert_item",
		sir.SearchItem,
		sir.P0,
		sir.P25,
		sir.P50,
		sir.P75,
		sir.P100,
		sir.Mean,
		sir.Std,
		sir.NOweled,
	)
	if err != nil {
		errorc <- err
		fmt.Println(err)
	}

	// Retrieve slim.json from items task id and dispatch to chan
	tir := TaskIdResponse{}
	taskIdUrl := "https://storage.googleapis.com/owlrepo/v1/uploads/" + sir.TaskId + "/slim.json"
	err = httpGet(taskIdUrl, &tir)
	if err != nil {
		errorc <- err
		return
	}
	taskIdRespc <- tir
}

// Panic wrapper for critical errors
func panicf(err error, message string) {
	if err != nil {
		panic(fmt.Sprintf("%s \n %e", message, err))
	}
}
