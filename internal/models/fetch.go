package models

import (
	"context"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"net/http"
	"strings"
	"time"
)

const maxModelsBody = 4 * 1024 * 1024

func authorize(req *http.Request, protocol, key string) error {
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n") {
		return domain.NewError("validation_failed", "valid API key is required")
	}
	switch protocol {
	case "openai_compatible":
		req.Header.Set("Authorization", "Bearer "+key)
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		return domain.NewError("validation_failed", "unknown site protocol")
	}
	return nil
}
func Fetch(ctx context.Context, client *http.Client, site domain.SiteDTO, key string) (domain.FetchModelsResult, error) {
	preview, e := netx.NormalizeURL(site.BaseURL)
	if e != nil {
		return domain.FetchModelsResult{}, e
	}
	req, e := netx.Request(ctx, client, preview.ModelsURL)
	if e != nil {
		return domain.FetchModelsResult{}, e
	}
	if e = authorize(req, site.Protocol, key); e != nil {
		return domain.FetchModelsResult{}, e
	}
	start := time.Now()
	resp, e := client.Do(req)
	if e != nil {
		return domain.FetchModelsResult{}, netx.NetworkError(e, key)
	}
	body, e := netx.ReadBody(resp, maxModelsBody)
	if e != nil {
		return domain.FetchModelsResult{}, netx.NetworkError(e, key)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.FetchModelsResult{}, netx.StatusError(resp.StatusCode, body, key)
	}
	models, e := parseModels(body, site)
	if e != nil {
		return domain.FetchModelsResult{}, e
	}
	keyID := ""
	if site.ActiveAPIKeyID != nil {
		keyID = *site.ActiveAPIKeyID
	}
	return domain.FetchModelsResult{Models: models, LatencyMS: uint64(time.Since(start).Milliseconds()), Endpoint: preview.ModelsURL, FetchedAt: time.Now().UnixMilli(), APIKeyID: keyID}, nil
}
func parseModels(body []byte, site domain.SiteDTO) ([]domain.SiteModelDTO, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &response) != nil || len(response.Data) == 0 || string(response.Data) == "null" {
		return nil, domain.NewError("invalid_response", "models response must contain a data array")
	}
	var items []struct {
		ID          string  `json:"id"`
		DisplayName *string `json:"display_name"`
		OwnedBy     *string `json:"owned_by"`
	}
	if json.Unmarshal(response.Data, &items) != nil {
		return nil, domain.NewError("invalid_response", "invalid models response")
	}
	out := []domain.SiteModelDTO{}
	seen := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" || seen[item.ID] {
			return nil, domain.NewError("invalid_response", "model IDs must be nonempty and unique")
		}
		seen[item.ID] = true
		display := item.ID
		owned := item.OwnedBy
		if site.Protocol == "anthropic" {
			if item.DisplayName != nil {
				display = *item.DisplayName
			}
			v := "anthropic"
			owned = &v
		}
		keyID := ""
		if site.ActiveAPIKeyID != nil {
			keyID = *site.ActiveAPIKeyID
		}
		out = append(out, domain.SiteModelDTO{ID: domain.NewID(), SiteID: site.ID, APIKeyID: keyID, ModelID: item.ID, DisplayName: display, OwnedBy: owned})
	}
	return out, nil
}
func ProbeKey(ctx context.Context, client *http.Client, site domain.SiteDTO, key string) (domain.ProbeSiteApiKeyResult, error) {
	f, e := Fetch(ctx, client, site, key)
	if e != nil {
		return domain.ProbeSiteApiKeyResult{}, e
	}
	return domain.ProbeSiteApiKeyResult{ModelCount: len(f.Models), LatencyMS: f.LatencyMS, Endpoint: f.Endpoint}, nil
}
