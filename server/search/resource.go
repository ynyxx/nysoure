package search

import (
	"errors"
	"fmt"
	"log/slog"
	"nysoure/server/dao"
	"nysoure/server/model"
	"nysoure/server/utils"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
)

var (
	index bleve.Index
	mu    = sync.RWMutex{}
)

const (
	minSearchHit  = 0.2
	maxSearchHits = 10000
)

type ResourceParams struct {
	Id         uint
	Title      string
	Subtitles  []string
	Time       time.Time
	Characters []ResourceCharacter
}

type ResourceCharacter struct {
	Name  string
	Alias []string
	CV    string
}

func AddResourceToIndex(r model.Resource) error {
	mu.RLock()
	defer mu.RUnlock()
	cs := make([]ResourceCharacter, 0, len(r.Characters))
	for _, c := range r.Characters {
		cs = append(cs, ResourceCharacter{
			Name:  c.Name,
			Alias: c.Alias,
			CV:    c.CV,
		})
	}
	return index.Index(fmt.Sprintf("%d", r.ID), ResourceParams{
		Id:         r.ID,
		Title:      r.Title,
		Subtitles:  r.AlternativeTitles,
		Time:       r.CreatedAt,
		Characters: cs,
	})
}

func RemoveResourceFromIndex(id uint) error {
	return index.Delete(fmt.Sprintf("%d", id))
}

func createIndex() error {
	for !dao.IsReady() {
		time.Sleep(1 * time.Second)
	}
	page := 1
	total := 1
	current := 0
	for page <= total {
		res, totalPages, err := dao.GetResourceList(page, 100, model.RSortTimeAsc)
		if err != nil {
			return err
		}
		for _, r := range res {
			r, err := dao.GetResourceByID(r.ID)
			if err != nil {
				return err
			}
			err = AddResourceToIndex(r)
			if err != nil {
				return err
			}
			current++
			if current%20 == 0 {
				slog.Info("Rebuilding search index", "current", current, "total", totalPages*100)
			}
		}
		page++
		total = totalPages
	}
	return nil
}

func init() {
	indexPath := utils.GetStoragePath() + "/resource_index.bleve"

	var err error
	index, err = bleve.Open(indexPath)
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(indexPath, mapping)
		if err != nil {
			panic("Failed to create search index: " + err.Error())
		}
		go func() {
			err := createIndex()
			if err != nil {
				panic("Failed to create search index: " + err.Error())
			}
		}()
	} else if err != nil {
		panic("Failed to open search index: " + err.Error())
	}
}

func SearchResource(keyword string) ([]uint, error) {
	mu.RLock()
	defer mu.RUnlock()
	query := bleve.NewMatchQuery(keyword)
	searchRequest := bleve.NewSearchRequestOptions(query, maxSearchHits, 0, false)
	searchResults, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	results := make([]uint, 0)
	for _, hit := range searchResults.Hits {
		id, err := strconv.ParseUint(hit.ID, 10, 32)
		if err != nil {
			continue
		}
		if hit.Score < minSearchHit {
			continue
		}
		results = append(results, uint(id))
	}

	return results, nil
}

func IsStopWord(word string) bool {
	mapping := bleve.NewIndexMapping()
	analyzerName := mapping.DefaultAnalyzer
	analyzer := mapping.AnalyzerNamed(analyzerName)
	if analyzer == nil {
		return false
	}
	tokens := analyzer.Analyze([]byte(word))
	return len(tokens) == 0
}

func RebuildSearchIndex() error {
	mu.Lock()
	defer mu.Unlock()
	err := index.Close()
	if err != nil {
		return fmt.Errorf("failed to close search index: %w", err)
	}
	indexPath := utils.GetStoragePath() + "/resource_index.bleve"
	err = os.RemoveAll(indexPath)
	if err != nil {
		return fmt.Errorf("failed to remove search index: %w", err)
	}
	mapping := bleve.NewIndexMapping()
	index, err = bleve.New(indexPath, mapping)
	if err != nil {
		return fmt.Errorf("failed to create search index: %w", err)
	}
	go createIndex()
	return nil
}
