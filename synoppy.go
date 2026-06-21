// Package synoppy is the official Go SDK for the Synoppy web-data API —
// scrape, screenshot, crawl, map, extract, classify, enrich (brand), and
// images on one key.
//
// Every successful response carries metered billing fields: use
// Result.CreditsUsed and Result.CreditsRemaining to read them.
package synoppy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Version is the SDK version, sent as part of the User-Agent header.
const Version = "1.0.0"

const defaultBaseURL = "https://synoppy.com"

// Client is a Synoppy API client.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (default https://synoppy.com).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient sets a custom *http.Client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// New creates a Client. apiKey is required and must be a Synoppy key
// (prefixed "syn_"), sent as "Authorization: Bearer syn_...".
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError is returned when the API responds with an error.
type APIError struct {
	Message string
	Code    string
	Status  int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("synoppy: %s (code=%s status=%d)", e.Message, e.Code, e.Status)
}

// Result is a decoded JSON response body. Because the underlying type is
// map[string]any, every field returned by the API is reachable by key
// (for example result["metadata"], result["pages"], result["screenshot"]).
// Helper accessors are provided for the billing fields common to every
// successful response.
type Result map[string]any

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case int:
		return float64(n), true
	}
	return 0, false
}

// CreditsUsed reports how many credits the call consumed. The boolean is
// false when the field is absent.
func (r Result) CreditsUsed() (float64, bool) {
	if r == nil {
		return 0, false
	}
	return toFloat(r["creditsUsed"])
}

// CreditsRemaining reports the credits left on the key after the call. The
// API may return null (unmetered/unlimited keys), in which case the second
// return value is false even though the field was present.
func (r Result) CreditsRemaining() (float64, bool) {
	if r == nil {
		return 0, false
	}
	return toFloat(r["creditsRemaining"])
}

func (c *Client) do(ctx context.Context, path string, body map[string]any) (Result, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("synoppy: apiKey is required")
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "synoppy-go/"+Version)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	data := Result{}
	_ = json.Unmarshal(raw, &data)

	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if success, has := data["success"]; has {
		if b, _ := success.(bool); !b {
			ok = false
		}
	}
	if !ok {
		msg, _ := data["error"].(string)
		code, _ := data["code"].(string)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		if code == "" {
			code = "ERROR"
		}
		return nil, &APIError{Message: msg, Code: code, Status: resp.StatusCode}
	}
	return data, nil
}

// Read (POST /api/scrape) fetches a URL and returns clean markdown / HTML /
// text plus page metadata.
//
// opts may include any of: "formats" ([]string of "markdown"|"html"|"text"),
// "onlyMainContent" (bool), "timeoutMs" (number), "render" (bool or "auto"),
// "waitMs" (number). The result carries metadata{title, description,
// language, siteName, author, ogImage, sourceUrl, statusCode, wordCount,
// fetchedAt, rendered, bytesIn}, markdown/html/text, renderMs, latencyMs,
// and creditsUsed/creditsRemaining.
func (c *Client) Read(ctx context.Context, url string, opts map[string]any) (Result, error) {
	body := map[string]any{"url": url}
	for k, v := range opts {
		body[k] = v
	}
	return c.do(ctx, "/api/scrape", body)
}

// Screenshot (POST /api/screenshot) captures a PNG of a page and returns it
// as a data URL in result["screenshot"].
//
// opts may include any of: "fullPage" (bool), "waitMs" (number),
// "timeoutMs" (number). The result carries screenshot, sourceUrl,
// statusCode, fullPage, latencyMs, and creditsUsed/creditsRemaining. The
// endpoint can return 503 RENDER_UNAVAILABLE, surfaced as an *APIError.
func (c *Client) Screenshot(ctx context.Context, url string, opts map[string]any) (Result, error) {
	body := map[string]any{"url": url}
	for k, v := range opts {
		body[k] = v
	}
	return c.do(ctx, "/api/screenshot", body)
}

// Crawl (POST /api/crawl) reads each page of a site (requires a key). limit
// is clamped by the API to 1-25; a limit <= 0 uses the API default. The
// result carries domain, discovered, count, pages[{url,title,markdown,words}],
// credits, latencyMs, and creditsUsed/creditsRemaining.
func (c *Client) Crawl(ctx context.Context, url string, limit int) (Result, error) {
	body := map[string]any{"url": url}
	if limit > 0 {
		body["limit"] = limit
	}
	return c.do(ctx, "/api/crawl", body)
}

// Map (POST /api/map) discovers every URL on a domain. The result carries
// domain, urls ([]string), count, source ("sitemap"|"links"), latencyMs,
// and creditsUsed/creditsRemaining.
func (c *Client) Map(ctx context.Context, url string) (Result, error) {
	return c.do(ctx, "/api/map", map[string]any{"url": url})
}

// Extract (POST /api/extract) returns structured JSON via AI (requires a
// key). prompt may be empty; it is sent under the "prompt" key (the API also
// accepts the "instruction" alias). The result carries url, model, data,
// metadata, truncated, usage{inputTokens,outputTokens}, latencyMs, and
// creditsUsed/creditsRemaining.
func (c *Client) Extract(ctx context.Context, url, prompt string) (Result, error) {
	body := map[string]any{"url": url}
	if prompt != "" {
		body["prompt"] = prompt
	}
	return c.do(ctx, "/api/extract", body)
}

// Classify (POST /api/classify) returns an industry classification or a
// best-matching label (requires a key).
//
// With labels == nil the default NAICS/SIC mode runs, returning data{industry,
// naics_code, naics_title, naics_sector, naics_sector_title, naics_valid,
// sic_code, sic_title, sic_division, sic_division_title, sic_valid,
// categories, confidence}. When labels are supplied, the result data is
// {label, matched, confidence, reasoning}. Both modes carry
// creditsUsed/creditsRemaining.
func (c *Client) Classify(ctx context.Context, url string, labels []string) (Result, error) {
	body := map[string]any{"url": url}
	if labels != nil {
		body["labels"] = labels
	}
	return c.do(ctx, "/api/classify", body)
}

// Enrich (POST /api/brand) resolves a URL into a brand profile. It is a
// convenience wrapper over Brand for the common url case.
//
// The result carries domain, name, description, logo, colors ([]string),
// fonts ([]string), address, socials ([{label,url}]), bytesIn, latencyMs,
// and creditsUsed/creditsRemaining.
func (c *Client) Enrich(ctx context.Context, url string) (Result, error) {
	return c.Brand(ctx, BrandInput{URL: url})
}

// BrandInput selects how a brand is looked up. Exactly one of URL, Domain, or
// Email should be set; the API maps a work Email to its Domain. If more than
// one is set, URL takes precedence, then Domain, then Email.
type BrandInput struct {
	URL    string
	Domain string
	Email  string
}

// Brand (POST /api/brand) resolves a URL, domain, or work email into a brand
// profile. See Enrich for the response shape.
func (c *Client) Brand(ctx context.Context, in BrandInput) (Result, error) {
	body := map[string]any{}
	switch {
	case in.URL != "":
		body["url"] = in.URL
	case in.Domain != "":
		body["domain"] = in.Domain
	case in.Email != "":
		body["email"] = in.Email
	default:
		return nil, fmt.Errorf("synoppy: Brand requires one of URL, Domain, or Email")
	}
	return c.do(ctx, "/api/brand", body)
}

// Images (POST /api/images) returns every image on a page. The result carries
// url, count, images ([{src,alt,width,height}]), bytesIn, latencyMs, and
// creditsUsed/creditsRemaining.
func (c *Client) Images(ctx context.Context, url string) (Result, error) {
	return c.do(ctx, "/api/images", map[string]any{"url": url})
}

// Act (/api/act) is not live yet — "coming soon". It is intentionally not
// implemented; calling the API directly would return an error. A method will
// be added here once the endpoint ships.
