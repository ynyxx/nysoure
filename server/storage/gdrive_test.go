package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"nysoure/server/model"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string, headers map[string]string) *http.Response {
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func resetGDriveTestEnv(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	origToken := gdriveTokenEndpoint
	origUpload := gdriveUploadEndpoint
	origAPI := gdriveAPIEndpoint
	origHTTP := gdriveHTTPClient
	origUploadHTTP := gdriveUploadHTTPClient

	gdriveTokenEndpoint = "https://mock.google.test/token"
	gdriveUploadEndpoint = "https://mock.google.test/upload"
	gdriveAPIEndpoint = "https://mock.google.test/api"
	gdriveHTTPClient = &http.Client{Transport: rt}
	gdriveUploadHTTPClient = &http.Client{Transport: rt}

	gdriveTokenCacheMu.Lock()
	gdriveTokenCache = make(map[gdriveTokenCacheKey]gdriveCachedToken)
	gdriveTokenCacheMu.Unlock()

	t.Cleanup(func() {
		gdriveTokenEndpoint = origToken
		gdriveUploadEndpoint = origUpload
		gdriveAPIEndpoint = origAPI
		gdriveHTTPClient = origHTTP
		gdriveUploadHTTPClient = origUploadHTTP
		gdriveTokenCacheMu.Lock()
		gdriveTokenCache = make(map[gdriveTokenCacheKey]gdriveCachedToken)
		gdriveTokenCacheMu.Unlock()
	})
}

func TestGoogleDriveStorage_UploadDownloadDelete(t *testing.T) {
	var tokenCalls atomic.Int32
	var uploadedBytes []byte
	var initMeta map[string]any
	var permPayload map[string]string
	var deletedID string

	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			tokenCalls.Add(1)
			rawBody, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(rawBody))
			if err := r.ParseForm(); err != nil {
				return jsonResponse(http.StatusBadRequest, err.Error(), nil), nil
			}
			if r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "csec" || r.Form.Get("refresh_token") != "rtok" {
				return jsonResponse(http.StatusUnauthorized, "invalid credentials", nil), nil
			}
			return jsonResponse(http.StatusOK, `{"access_token":"test-access-token","expires_in":3600}`, nil), nil

		case r.Method == http.MethodPost && r.URL.Path == "/upload/files":
			if r.Header.Get("Authorization") != "Bearer test-access-token" {
				return jsonResponse(http.StatusUnauthorized, "unauthorized", nil), nil
			}
			if r.URL.Query().Get("uploadType") != "resumable" {
				return jsonResponse(http.StatusBadRequest, "expected resumable upload", nil), nil
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &initMeta)
			return jsonResponse(http.StatusOK, "", map[string]string{
				"Location": "https://mock.google.test/upload-session/123",
			}), nil

		case r.Method == http.MethodPut && r.URL.Path == "/upload-session/123":
			uploadedBytes, _ = io.ReadAll(r.Body)
			return jsonResponse(http.StatusOK, `{"id":"drive-file-id-1"}`, nil), nil

		case r.Method == http.MethodPost && r.URL.Path == "/api/files/drive-file-id-1/permissions":
			if r.Header.Get("Authorization") != "Bearer test-access-token" {
				return jsonResponse(http.StatusUnauthorized, "unauthorized", nil), nil
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &permPayload)
			return jsonResponse(http.StatusOK, `{"id":"anyoneWithLink"}`, nil), nil

		case r.Method == http.MethodDelete && r.URL.Path == "/api/files/drive-file-id-1":
			if r.Header.Get("Authorization") != "Bearer test-access-token" {
				return jsonResponse(http.StatusUnauthorized, "unauthorized", nil), nil
			}
			deletedID = "drive-file-id-1"
			return jsonResponse(http.StatusNoContent, "", nil), nil

		case r.Method == http.MethodDelete && r.URL.Path == "/api/files/missing-id":
			return jsonResponse(http.StatusNotFound, "", nil), nil

		default:
			return jsonResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	resetGDriveTestEnv(t, rt)

	tmpFile := filepath.Join(t.TempDir(), "sample.bin")
	content := []byte("hello google drive storage")
	if err := os.WriteFile(tmpFile, content, 0600); err != nil {
		t.Fatal(err)
	}

	g := &GoogleDriveStorage{
		ClientID:     "cid",
		ClientSecret: "csec",
		RefreshToken: "rtok",
		FolderID:     "folder-abc",
	}

	// Verify factory + serialization round-trip.
	reconstructed := NewStorage(model.Storage{
		Type:   g.Type(),
		Config: g.ToString(),
	})
	if reconstructed == nil {
		t.Fatal("NewStorage returned nil for gdrive")
	}

	// Upload
	key, err := reconstructed.Upload(tmpFile, "archive.zip")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if key != "drive-file-id-1" {
		t.Fatalf("unexpected storage key: %q", key)
	}
	if string(uploadedBytes) != string(content) {
		t.Fatalf("uploaded content mismatch: got %q", string(uploadedBytes))
	}
	if initMeta["name"] != "archive.zip" {
		t.Fatalf("unexpected upload name: %v", initMeta["name"])
	}
	parents, ok := initMeta["parents"].([]any)
	if !ok || len(parents) != 1 || parents[0] != "folder-abc" {
		t.Fatalf("unexpected parents metadata: %v", initMeta["parents"])
	}
	if permPayload["role"] != "reader" || permPayload["type"] != "anyone" {
		t.Fatalf("unexpected permission payload: %v", permPayload)
	}

	// Download
	dlURL, err := reconstructed.Download(key, "archive.zip")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	expectedURL := "https://drive.usercontent.google.com/download?id=drive-file-id-1&export=download&confirm=t"
	if dlURL != expectedURL {
		t.Fatalf("unexpected download URL: got %q, want %q", dlURL, expectedURL)
	}

	// Delete (should reuse cached token!)
	if err := reconstructed.Delete(key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if deletedID != "drive-file-id-1" {
		t.Fatalf("expected delete for drive-file-id-1, got %q", deletedID)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("expected access token to be cached (1 call), got %d calls", tokenCalls.Load())
	}

	// Delete missing object returns ErrObjectNotFound
	err = reconstructed.Delete("missing-id")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestGoogleDriveStorage_UploadRollsBackOnPermissionFailure(t *testing.T) {
	var rolledBack atomic.Bool

	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			return jsonResponse(http.StatusOK, `{"access_token":"tok","expires_in":3600}`, nil), nil
		case r.Method == http.MethodPost && r.URL.Path == "/upload/files":
			return jsonResponse(http.StatusOK, "", map[string]string{
				"Location": "https://mock.google.test/upload-session/1",
			}), nil
		case r.Method == http.MethodPut && r.URL.Path == "/upload-session/1":
			_, _ = io.Copy(io.Discard, r.Body)
			return jsonResponse(http.StatusOK, `{"id":"orphan-file-id"}`, nil), nil
		case r.Method == http.MethodPost && r.URL.Path == "/api/files/orphan-file-id/permissions":
			return jsonResponse(http.StatusForbidden, "forbidden", nil), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/api/files/orphan-file-id":
			rolledBack.Store(true)
			return jsonResponse(http.StatusNoContent, "", nil), nil
		default:
			return jsonResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	resetGDriveTestEnv(t, rt)

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	g := &GoogleDriveStorage{
		ClientID:     "cid",
		ClientSecret: "csec",
		RefreshToken: "rtok",
	}

	_, err := g.Upload(tmpFile, "test.txt")
	if err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected permission error, got %v", err)
	}
	if !rolledBack.Load() {
		t.Fatal("expected uploaded file to be deleted when setting public permission fails")
	}
}

func TestGoogleDriveStorage_FromStringValidation(t *testing.T) {
	var g GoogleDriveStorage
	if err := g.FromString(`{"clientID":"a","clientSecret":"","refreshToken":"c"}`); err == nil {
		t.Fatal("expected error when clientSecret is empty")
	}
	if err := g.FromString(`{"clientID":" a ","clientSecret":" b ","refreshToken":" c ","folderID":" f "}`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ClientID != "a" || g.ClientSecret != "b" || g.RefreshToken != "c" || g.FolderID != "f" {
		t.Fatalf("expected trimmed values, got %+v", g)
	}
}
