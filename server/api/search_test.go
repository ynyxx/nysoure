package api

import (
	"reflect"
	"testing"
	"time"
)

func TestParseSearchDate(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := parseSearchDate("  ")
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("valid", func(t *testing.T) {
		got, err := parseSearchDate("2024-06-01")
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("expected date")
		}
		want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseSearchDate("2024/06/01")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestParseSearchTags(t *testing.T) {
	got := parseSearchTags(" RPG, ADV, RPG,  ")
	want := []string{"RPG", "ADV"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSearchTags() = %#v, want %#v", got, want)
	}
	if got := parseSearchTags(""); got != nil && len(got) != 0 {
		t.Fatalf("expected empty tags, got %#v", got)
	}
}
