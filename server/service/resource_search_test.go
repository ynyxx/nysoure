package service

import (
	"nysoure/server/model"
	"strings"
	"testing"
	"time"
)

func TestSearchResourcesValidation(t *testing.T) {
	t.Run("requires at least one condition", func(t *testing.T) {
		_, _, err := SearchResources(ResourceSearchParams{})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "At least one search condition is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects inverted date range", func(t *testing.T) {
		from := time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC)
		to := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		_, _, err := SearchResources(ResourceSearchParams{
			ReleaseFrom: &from,
			ReleaseTo:   &to,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "Invalid date range") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects too many tags", func(t *testing.T) {
		tags := make([]string, maxSearchTagCount+1)
		for i := range tags {
			tags[i] = "tag-" + strings.Repeat("x", i+1)
		}
		_, _, err := SearchResources(ResourceSearchParams{Tags: tags})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "Too many tags") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects overly long keyword", func(t *testing.T) {
		_, _, err := SearchResources(ResourceSearchParams{
			Keyword: strings.Repeat("a", maxSearchQueryLength+1),
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "too long") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("sort does not replace required conditions", func(t *testing.T) {
		sort := model.RSortTimeDesc
		_, _, err := SearchResources(ResourceSearchParams{Sort: &sort})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "At least one search condition is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
