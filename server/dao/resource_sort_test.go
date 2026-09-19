package dao

import (
	"nysoure/server/model"
	"reflect"
	"testing"
)

func TestSortResourceIDsRelevancePreservesOrder(t *testing.T) {
	ids := []uint{7, 2, 9, 2}
	got, err := SortResourceIDs(ids, model.RSortRelevance)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("SortResourceIDs(relevance) = %v, want %v", got, ids)
	}
}
