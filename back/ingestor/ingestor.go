// Ingestor: takes existing data from owlrepo and formats and stores
// it into our db
package ingestor

import (
	"encoding/json"
	"fmt"
	_ "fmt"
	"net/http"
	"sync"
)

// A singular entry representing the JSON format from owlrepo.com/items
type SearchIndexEntry struct {
	TaskId string `json:"task_id"` // this is what we use to get the slim.json
}

// Represents the return value from a given TaskId
type IndexEntryReponse struct {
	Payload []IndexEntryPayload `json:"payload"`
}

type IndexEntryPayload struct {
	Screenshot struct {
		Timestamp string `json:"timestamp"`
	} `json:"screenshot"`
	Search struct {
		Item    string `json:"item"`
		Results int    `json:"results"`
	} `json:"search"`
	// TODO: see if we need to care about the paginator
	Body struct {
		Entries []struct {
			Id        string `json:"id"`
			StoreName string `json:"store_name"`
			Bundle    int    `json:"bundle"`
			Price     int    `json:"price"`
			Quantity  int    `json:"quantity"`
		} `json:"entries"`
	} `json:"body"`
}

func GetAndDecode(url string, target any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func IngestFromOwlRepo() error {
	// This is the main entry point to thier search "index" (loaded on owlrepo.com/items)
	baseUrl := "https://storage.googleapis.com/owlrepo/v1/queries/search_item_listing.json"
	searchEntries := make([]SearchIndexEntry, 0)

	err := GetAndDecode(baseUrl, &searchEntries)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	sem := make(chan int, 10)

	entryCount := len(searchEntries)
	indexEntries := make(chan IndexEntryReponse, entryCount)

	for i, entry := range searchEntries {
		// TESTING: REMOVE ME
		if i > 10 {
			break
		}
		url := "https://storage.googleapis.com/owlrepo/v1/uploads/" + entry.TaskId + "/slim.json"
		sem <- 1
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			var ier IndexEntryReponse
			err := GetAndDecode(url, &ier)
			if err != nil {
				// TODO: Error handling
				fmt.Print(err)
			}

			indexEntries <- ier
		}()
	}

	wg.Wait()
	close(indexEntries)

	for e := range indexEntries {
		fmt.Print(e)
	}

	return nil
}
