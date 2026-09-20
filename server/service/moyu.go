package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"nysoure/server/cache"
	"nysoure/server/model"

	"github.com/gofiber/fiber/v3/log"
)

const (
	nextmoeDefaultBaseURL = "https://api.nextmoe.dev"
	moyuCacheTTL          = 24 * time.Hour
	moyuMaxResourcePages  = 20
)

var (
	vndbIDPattern     = regexp.MustCompile(`^v\d+$`)
	nextmoeBaseURL    = nextmoeDefaultBaseURL
	nextmoeHTTPClient = &http.Client{Timeout: 20 * time.Second}
	moyuCacheEnabled  = true
	warnedNoMoyuKey   sync.Once
)

type MoyuPublisher struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type MoyuResource struct {
	Object                string         `json:"object"`
	ID                    string         `json:"id"`
	PatchID               string         `json:"patch_id"`
	Name                  string         `json:"name"`
	Storage               string         `json:"storage"`
	Size                  string         `json:"size"`
	Hash                  string         `json:"hash"`
	ModelName             string         `json:"model_name"`
	LocalizationGroupName string         `json:"localization_group_name"`
	Note                  string         `json:"note"`
	Type                  []string       `json:"type"`
	Language              []string       `json:"language"`
	Platform              []string       `json:"platform"`
	DownloadCount         int            `json:"download_count"`
	LikeCount             int            `json:"like_count"`
	WebURL                string         `json:"web_url"`
	CreatedAt             string         `json:"created_at"`
	UpdatedAt             string         `json:"updated_at"`
	Publisher             *MoyuPublisher `json:"publisher,omitempty"`
}

type MoyuPatch struct {
	Object            string         `json:"object"`
	ID                string         `json:"id"`
	VndbID            string         `json:"vndb_id"`
	CatalogWorkID     *string        `json:"catalog_work_id"`
	ContentLimit      *string        `json:"content_limit"`
	ReleaseDate       *string        `json:"release_date"`
	Type              []string       `json:"type"`
	Language          []string       `json:"language"`
	Platform          []string       `json:"platform"`
	ResourceCount     int            `json:"resource_count"`
	DownloadCount     int            `json:"download_count"`
	ViewCount         int            `json:"view_count"`
	FavoriteCount     int            `json:"favorite_count"`
	CommentCount      int            `json:"comment_count"`
	WebURL            string         `json:"web_url"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
	ResourceUpdatedAt string         `json:"resource_updated_at"`
	Publisher         *MoyuPublisher `json:"publisher,omitempty"`
	Resources         []MoyuResource `json:"resources"`
}

type moyuList[T any] struct {
	Object     string   `json:"object"`
	Items      []T      `json:"items"`
	NextCursor *string  `json:"next_cursor"`
	Total      *int     `json:"total"`
	Missing    []string `json:"missing"`
}

type moyuProblem struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func nextmoeAPIKey() string {
	return strings.TrimSpace(os.Getenv("NEXTMOE_API_KEY"))
}

func GetMoyuPatchByVNDB(vndbID string) (*MoyuPatch, error) {
	vndbID = strings.TrimSpace(vndbID)
	vndbID = strings.TrimSuffix(vndbID, "/")
	if i := strings.IndexAny(vndbID, "?#"); i >= 0 {
		vndbID = vndbID[:i]
	}
	if !vndbIDPattern.MatchString(vndbID) {
		return nil, model.NewRequestError("Invalid vndb_id")
	}

	key := nextmoeAPIKey()
	if key == "" {
		warnedNoMoyuKey.Do(func() {
			log.Warn("NEXTMOE_API_KEY is not set; moyu patch lookup is disabled")
		})
		return nil, model.NewNotFoundError("Patch not found")
	}

	cacheKey := "moyu:patch:" + vndbID
	if moyuCacheEnabled {
		if raw, err := cache.Get(cacheKey); err == nil {
			var cached MoyuPatch
			if json.Unmarshal([]byte(raw), &cached) == nil {
				return &cached, nil
			}
		}
	}

	patch, err := lookupMoyuPatch(key, vndbID)
	if err != nil {
		return nil, err
	}
	resources, err := listMoyuResources(key, patch.ID)
	if err != nil {
		return nil, err
	}
	if resources == nil {
		resources = []MoyuResource{}
	}
	patch.Resources = resources

	if moyuCacheEnabled {
		if raw, err := json.Marshal(patch); err == nil {
			if err := cache.Set(cacheKey, string(raw), moyuCacheTTL); err != nil {
				log.Error("Failed to cache moyu patch: ", err)
			}
		}
	}
	return patch, nil
}

func lookupMoyuPatch(key, vndbID string) (*MoyuPatch, error) {
	q := url.Values{}
	q.Set("refs", "vndb:"+vndbID)
	q.Set("nsfw", "true")
	endpoint := nextmoeBaseURL + "/v2/moyu/patches?" + q.Encode()

	var page moyuList[MoyuPatch]
	if err := nextmoeGet(key, endpoint, &page); err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return nil, model.NewNotFoundError("Patch not found")
	}
	return &page.Items[0], nil
}

func listMoyuResources(key, patchID string) ([]MoyuResource, error) {
	var all []MoyuResource
	cursor := ""
	for range moyuMaxResourcePages {
		q := url.Values{}
		q.Set("include", "publisher")
		q.Set("limit", "100")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		endpoint := nextmoeBaseURL + "/v2/moyu/patches/" + url.PathEscape(patchID) + "/resources?" + q.Encode()

		var page moyuList[MoyuResource]
		if err := nextmoeGet(key, endpoint, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = *page.NextCursor
	}
	return all, nil
}

func nextmoeGet(key, endpoint string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return model.NewInternalServerError("Failed to fetch moyu patch")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nysoure")

	resp, err := nextmoeHTTPClient.Do(req)
	if err != nil {
		log.Error("NextMoe request failed: ", err)
		return model.NewInternalServerError("Failed to fetch moyu patch")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.NewInternalServerError("Failed to fetch moyu patch")
	}

	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, dest); err != nil {
			log.Error("Failed to decode NextMoe response: ", err)
			return model.NewInternalServerError("Failed to fetch moyu patch")
		}
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return model.NewNotFoundError("Patch not found")
	}

	var problem moyuProblem
	_ = json.Unmarshal(body, &problem)
	detail := strings.TrimSpace(problem.Detail)
	if detail == "" {
		detail = fmt.Sprintf("NextMoe returned HTTP %d", resp.StatusCode)
	}
	log.Error("NextMoe error: ", problem.Code, " ", detail)
	if resp.StatusCode == http.StatusTooManyRequests {
		return model.NewInternalServerError("Moyu API rate limited")
	}
	return model.NewInternalServerError("Failed to fetch moyu patch")
}
