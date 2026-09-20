package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetMoyuPatchByVNDB(t *testing.T) {
	moyuCacheEnabled = false
	t.Setenv("NEXTMOE_API_KEY", "nmk_test_dummy")

	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RequestURI())
		if got := r.Header.Get("Authorization"); got != "Bearer nmk_test_dummy" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/v2/moyu/patches/") && strings.HasSuffix(r.URL.Path, "/resources"):
			cursor := r.URL.Query().Get("cursor")
			if cursor == "" {
				io.WriteString(w, `{
					"object":"list",
					"items":[{"object":"resource","id":"r1","patch_id":"p1","name":"patch 1","storage":"s3","size":"1 MB","hash":"","model_name":"","localization_group_name":"","note":"note","type":["fix"],"language":["zh-Hans"],"platform":["windows"],"download_count":3,"like_count":1,"web_url":"https://www.moyu.moe/resource/r1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","publisher":{"object":"publisher","id":"u1","name":"alice","avatar_url":"https://example.com/a.png"}}],
					"next_cursor":"cur_2"
				}`)
				return
			}
			io.WriteString(w, `{
				"object":"list",
				"items":[{"object":"resource","id":"r2","patch_id":"p1","name":"patch 2","storage":"user","size":"2 MB","hash":"","model_name":"gpt","localization_group_name":"","note":"","type":["ai"],"language":["ja"],"platform":["android"],"download_count":1,"like_count":0,"web_url":"https://www.moyu.moe/resource/r2","created_at":"2024-01-03T00:00:00Z","updated_at":"2024-01-03T00:00:00Z"}],
				"next_cursor":null
			}`)
		case r.URL.Path == "/v2/moyu/patches":
			if r.URL.Query().Get("refs") != "vndb:v19658" {
				t.Errorf("refs = %q", r.URL.Query().Get("refs"))
			}
			if r.URL.Query().Get("nsfw") != "true" {
				t.Errorf("nsfw = %q", r.URL.Query().Get("nsfw"))
			}
			io.WriteString(w, `{
				"object":"list",
				"items":[{"object":"patch","id":"p1","vndb_id":"v19658","catalog_work_id":null,"content_limit":"nsfw","release_date":"2016-11-25","type":["fix"],"language":["zh-Hans"],"platform":["windows"],"resource_count":2,"download_count":10,"view_count":20,"favorite_count":1,"comment_count":0,"web_url":"https://www.moyu.moe/patch/p1","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T00:00:00Z","resource_updated_at":"2024-01-03T00:00:00Z"}],
				"missing":[]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	oldBase := nextmoeBaseURL
	oldClient := nextmoeHTTPClient
	nextmoeBaseURL = srv.URL
	nextmoeHTTPClient = srv.Client()
	t.Cleanup(func() {
		nextmoeBaseURL = oldBase
		nextmoeHTTPClient = oldClient
	})

	patch, err := GetMoyuPatchByVNDB("v19658")
	if err != nil {
		t.Fatal(err)
	}
	if patch.ID != "p1" || patch.VndbID != "v19658" {
		t.Fatalf("patch = %+v", patch)
	}
	if len(patch.Resources) != 2 {
		t.Fatalf("resources = %#v", patch.Resources)
	}
	if patch.Resources[0].Publisher == nil || patch.Resources[0].Publisher.Name != "alice" {
		t.Fatalf("publisher = %#v", patch.Resources[0].Publisher)
	}
	if patch.Resources[1].ModelName != "gpt" {
		t.Fatalf("second resource = %#v", patch.Resources[1])
	}

	joined := strings.Join(seen, " ")
	if !strings.Contains(joined, "/v2/moyu/patches?") {
		t.Fatalf("lookup not called: %v", seen)
	}
	if !strings.Contains(joined, "/v2/moyu/patches/p1/resources") {
		t.Fatalf("resources not called: %v", seen)
	}
}

func TestGetMoyuPatchByVNDBMissing(t *testing.T) {
	moyuCacheEnabled = false
	t.Setenv("NEXTMOE_API_KEY", "nmk_test_dummy")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"object":"list","items":[],"missing":["vndb:v1"]}`)
	}))
	defer srv.Close()

	oldBase := nextmoeBaseURL
	oldClient := nextmoeHTTPClient
	nextmoeBaseURL = srv.URL
	nextmoeHTTPClient = srv.Client()
	t.Cleanup(func() {
		nextmoeBaseURL = oldBase
		nextmoeHTTPClient = oldClient
	})

	_, err := GetMoyuPatchByVNDB("v1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "Patch not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetMoyuPatchByVNDBInvalidID(t *testing.T) {
	_, err := GetMoyuPatchByVNDB("not-a-vndb-id")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetMoyuPatchByVNDBNoKey(t *testing.T) {
	moyuCacheEnabled = false
	t.Setenv("NEXTMOE_API_KEY", "")
	_, err := GetMoyuPatchByVNDB("v19658")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMoyuListJSON(t *testing.T) {
	raw := []byte(`{"object":"list","items":[],"next_cursor":null,"total":null}`)
	var page moyuList[MoyuPatch]
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
}
