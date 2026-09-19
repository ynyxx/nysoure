package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"nysoure/server/cache"
	"nysoure/server/model"
	"os"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v3"
)

var (
	nextMoeAPIKey = os.Getenv("NEXTMOE_API_KEY")
	vndbIDRegexp  = regexp.MustCompile(`^v\d+$`)
	moyuClient    = &http.Client{Timeout: 10 * time.Second}
)

// handleMoyuPatch looks up the 鲲 Galgame 补丁 (moyu.moe) page of a VNDB id
// through NextMoe's /v2/moyu API. The API only accepts server-side calls with
// an nmk_ key, so the browser cannot call it directly or via the generic proxy.
// The upstream PatchList is passed through unchanged.
func handleMoyuPatch(c fiber.Ctx) error {
	vndbID := c.Query("vndb_id")
	if nextMoeAPIKey == "" || !vndbIDRegexp.MatchString(vndbID) {
		return model.NewNotFoundError("Patch not found")
	}

	cacheKey := "moyu:patch:" + vndbID
	body, err := cache.Get(cacheKey)
	if err != nil {
		body, err = fetchMoyuPatch(vndbID)
		if err != nil {
			slog.ErrorContext(c, "Failed to fetch moyu patch", "vndb_id", vndbID, "error", err)
			return model.NewInternalServerError("Error")
		}
		if err := cache.Set(cacheKey, body, 24*time.Hour); err != nil {
			slog.ErrorContext(c, "Failed to cache moyu patch", "error", err)
		}
	}

	c.Response().Header.SetContentType(fiber.MIMEApplicationJSONCharsetUTF8)
	return c.SendString(body)
}

func fetchMoyuPatch(vndbID string) (string, error) {
	q := url.Values{}
	q.Set("refs", "vndb:"+vndbID)
	// nsfw defaults to false, which would hide every adult-rated game.
	q.Set("nsfw", "true")
	q.Set("include", "resources,publisher")
	req, err := http.NewRequest("GET", "https://api.nextmoe.dev/v2/moyu/patches?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+nextMoeAPIKey)
	resp, err := moyuClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, data)
	}
	return string(data), nil
}

func AddMoyuRoutes(router fiber.Router) {
	router.Get("/moyu/patch", handleMoyuPatch)
}
