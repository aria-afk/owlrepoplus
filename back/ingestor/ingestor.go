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

type StoreItemEntry struct {
	StoreName string
	Bundle    int
	Price     int
	Quantity  int
}

// For now we can do de-duplication in a mem cache
// Later on if wanted can just implement this via queries
type Cache struct {
	Store map[string] /*item name*/ map[string] /*timestamp*/ map[string] /*id*/ StoreItemEntry
	sync.Mutex
}

func NewCache() Cache {
	return Cache{
		Store: make(map[string]map[string]map[string]StoreItemEntry),
	}
}

func (c *Cache) write(wg *sync.WaitGroup, itemName string, timestamp string, id string, payload StoreItemEntry) {
	c.Lock()
	defer c.Unlock()
	defer wg.Done()

	// Init new map item if doesnt exist
	_, exists := c.Store[itemName]
	if !exists {
		c.Store[itemName] = make(map[string]map[string]StoreItemEntry)
	}

	_, timestampExists := c.Store[itemName][timestamp]
	if !timestampExists {
		c.Store[itemName][timestamp] = make(map[string]StoreItemEntry)
	}

	c.Store[itemName][timestamp][id] = payload
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
	c := NewCache()
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
		if i > 1 {
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
		for i, p := range e.Payload {
			if i > 1 {
				break
			}
			for _, b := range p.Body.Entries {
				wg.Add(1)
				sem <- 1
				go func() {
					defer func() { <-sem }()
					payload := StoreItemEntry{
						StoreName: b.StoreName,
						Bundle:    b.Bundle,
						Price:     b.Price,
						Quantity:  b.Quantity,
					}
					c.write(&wg, p.Search.Item, p.Screenshot.Timestamp, b.Id, payload)
				}()
			}
		}
	}

	wg.Wait()

	fmt.Print(c.Store)

	return nil
}
