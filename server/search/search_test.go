package search

import (
	"nysoure/server/model"
	"os"
	"testing"

	"github.com/blevesearch/bleve"
	"gorm.io/gorm"
)

func Init() {
	err := index.Close()
	if err != nil {
		panic(err)
	}
	_ = os.RemoveAll("search_test.bleve")
	mapper := bleve.NewIndexMapping()
	index, err = bleve.New("search_test.bleve", mapper)
	if err != nil {
		panic(err)
	}
}

func TearDown() {
	err := index.Close()
	if err != nil {
		panic(err)
	}
	os.RemoveAll("search_test.bleve")
	os.Remove("test.db")
}

func TestSearchResource(t *testing.T) {
	Init()
	defer TearDown()

	resources := []model.Resource{
		// normal cases
		{Model: gorm.Model{ID: 1}, Title: "The Great Adventure", AlternativeTitles: []string{"Adventure Time", "The Big Adventure"}},
		{Model: gorm.Model{ID: 2}, Title: "Mystery of the Lost City", AlternativeTitles: []string{"Lost City Chronicles"}},
		{Model: gorm.Model{ID: 3}, Title: "Romance in Paris", AlternativeTitles: []string{"Love in Paris", "Parisian Romance"}},
		{Model: gorm.Model{ID: 4}, Title: "Sci-Fi Extravaganza", AlternativeTitles: []string{"Future World", "Sci-Fi Saga"}},
		{Model: gorm.Model{ID: 5}, Title: "Comedy Nights", AlternativeTitles: []string{"Laugh Out Loud", "Comedy Central"}},
		// With special characters
		{Model: gorm.Model{ID: 6}, Title: "Action & Adventure", AlternativeTitles: []string{"Action-Packed", "Adventure Time!"}},
		{Model: gorm.Model{ID: 7}, Title: "Horror: The Awakening", AlternativeTitles: []string{"Scary Movie", "Horror Nights"}},
		{Model: gorm.Model{ID: 8}, Title: "Drama @ Home", AlternativeTitles: []string{"Home Stories", "Dramatic Tales"}},
		{Model: gorm.Model{ID: 9}, Title: "Fantasy #1", AlternativeTitles: []string{"Fantasy World", "Magical Tales"}},
		{Model: gorm.Model{ID: 10}, Title: "Thriller ~Uncut~"},
		{Model: gorm.Model{ID: 11}, Title: "Epic ~ Saga ~"},
		{Model: gorm.Model{ID: 12}, Title: "Journey - Dawn -", AlternativeTitles: []string{"Dawn Adventures", "Journey Chronicles"}},
		{Model: gorm.Model{ID: 13}, Title: "Legends -Rise-", AlternativeTitles: []string{"Rise of Legends", "Legendary Tales"}},
		{Model: gorm.Model{ID: 14}, Title: "Chronicles: Time", AlternativeTitles: []string{"Time Chronicles", "Chronicles of Ages"}},
	}

	// Add resources to index
	for _, r := range resources {
		err := AddResourceToIndex(r)
		if err != nil {
			t.Fatalf("Failed to add resource ID %d to index: %v", r.ID, err)
		}
	}

	tests := []struct {
		query        string
		expectedIDs  []uint
		unexpectedID uint
	}{
		// Basic searches
		{query: "Adventure", expectedIDs: []uint{1, 6}},
		{query: "Mystery", expectedIDs: []uint{2}},
		{query: "Romance", expectedIDs: []uint{3}},
		{query: "Sci-Fi", expectedIDs: []uint{4}},
		{query: "Comedy", expectedIDs: []uint{5}},
		// Exact matches
		{query: "The Great Adventure", expectedIDs: []uint{1}},
		{query: "Mystery of the Lost City", expectedIDs: []uint{2}},
		{query: "Romance in Paris", expectedIDs: []uint{3}},
		{query: "Sci-Fi Extravaganza", expectedIDs: []uint{4}},
		{query: "Comedy Nights", expectedIDs: []uint{5}},
		{query: "Action & Adventure", expectedIDs: []uint{6}},
		{query: "Horror: The Awakening", expectedIDs: []uint{7}},
		{query: "Drama @ Home", expectedIDs: []uint{8}},
		{query: "Fantasy #1", expectedIDs: []uint{9}},
		{query: "Thriller ~Uncut~", expectedIDs: []uint{10}},
		{query: "Epic ~ Saga ~", expectedIDs: []uint{11}},
		{query: "Journey - Dawn -", expectedIDs: []uint{12}},
		{query: "Legends -Rise-", expectedIDs: []uint{13}},
		{query: "Chronicles: Time", expectedIDs: []uint{14}},
		// Searches with special characters
		{query: "Action & Adventure", expectedIDs: []uint{6}},
		{query: "Horror: The Awakening", expectedIDs: []uint{7}},
		{query: "Drama @ Home", expectedIDs: []uint{8}},
		{query: "Fantasy #1", expectedIDs: []uint{9}},
		{query: "Thriller ~Uncut~", expectedIDs: []uint{10}},
		{query: "Epic ~ Saga ~", expectedIDs: []uint{11}},
		{query: "Journey - Dawn -", expectedIDs: []uint{12}},
		{query: "Legends -Rise-", expectedIDs: []uint{13}},
		{query: "Chronicles: Time", expectedIDs: []uint{14}},
		// Case insensitivity
		{query: "adventure", expectedIDs: []uint{1, 6}},
		{query: "MYSTERY", expectedIDs: []uint{2}},
		{query: "rOmAnCe", expectedIDs: []uint{3}},
		// Searches using alternative titles
		{query: "Adventure Time", expectedIDs: []uint{1, 6}},
		{query: "Lost City Chronicles", expectedIDs: []uint{2}},
		{query: "Love in Paris", expectedIDs: []uint{3}},
		{query: "Future World", expectedIDs: []uint{4}},
		{query: "Laugh Out Loud", expectedIDs: []uint{5}},
		// Searches with special characters in alternative titles
		{query: "Action-Packed", expectedIDs: []uint{6}},
		{query: "Scary Movie", expectedIDs: []uint{7}},
		{query: "Home Stories", expectedIDs: []uint{8}},
		{query: "Fantasy World", expectedIDs: []uint{9}},
		{query: "Epic Tales", expectedIDs: []uint{11}},
		{query: "Dawn Adventures", expectedIDs: []uint{12}},
		{query: "Rise of Legends", expectedIDs: []uint{13}},
		{query: "Time Chronicles", expectedIDs: []uint{14}},
		// Searches with special characters in queries
		{query: "Horror:", expectedIDs: []uint{7}},
		{query: "@ Home", expectedIDs: []uint{8}},
		{query: "#1", expectedIDs: []uint{9}},
		{query: "~Uncut~", expectedIDs: []uint{10}},
		{query: "Uncut", expectedIDs: []uint{10}},
		{query: "~ Saga ~", expectedIDs: []uint{11}},
		{query: "Saga", expectedIDs: []uint{11}},
		{query: "- Dawn -", expectedIDs: []uint{12}},
		{query: "-Rise-", expectedIDs: []uint{13}},
	}

	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			resultIDs, err := SearchResource(test.query)
			if err != nil {
				t.Fatalf("Search failed for query '%s': %v", test.query, err)
			}

			// Check for expected IDs
			for _, expectedID := range test.expectedIDs {
				found := false
				for _, resultID := range resultIDs {
					if resultID == expectedID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected ID %d not found in results for query '%s'", expectedID, test.query)
				}
			}

			// Check for unexpected ID
			if test.unexpectedID != 0 {
				for _, resultID := range resultIDs {
					if resultID == test.unexpectedID {
						t.Errorf("Unexpected ID %d found in results for query '%s'", test.unexpectedID, test.query)
					}
				}
			}
		})
	}
}

func TestSearchResourceRanksByScore(t *testing.T) {
	Init()
	defer TearDown()

	resources := []model.Resource{
		{Model: gorm.Model{ID: 1}, Title: "Adventure"},
		{Model: gorm.Model{ID: 2}, Title: "The Great Adventure", AlternativeTitles: []string{"Great Adventure", "Adventure Saga"}},
		{Model: gorm.Model{ID: 3}, Title: "Unrelated Mystery"},
	}
	for _, r := range resources {
		if err := AddResourceToIndex(r); err != nil {
			t.Fatalf("Failed to add resource ID %d to index: %v", r.ID, err)
		}
	}

	resultIDs, err := SearchResource("The Great Adventure")
	if err != nil {
		t.Fatal(err)
	}
	if len(resultIDs) == 0 {
		t.Fatal("expected search hits")
	}
	if resultIDs[0] != 2 {
		t.Fatalf("expected most relevant ID 2 first, got %v", resultIDs)
	}
}

func TestSearchResourceReturnsMoreThanDefaultPage(t *testing.T) {
	Init()
	defer TearDown()

	for id := uint(1); id <= 15; id++ {
		r := model.Resource{Model: gorm.Model{ID: id}, Title: "UniqueToken Title"}
		if err := AddResourceToIndex(r); err != nil {
			t.Fatalf("Failed to add resource ID %d to index: %v", id, err)
		}
	}

	resultIDs, err := SearchResource("UniqueToken")
	if err != nil {
		t.Fatal(err)
	}
	if len(resultIDs) != 15 {
		t.Fatalf("expected 15 hits, got %d", len(resultIDs))
	}
}

func TestIsStopWord(t *testing.T) {
	Init()
	defer TearDown()

	stopWords := []string{"the", "is", "at", "which", "on", "and", "a", "an", "in", "to", "of"}
	nonStopWords := []string{"adventure", "mystery", "romance", "sci-fi", "comedy", "action", "horror"}

	for _, word := range stopWords {
		if !IsStopWord(word) {
			t.Errorf("Expected '%s' to be identified as a stop word", word)
		}
	}

	for _, word := range nonStopWords {
		if IsStopWord(word) {
			t.Errorf("Expected '%s' to not be identified as a stop word", word)
		}
	}
}
