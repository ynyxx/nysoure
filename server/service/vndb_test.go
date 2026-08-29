package service

import (
	"reflect"
	"testing"

	govndb "git.nyne.dev/o/go_vndb"
)

func TestCharactersFromVndbAllowsMissingCharacterImage(t *testing.T) {
	role := "main"
	vn := &govndb.VN{
		ID: "v123",
		VoiceActors: []govndb.VoiceActor{
			{
				Character: &govndb.Character{
					ID:   "c1",
					Name: "Heroine",
					VNs: []govndb.CharacterVN{
						{
							VN:   govndb.VN{ID: "v123"},
							Role: &role,
						},
					},
				},
				Staff: &govndb.Staff{Name: "CV Name"},
			},
		},
	}

	characters, err := charactersFromVndb(vn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(characters) != 1 {
		t.Fatalf("expected 1 character, got %d", len(characters))
	}
	if characters[0].Name != "Heroine" {
		t.Fatalf("expected character name to be preserved, got %q", characters[0].Name)
	}
	if characters[0].CV != "CVName" {
		t.Fatalf("expected CV name fallback to work, got %q", characters[0].CV)
	}
	if characters[0].Image != 0 {
		t.Fatalf("expected missing image to keep default image ID, got %d", characters[0].Image)
	}
	if characters[0].Role != role {
		t.Fatalf("expected role %q, got %q", role, characters[0].Role)
	}
}

func TestYearTagNameFromReleased(t *testing.T) {
	tests := []struct {
		released string
		want     string
	}{
		{"2024-12-26", "2024年"},
		{"2024-12", "2024年"},
		{"2024", "2024年"},
		{" TBA ", ""},
		{"unknown", ""},
		{"", ""},
		{"99", ""},
	}
	for _, tt := range tests {
		if got := yearTagNameFromReleased(tt.released); got != tt.want {
			t.Errorf("yearTagNameFromReleased(%q) = %q, want %q", tt.released, got, tt.want)
		}
	}
}

func TestSitePlatformsFromVNDBPlatforms(t *testing.T) {
	found := make(map[string]struct{})
	collectSitePlatforms(found, []string{"ps2", "win", "and", "ios", "web", "lin"})
	got := sitePlatformsInOrder(found)
	want := []string{"PC", "Android", "iOS"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("site platforms = %#v, want %#v", got, want)
	}

	found = make(map[string]struct{})
	collectSitePlatforms(found, []string{"swi", "psv"})
	if got := sitePlatformsInOrder(found); len(got) != 0 {
		t.Fatalf("unexpected platforms %v", got)
	}
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrBool(v bool) *bool          { return &v }

func TestGameTypeTagNamesFromVNTags(t *testing.T) {
	tags := []govndb.VNTag{
		{ID: "g32", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(2.8)},
		{ID: "g43", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(2.5)}, // also ADV
		{ID: "g35", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(2.1)},
		{ID: "g34", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(1.2)}, // below threshold
		{ID: "g2038", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(2.4), Lie: ptrBool(true)},
		{ID: "g32", Category: govndb.TagCategoryContent, Rating: ptrFloat64(3.0)},    // wrong category
		{ID: "g999", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(3.0)}, // unmapped
	}
	got := gameTypeTagNamesFromVNTags(tags)
	want := []string{"ADV", "RPG"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gameTypeTagNamesFromVNTags() = %#v, want %#v", got, want)
	}
}

func TestParsePrefillSections(t *testing.T) {
	all := ParsePrefillSections("")
	if !all.Basic || !all.Article || !all.Images || !all.Characters {
		t.Fatalf("empty sections should enable core fields, got %+v", all)
	}
	if !all.TagsBasic || !all.TagsStaff || !all.TagsContent {
		t.Fatalf("empty sections should enable all tag groups, got %+v", all)
	}

	legacy := ParsePrefillSections("tags")
	if !legacy.TagsBasic || !legacy.TagsStaff || !legacy.TagsContent {
		t.Fatalf("tags should enable all tag groups, got %+v", legacy)
	}
	if legacy.Basic || legacy.Article || legacy.Images || legacy.Characters {
		t.Fatalf("tags should not enable non-tag sections, got %+v", legacy)
	}

	partial := ParsePrefillSections("tags_basic,tags_content")
	if !partial.TagsBasic || partial.TagsStaff || !partial.TagsContent {
		t.Fatalf("partial tag sections = %+v", partial)
	}
	if partial.wantsAnyTags() != true {
		t.Fatal("expected wantsAnyTags for tags_basic,tags_content")
	}

	staffOnly := ParsePrefillSections("tags_staff")
	if staffOnly.TagsBasic || !staffOnly.TagsStaff || staffOnly.TagsContent {
		t.Fatalf("tags_staff = %+v", staffOnly)
	}
}

func TestContentTagsFromVNIgnoresTechAndLowRating(t *testing.T) {
	tags := []govndb.VNTag{
		{ID: "g10", Name: "Nakige", Category: govndb.TagCategoryContent, Rating: ptrFloat64(2.5)},
		{ID: "g32", Name: "ADV", Category: govndb.TagCategoryTechnical, Rating: ptrFloat64(2.8)},
		{ID: "g11", Name: "Weak", Category: govndb.TagCategoryContent, Rating: ptrFloat64(1.5)},
	}
	got := contentTagsFromVN(tags)
	if len(got) != 1 || got[0].ID != "g10" {
		t.Fatalf("contentTagsFromVN() = %#v, want only g10", got)
	}
}
