package service

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractTemplateParams(t *testing.T) {
	t.Run("no params", func(t *testing.T) {
		got, err := extractTemplateParams("tag1,tag2,tag3")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("got %#v, want empty", got)
		}
	})

	t.Run("multiple params keep first-seen order", func(t *testing.T) {
		got, err := extractTemplateParams("tag1,released-at-{released_time},by-{author},tag4")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"released_time", "author"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("duplicate params", func(t *testing.T) {
		got, err := extractTemplateParams("{year},y-{year}")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"year"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("unmatched close", func(t *testing.T) {
		if _, err := extractTemplateParams("tag}"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unmatched open", func(t *testing.T) {
		if _, err := extractTemplateParams("tag-{foo"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nested braces", func(t *testing.T) {
		if _, err := extractTemplateParams("{foo{bar}}"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty param name", func(t *testing.T) {
		if _, err := extractTemplateParams("tag-{}"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("param name with space", func(t *testing.T) {
		if _, err := extractTemplateParams("{released time}"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unicode param name", func(t *testing.T) {
		got, err := extractTemplateParams("发售-{发售时间}")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"发售时间"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
}

func TestValidateTagTemplate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		params, err := ValidateTagTemplate("release", "tag1,tag2,released-at-{released_time},tag4")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"released_time"}
		if !reflect.DeepEqual(params, want) {
			t.Fatalf("got %#v, want %#v", params, want)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		if _, err := ValidateTagTemplate("  ", "tag1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty content", func(t *testing.T) {
		if _, err := ValidateTagTemplate("name", "  "); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("only commas", func(t *testing.T) {
		if _, err := ValidateTagTemplate("name", ",,,"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestExpandTagTemplate(t *testing.T) {
	t.Run("no params", func(t *testing.T) {
		got, err := ExpandTagTemplate("tag1, tag2,tag1", nil)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"tag1", "tag2"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("substitute params", func(t *testing.T) {
		got, err := ExpandTagTemplate("tag1,tag2,released-at-{released_time},tag4", map[string]string{
			"released_time": "2024",
		})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"tag1", "tag2", "released-at-2024", "tag4"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("missing param", func(t *testing.T) {
		if _, err := ExpandTagTemplate("x-{foo}", map[string]string{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty param value", func(t *testing.T) {
		if _, err := ExpandTagTemplate("x-{foo}", map[string]string{"foo": "  "}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("comma in param value", func(t *testing.T) {
		if _, err := ExpandTagTemplate("x-{foo}", map[string]string{"foo": "a,b"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("tag too long", func(t *testing.T) {
		long := strings.Repeat("a", maxTagLength+1)
		if _, err := ExpandTagTemplate(long, nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("percent in tag", func(t *testing.T) {
		if _, err := ExpandTagTemplate("100%", nil); err == nil {
			t.Fatal("expected error")
		}
	})
}
