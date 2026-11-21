package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/log"
)

var (
	gdriveTokenEndpoint  = "https://oauth2.googleapis.com/token"
	gdriveUploadEndpoint = "https://www.googleapis.com/upload/drive/v3"
	gdriveAPIEndpoint    = "https://www.googleapis.com/drive/v3"

	gdriveHTTPClient = &http.Client{
		Timeout: 30 * time.Second,
	}
	gdriveUploadHTTPClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 5 * time.Minute,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
)

type gdriveTokenCacheKey struct {
	endpoint     string
	clientID     string
	clientSecret string
	refreshToken string
}

type gdriveCachedToken struct {
	accessToken string
	expiresAt   time.Time
}

var (
	gdriveTokenCacheMu sync.Mutex
	gdriveTokenCache   = make(map[gdriveTokenCacheKey]gdriveCachedToken)
)

type GoogleDriveStorage struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	RefreshToken string `json:"refreshToken"`
	FolderID     string `json:"folderID,omitempty"`
}

func (g *GoogleDriveStorage) getAccessToken() (string, error) {
	key := gdriveTokenCacheKey{
		endpoint:     gdriveTokenEndpoint,
		clientID:     g.ClientID,
		clientSecret: g.ClientSecret,
		refreshToken: g.RefreshToken,
	}

	gdriveTokenCacheMu.Lock()
	if cached, ok := gdriveTokenCache[key]; ok && time.Now().Add(60*time.Second).Before(cached.expiresAt) {
		gdriveTokenCacheMu.Unlock()
		return cached.accessToken, nil
	}
	gdriveTokenCacheMu.Unlock()

	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("refresh_token", g.RefreshToken)
	form.Set("grant_type", "refresh_token")

	req, err := http.NewRequest(http.MethodPost, gdriveTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := gdriveHTTPClient.Do(req)
	if err != nil {
		log.Error("Failed to refresh Google Drive access token: ", err)
		return "", errors.New("failed to refresh Google Drive access token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errors.New("failed to read Google Drive token response")
	}
	if resp.StatusCode != http.StatusOK {
		log.Errorf("Google Drive token refresh failed (status %d): %s", resp.StatusCode, string(body))
		return "", errors.New("failed to refresh Google Drive access token")
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil || tokenResp.AccessToken == "" {
		log.Error("Invalid Google Drive token response: ", string(body))
		return "", errors.New("invalid Google Drive token response")
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	gdriveTokenCacheMu.Lock()
	gdriveTokenCache[key] = gdriveCachedToken{
		accessToken: tokenResp.AccessToken,
		expiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	gdriveTokenCacheMu.Unlock()

	return tokenResp.AccessToken, nil
}

func (g *GoogleDriveStorage) Upload(filePath string, fileName string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Error("Failed to open local file for Google Drive upload: ", err)
		return "", errors.New("failed to open local file")
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		log.Error("Failed to stat local file for Google Drive upload: ", err)
		return "", errors.New("failed to stat local file")
	}
	fileSize := stat.Size()

	accessToken, err := g.getAccessToken()
	if err != nil {
		return "", err
	}

	// Step 1: Initiate a resumable upload session.
	metadata := map[string]any{
		"name": fileName,
	}
	if g.FolderID != "" {
		metadata["parents"] = []string{g.FolderID}
	}
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	initURL := gdriveUploadEndpoint + "/files?uploadType=resumable&supportsAllDrives=true&fields=id"
	initReq, err := http.NewRequest(http.MethodPost, initURL, bytes.NewReader(metaBytes))
	if err != nil {
		return "", err
	}
	initReq.Header.Set("Authorization", "Bearer "+accessToken)
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	initReq.Header.Set("X-Upload-Content-Type", "application/octet-stream")
	initReq.Header.Set("X-Upload-Content-Length", strconv.FormatInt(fileSize, 10))

	initResp, err := gdriveHTTPClient.Do(initReq)
	if err != nil {
		log.Error("Failed to initiate Google Drive upload: ", err)
		return "", errors.New("failed to initiate Google Drive upload")
	}
	defer initResp.Body.Close()

	if initResp.StatusCode != http.StatusOK && initResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(initResp.Body, 1<<20))
		log.Errorf("Google Drive upload init failed (status %d): %s", initResp.StatusCode, string(body))
		return "", errors.New("failed to initiate Google Drive upload")
	}

	uploadURI := initResp.Header.Get("Location")
	if uploadURI == "" {
		return "", errors.New("missing upload session location from Google Drive")
	}

	// Step 2: Stream file content to the resumable upload URI.
	var uploadBody io.Reader = file
	if fileSize == 0 {
		uploadBody = http.NoBody
	}
	uploadReq, err := http.NewRequest(http.MethodPut, uploadURI, uploadBody)
	if err != nil {
		return "", err
	}
	uploadReq.ContentLength = fileSize
	uploadReq.Header.Set("Content-Type", "application/octet-stream")

	uploadResp, err := gdriveUploadHTTPClient.Do(uploadReq)
	if err != nil {
		log.Error("Failed to upload file content to Google Drive: ", err)
		return "", errors.New("failed to upload file to Google Drive")
	}
	defer uploadResp.Body.Close()

	uploadRespBody, err := io.ReadAll(io.LimitReader(uploadResp.Body, 1<<20))
	if err != nil {
		return "", errors.New("failed to read Google Drive upload response")
	}
	if uploadResp.StatusCode != http.StatusOK && uploadResp.StatusCode != http.StatusCreated {
		log.Errorf("Google Drive upload failed (status %d): %s", uploadResp.StatusCode, string(uploadRespBody))
		return "", errors.New("failed to upload file to Google Drive")
	}

	var fileMeta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRespBody, &fileMeta); err != nil || fileMeta.ID == "" {
		log.Error("Invalid Google Drive upload response: ", string(uploadRespBody))
		return "", errors.New("invalid Google Drive upload response")
	}

	// Step 3: Set public read permission so the file can be downloaded via share link.
	if err := g.setPublicPermission(accessToken, fileMeta.ID); err != nil {
		_ = g.Delete(fileMeta.ID)
		return "", err
	}

	return fileMeta.ID, nil
}

func (g *GoogleDriveStorage) setPublicPermission(accessToken string, fileID string) error {
	permURL := fmt.Sprintf("%s/files/%s/permissions?supportsAllDrives=true", gdriveAPIEndpoint, url.PathEscape(fileID))
	permBody := `{"role":"reader","type":"anyone"}`
	req, err := http.NewRequest(http.MethodPost, permURL, strings.NewReader(permBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gdriveHTTPClient.Do(req)
	if err != nil {
		log.Error("Failed to set Google Drive file permission: ", err)
		return errors.New("failed to set Google Drive file permission")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		log.Errorf("Google Drive set permission failed (status %d): %s", resp.StatusCode, string(body))
		return errors.New("failed to set Google Drive file permission")
	}
	return nil
}

func (g *GoogleDriveStorage) Download(storageKey string, _ string) (string, error) {
	if storageKey == "" {
		return "", ErrFileUnavailable
	}
	return fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=download&confirm=t", url.QueryEscape(storageKey)), nil
}

func (g *GoogleDriveStorage) Delete(storageKey string) error {
	if storageKey == "" {
		return ErrObjectNotFound
	}

	accessToken, err := g.getAccessToken()
	if err != nil {
		return err
	}

	deleteURL := fmt.Sprintf("%s/files/%s?supportsAllDrives=true", gdriveAPIEndpoint, url.PathEscape(storageKey))
	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := gdriveHTTPClient.Do(req)
	if err != nil {
		log.Error("Failed to delete file from Google Drive: ", err)
		return errors.New("failed to delete file from Google Drive")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrObjectNotFound
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		log.Errorf("Google Drive delete failed (status %d): %s", resp.StatusCode, string(body))
		return errors.New("failed to delete file from Google Drive")
	}

	return nil
}

func (g *GoogleDriveStorage) ToString() string {
	data, _ := json.Marshal(g)
	return string(data)
}

func (g *GoogleDriveStorage) FromString(config string) error {
	var cfg GoogleDriveStorage
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return err
	}
	g.ClientID = strings.TrimSpace(cfg.ClientID)
	g.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	g.RefreshToken = strings.TrimSpace(cfg.RefreshToken)
	g.FolderID = strings.TrimSpace(cfg.FolderID)
	if g.ClientID == "" || g.ClientSecret == "" || g.RefreshToken == "" {
		return errors.New("invalid Google Drive configuration")
	}
	return nil
}

func (g *GoogleDriveStorage) Type() string {
	return "gdrive"
}
