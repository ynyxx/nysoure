package model

import "testing"

func TestToViewIncludesFirstSubtitle(t *testing.T) {
	r := Resource{
		Title:             "Main Title",
		AlternativeTitles: []string{"  ", "First Subtitle", "Second"},
		User:              User{Username: "alice"},
	}
	view := r.ToView()
	if view.Title != "Main Title" {
		t.Fatalf("Title = %q, want %q", view.Title, "Main Title")
	}
	if view.Subtitle != "First Subtitle" {
		t.Fatalf("Subtitle = %q, want %q", view.Subtitle, "First Subtitle")
	}
}

func TestToViewOmitsEmptySubtitle(t *testing.T) {
	r := Resource{
		Title:             "Main Title",
		AlternativeTitles: []string{"", "   "},
		User:              User{Username: "alice"},
	}
	view := r.ToView()
	if view.Subtitle != "" {
		t.Fatalf("Subtitle = %q, want empty", view.Subtitle)
	}
}
