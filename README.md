# synoppy-go

[![Go Reference](https://pkg.go.dev/badge/github.com/Synoppy/synoppy-go.svg)](https://pkg.go.dev/github.com/Synoppy/synoppy-go)

**Give your AI agents the whole web.** Synoppy is the web-data layer for AI agents — one key to **read, crawl, map, extract, classify & enrich** any site, plus screenshots and image scraping. Standard library only, zero dependencies.

[**Get a free key →**](https://synoppy.com/dashboard) · [Docs](https://synoppy.com/docs) · [synoppy.com](https://synoppy.com)

```bash
go get github.com/Synoppy/synoppy-go
```

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"os"

	synoppy "github.com/Synoppy/synoppy-go"
)

func main() {
	client := synoppy.New(os.Getenv("SYNOPPY_API_KEY")) // key looks like syn_...
	ctx := context.Background()

	// Read any URL -> clean markdown (+ render the page first if needed)
	page, err := client.Read(ctx, "https://stripe.com/blog", map[string]any{
		"formats":         []string{"markdown"},
		"onlyMainContent": true,
		"render":          "auto", // true | false | "auto"
		"waitMs":          500,
		"timeoutMs":       15000,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(page["markdown"])
	fmt.Println(page["metadata"]) // title, description, wordCount, rendered, bytesIn, ...

	// Screenshot a page (PNG data URL)
	shot, _ := client.Screenshot(ctx, "https://stripe.com", map[string]any{"fullPage": true})
	fmt.Println(shot["screenshot"]) // data:image/png;base64,...

	// Crawl a site (limit 1-25)
	site, _ := client.Crawl(ctx, "https://example.com", 25)
	fmt.Println(site["count"], "of", site["discovered"], "pages")

	// Brand intelligence — by url, domain, or work email
	brand, _ := client.Enrich(ctx, "https://linear.app")
	fmt.Println(brand["colors"], brand["fonts"])
}
```

## Methods

Every method takes a `context.Context` and returns `synoppy.Result` (a `map[string]any`),
so every field of the JSON response is reachable by key.

| Method | Endpoint | Signature |
| --- | --- | --- |
| `Read` | `POST /api/scrape` | `Read(ctx, url string, opts map[string]any)` |
| `Screenshot` | `POST /api/screenshot` | `Screenshot(ctx, url string, opts map[string]any)` |
| `Crawl` | `POST /api/crawl` | `Crawl(ctx, url string, limit int)` |
| `Map` | `POST /api/map` | `Map(ctx, url string)` |
| `Extract` | `POST /api/extract` | `Extract(ctx, url, prompt string)` |
| `Classify` | `POST /api/classify` | `Classify(ctx, url string, labels []string)` |
| `Enrich` / `Brand` | `POST /api/brand` | `Enrich(ctx, url string)` / `Brand(ctx, in BrandInput)` |
| `Images` | `POST /api/images` | `Images(ctx, url string)` |

### Read options (`map[string]any`)

`formats` (`[]string` of `"markdown"`/`"html"`/`"text"`), `onlyMainContent` (`bool`),
`timeoutMs` (number), `render` (`bool` or `"auto"`), `waitMs` (number).
Response: `metadata` (`title`, `description`, `language`, `siteName`, `author`, `ogImage`,
`sourceUrl`, `statusCode`, `wordCount`, `fetchedAt`, `rendered`, `bytesIn`), `markdown`/`html`/`text`,
`renderMs`, `latencyMs`.

### Screenshot options (`map[string]any`)

`fullPage` (`bool`), `waitMs` (number), `timeoutMs` (number).
Response: `screenshot` (PNG data URL), `sourceUrl`, `statusCode`, `fullPage`, `latencyMs`.
May return `503 RENDER_UNAVAILABLE` (surfaced as an `*APIError`).

### Classify

Default mode (`labels == nil`) returns `data` with NAICS/SIC fields: `industry`,
`naics_code`, `naics_title`, `naics_sector`, `naics_sector_title`, `naics_valid`,
`sic_code`, `sic_title`, `sic_division`, `sic_division_title`, `sic_valid`,
`categories`, `confidence`. Passing `labels` switches to label mode, returning
`data` `{label, matched, confidence, reasoning}`.

### Brand by domain or email

```go
brand, _ := client.Brand(ctx, synoppy.BrandInput{Domain: "linear.app"})
brand, _ = client.Brand(ctx, synoppy.BrandInput{Email: "founder@linear.app"}) // maps to the domain
```

### Coming soon

`/api/act` is not live yet, so the SDK does not expose it.
A method will be added once that endpoint ships.

## Credits

Every successful response includes the metered billing fields `creditsUsed` and
`creditsRemaining`. Read them with the helpers on `Result`:

```go
res, err := client.Read(ctx, "https://example.com", nil)
if err != nil {
	panic(err)
}
if used, ok := res.CreditsUsed(); ok {
	fmt.Printf("spent %.0f credits\n", used)
}
if left, ok := res.CreditsRemaining(); ok {
	fmt.Printf("%.0f credits remaining\n", left)
} else {
	fmt.Println("credits remaining: unlimited / unmetered")
}
```

`CreditsRemaining` reports `ok == false` when the API returns `null`
(unmetered or unlimited keys). You can also read the raw values directly,
e.g. `res["creditsUsed"]`.

## Errors

Failed requests return an `*synoppy.APIError` with `Code` and `Status`:

```go
_, err := client.Crawl(ctx, "https://example.com", 0)
var apiErr *synoppy.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Code, apiErr.Status)
}
```

## Configuration

```go
client := synoppy.New(
	os.Getenv("SYNOPPY_API_KEY"),
	synoppy.WithBaseURL("https://api.synoppy.com"), // override the base URL
	synoppy.WithHTTPClient(&http.Client{Timeout: 90 * time.Second}),
)
```

MIT licensed.
