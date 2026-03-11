package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher performs HTTP requests to service APIs.
type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a Fetcher with a shared HTTP client.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch performs an HTTP request to a service endpoint and returns the response body.
func (f *Fetcher) Fetch(ctx context.Context, serviceURL, basePath string, ep EndpointDef, auth AuthDef, creds ServiceCredentials) ([]byte, error) {
	// Build full URL
	fullURL := strings.TrimRight(serviceURL, "/") + basePath + ep.Path

	// Expand template variables in params
	params := expandParams(ep.Params, creds, serviceURL)

	// Add query params
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		// If auth is api_key in query, add it here
		if auth.Type == "api_key" && auth.APIKey != nil && auth.APIKey.Location == "query" {
			q.Set(auth.APIKey.Name, creds.APIKey)
		}
		fullURL += "?" + q.Encode()
	} else if auth.Type == "api_key" && auth.APIKey != nil && auth.APIKey.Location == "query" {
		q := url.Values{}
		q.Set(auth.APIKey.Name, creds.APIKey)
		fullURL += "?" + q.Encode()
	}

	method := ep.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Apply authentication
	applyAuth(req, auth, creds)

	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", ep.Path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, ep.Path, truncate(string(body), 200))
	}

	return body, nil
}

// FetchAction performs an action request (POST/PUT/DELETE) against a service.
func (f *Fetcher) FetchAction(ctx context.Context, serviceURL string, action ActionDef, auth AuthDef, creds ServiceCredentials, bodyBytes []byte) ([]byte, error) {
	fullURL := strings.TrimRight(serviceURL, "/") + action.Endpoint

	method := action.Method
	if method == "" {
		method = http.MethodPost
	}

	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating action request: %w", err)
	}

	applyAuth(req, auth, creds)
	req.Header.Set("Accept", "application/json")
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("action %s: %w", action.ID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading action response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("action HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return body, nil
}

func applyAuth(req *http.Request, auth AuthDef, creds ServiceCredentials) {
	switch auth.Type {
	case "api_key":
		if auth.APIKey != nil && auth.APIKey.Location == "header" {
			req.Header.Set(auth.APIKey.Name, creds.APIKey)
		}
	case "custom":
		for _, entry := range auth.Custom {
			val := strings.ReplaceAll(entry.Value, "{{.api_key}}", creds.APIKey)
			req.Header.Set(entry.Name, val)
		}
	}
}

func expandParams(params map[string]any, creds ServiceCredentials, serviceURL string) map[string]any {
	if len(params) == 0 {
		return nil
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	todayPlus7 := now.AddDate(0, 0, 7).Format("2006-01-02")
	nowISO := now.Format(time.RFC3339)

	replacer := strings.NewReplacer(
		"{{.today}}", today,
		"{{.today_plus_7d}}", todayPlus7,
		"{{.now}}", nowISO,
		"{{.api_key}}", creds.APIKey,
		"{{.url}}", serviceURL,
	)

	expanded := make(map[string]any, len(params))
	for k, v := range params {
		if s, ok := v.(string); ok {
			expanded[k] = replacer.Replace(s)
		} else {
			expanded[k] = v
		}
	}
	return expanded
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
