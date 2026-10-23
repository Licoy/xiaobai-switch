package models

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"net/http"
	"strings"
	"time"
)

func ProbeEndpoint(protocol string, p domain.URLWritePreview) (string, error) {
	switch protocol {
	case "openai_compatible":
		return p.CodexBaseURL + "/chat/completions", nil
	case "anthropic":
		base := p.ClaudeBaseURL
		if !strings.HasSuffix(strings.ToLower(base), "/v1") {
			base += "/v1"
		}
		return base + "/messages", nil
	}
	return "", domain.NewError("validation_failed", "unknown site protocol")
}
func ProbeBody(protocol, modelID string) map[string]any {
	body := map[string]any{"model": modelID, "max_tokens": 16, "messages": []map[string]string{{"role": "user", "content": "Hi"}}}
	if protocol == "openai_compatible" {
		body["stream"] = false
	}
	return body
}
func interpretResponse(protocol string, status int, body []byte, key string) error {
	if status < 200 || status >= 300 {
		return netx.StatusError(status, body, key)
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(body, &value) != nil {
		return domain.NewError("invalid_response", "invalid response")
	}
	if protocol == "openai_compatible" {
		var choices []json.RawMessage
		if json.Unmarshal(value["choices"], &choices) == nil && len(choices) > 0 {
			return nil
		}
	} else {
		var kind string
		if json.Unmarshal(value["type"], &kind) == nil && kind == "message" {
			return nil
		}
		var content []json.RawMessage
		if len(value["content"]) > 0 && string(value["content"]) != "null" && json.Unmarshal(value["content"], &content) == nil {
			return nil
		}
	}
	return domain.NewError("invalid_response", "invalid response")
}
func Probe(ctx context.Context, client *http.Client, site domain.SiteDTO, key, modelID string) (domain.ModelProbeResult, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return domain.ModelProbeResult{}, domain.NewError("validation_failed", "model ID is required")
	}
	preview, e := netx.NormalizeURL(site.BaseURL)
	if e != nil {
		return domain.ModelProbeResult{}, e
	}
	endpoint, e := ProbeEndpoint(site.Protocol, preview)
	if e != nil {
		return domain.ModelProbeResult{}, e
	}
	start := time.Now()
	body := ProbeBody(site.Protocol, modelID)
	result := domain.ModelProbeResult{ModelID: modelID, Endpoint: endpoint}
	for attempt := 0; attempt < 2; attempt++ {
		status, err := sendProbe(ctx, client, endpoint, site.Protocol, key, body)
		result.Status = status
		result.LatencyMS = uint64(time.Since(start).Milliseconds())
		if err == nil {
			result.Ok = true
			return result, nil
		}
		if ctx.Err() != nil {
			return result, netx.NetworkError(ctx.Err(), key)
		}
		// One documented compatibility retry mirrors the existing OpenAI probe behavior
		if attempt == 0 && site.Protocol == "openai_compatible" && status != nil && *status == 400 && strings.Contains(err.Error(), "max_completion_tokens") {
			delete(body, "max_tokens")
			body["max_completion_tokens"] = 16
			continue
		}
		message := netx.Sanitize(err.Error(), key)
		result.Error = &message
		return result, nil
	}
	return result, domain.NewError("internal", "model probe exhausted its compatibility attempt")
}
func sendProbe(ctx context.Context, client *http.Client, endpoint, protocol, key string, body map[string]any) (*uint16, error) {
	payload, e := json.Marshal(body)
	if e != nil {
		return nil, e
	}
	if _, e = netx.Request(ctx, client, endpoint); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	if e = authorize(req, protocol, key); e != nil {
		return nil, e
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, netx.NetworkError(e, key)
	}
	status := uint16(resp.StatusCode)
	raw, e := netx.ReadBody(resp, 64*1024)
	if e != nil {
		return &status, netx.NetworkError(e, key)
	}
	return &status, interpretResponse(protocol, resp.StatusCode, raw, key)
}
