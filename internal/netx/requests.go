package netx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"net/http"
	"sync"
	"time"
)

func ErrorMessage(body []byte) string {
	var value map[string]json.RawMessage
	if json.Unmarshal(body, &value) != nil {
		return ""
	}
	var message string
	var nested map[string]json.RawMessage
	if json.Unmarshal(value["error"], &nested) == nil {
		if json.Unmarshal(nested["message"], &message) == nil && message != "" {
			return message
		}
	}
	if json.Unmarshal(value["error"], &message) == nil && message != "" {
		return message
	}
	if json.Unmarshal(value["message"], &message) == nil {
		return message
	}
	return ""
}
func Request(ctx context.Context, client *http.Client, raw string) (*http.Request, error) {
	if client == nil {
		return nil, domain.NewError("validation_failed", "HTTP client is required")
	}
	if _, e := validHTTP(raw); e != nil {
		return nil, e
	}
	return http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
}
func FetchText(ctx context.Context, client *http.Client, raw string) (domain.HTTPTextResult, error) {
	req, e := Request(ctx, client, raw)
	if e != nil {
		return domain.HTTPTextResult{}, e
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	resp, e := client.Do(req)
	if e != nil {
		return domain.HTTPTextResult{}, NetworkError(e, "")
	}
	body, e := ReadBody(resp, 512*1024)
	if e != nil {
		return domain.HTTPTextResult{}, NetworkError(e, "")
	}
	return domain.HTTPTextResult{Status: uint16(resp.StatusCode), ContentType: resp.Header.Get("Content-Type"), FinalURL: resp.Request.URL.String(), Body: string(body)}, nil
}
func FetchBytes(ctx context.Context, client *http.Client, raw string) (domain.HttpBytesResult, error) {
	req, e := Request(ctx, client, raw)
	if e != nil {
		return domain.HttpBytesResult{}, e
	}
	resp, e := client.Do(req)
	if e != nil {
		return domain.HttpBytesResult{}, NetworkError(e, "")
	}
	body, e := ReadBody(resp, 512*1024)
	if e != nil {
		return domain.HttpBytesResult{}, NetworkError(e, "")
	}
	return domain.HttpBytesResult{Status: uint16(resp.StatusCode), ContentType: resp.Header.Get("Content-Type"), FinalURL: resp.Request.URL.String(), Base64: base64.StdEncoding.EncodeToString(body)}, nil
}
func ProbeURLs(ctx context.Context, client *http.Client, urls []string) ([]domain.UrlProbeResult, error) {
	if len(urls) > 100 {
		return nil, domain.NewError("validation_failed", "at most 100 URL probes are allowed")
	}
	for _, raw := range urls {
		if _, e := Request(ctx, client, raw); e != nil {
			return nil, e
		}
	}
	out := make([]domain.UrlProbeResult, len(urls))
	var wg sync.WaitGroup
	limit := make(chan struct{}, 8)
	for i, raw := range urls {
		wg.Add(1)
		go func(i int, raw string) {
			defer wg.Done()
			start := time.Now()
			result := domain.UrlProbeResult{URL: raw}
			defer func() { result.LatencyMS = uint64(time.Since(start).Milliseconds()); out[i] = result }()
			select {
			case limit <- struct{}{}:
				defer func() { <-limit }()
			case <-ctx.Done():
				v := NetworkError(ctx.Err(), "").Message
				result.Error = &v
				return
			}
			req, e := Request(ctx, client, raw)
			if e != nil {
				v := e.Error()
				result.Error = &v
				return
			}
			resp, e := client.Do(req)
			if e != nil {
				v := NetworkError(e, "").Message
				result.Error = &v
				return
			}
			resp.Body.Close()
			status := uint16(resp.StatusCode)
			result.Status = &status
			result.Ok = true
		}(i, raw)
	}
	wg.Wait()
	return out, nil
}
