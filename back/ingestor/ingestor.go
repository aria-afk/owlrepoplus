// Ingestor: takes existing data from owlrepo and formats and stores
// it into our db
package ingestor

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"owlrepo/pg"
	"sync"

	"github.com/montanaflynn/stats"
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
	Id        string
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

type FormattedCachePayload struct {
	Timestamp string
	MinPrice  int
	P25Price  int
	P50Price  int
	P75Price  int
	MaxPrice  int
	Meta      []StoreItemEntry
}

func (c *Cache) get(key string) FormattedCachePayload {
	m := c.Store[key]

	result := FormattedCachePayload{Meta: make([]StoreItemEntry, 0)}
	minp := math.MaxInt
	maxp := math.MinInt
	prices := make([]int, 0)

	for timestamp, idMap := range m {
		if result.Timestamp == "" {
			result.Timestamp = timestamp
		}
		for _, entry := range idMap {
			result.Meta = append(result.Meta, entry)

			if entry.Price > maxp {
				maxp = entry.Price
			}
			if entry.Price < minp {
				minp = entry.Price
			}

			prices = append(prices, entry.Price)
		}
	}
	p25, _ := stats.PercentileNearestRank(stats.LoadRawData(prices), 25)
	p50, _ := stats.PercentileNearestRank(stats.LoadRawData(prices), 50)
	p75, _ := stats.PercentileNearestRank(stats.LoadRawData(prices), 50)

	result.P25Price = int(p25)
	result.P50Price = int(p50)
	result.P75Price = int(p75)
	result.MinPrice = minp
	result.MaxPrice = maxp

	return result
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
	db := pg.NewPG()

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

	workCount := len(searchEntries)
	for i, entry := range searchEntries {
		fmt.Printf("\r %d / %d items to do                                                       ", i, workCount)
		url := "https://storage.googleapis.com/owlrepo/v1/uploads/" + entry.TaskId + "/slim.json"
		sem <- 1
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			var ier IndexEntryReponse
			err := GetAndDecode(url, &ier)
			if err != nil {
				// TODO: Error handling maybe if we have > X errors we panic
				fmt.Print(err)
			}

			indexEntries <- ier
		}()
	}

	wg.Wait()
	close(indexEntries)

	for e := range indexEntries {
		for _, p := range e.Payload {
			for _, b := range p.Body.Entries {
				wg.Add(1)
				sem <- 1
				go func() {
					defer func() { <-sem }()
					payload := StoreItemEntry{
						Id:        b.Id,
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

	fmt.Println("hi there fren")

	for item := range c.Store {
		paylod := c.get(item)

		_, err := db.Conn.Exec(`INSERT INTO items (id) VALUES ($1) ON CONFLICT DO NOTHING`, item)
		if err != nil {
			fmt.Println(err)
		}
		_, err = db.Conn.Exec(`INSERT INTO item_meta (item_id, time, min_price, p25_price, p50_price, p75_price, max_price) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			item,
			paylod.Timestamp,
			paylod.MinPrice,
			paylod.P25Price,
			paylod.P50Price,
			paylod.P75Price,
			paylod.MaxPrice,
		)
		if err != nil {
			fmt.Println(err)
		}

		for _, se := range paylod.Meta {
			_, err = db.Conn.Exec(`INSERT INTO item_meta_recents (item_id, time, owner, store_name, bundle, price, quantity) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				item,
				paylod.Timestamp,
				se.Id,
				se.StoreName,
				se.Bundle,
				se.Price,
				se.Quantity,
			)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	return nil
}
