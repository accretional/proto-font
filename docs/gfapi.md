# Google Fonts developer API client

`internal/gfapi` is a minimal Go client for the
[Google Fonts developer API](https://developers.google.com/fonts/docs/developer_api).
It covers `GET /v1/webfonts` (list) and bulk font-file download.

## Auth

An API key is required. [Generate one in Google Cloud Console](https://developers.google.com/fonts/docs/developer_api#identifying_your_application_to_google)
and pass it to `gfapi.New`. No OAuth flow.

## Quick example

```go
package main

import (
    "context"
    "fmt"
    "os"

    "openformat/internal/fontcodec"
    "openformat/internal/gfapi"
)

func main() {
    ctx := context.Background()
    c := gfapi.New(os.Getenv("GOOGLE_FONTS_API_KEY"), nil)

    resp, err := c.ListWebfonts(ctx, gfapi.ListOptions{
        Sort:       "popularity",
        Family:     []string{"Noto Sans", "Roboto"},
        Capability: []string{"VF", "WOFF2"},
    })
    if err != nil {
        panic(err)
    }

    for _, fam := range resp.Items {
        fmt.Printf("%-20s  %s  %d files\n", fam.Family, fam.Category, len(fam.Files))
        // Pick the first file and decode it.
        for _, fileURL := range fam.Files {
            raw, err := c.Download(ctx, fileURL)
            if err != nil {
                continue
            }
            m, err := fontcodec.Decode(raw)
            if err != nil {
                continue
            }
            fmt.Printf("  -> %d tables\n", m.File.GetSfnt().GetNumTables())
            break
        }
    }
}
```

## Endpoints covered

| go method | API surface |
| --------- | ----------- |
| `ListWebfonts(ctx, ListOptions)` | `GET /v1/webfonts` with `key`, `sort`, `subset`, repeated `category=`, repeated `family=`, comma-joined `capability=` |
| `Download(ctx, url)`             | verbatim `GET` of any font file URL (typically `FontFamily.Files[<variant>]`) |

## `ListOptions` fields

| field | query param | notes |
| ----- | ----------- | ----- |
| `Sort` | `sort` | one of `alpha`, `date`, `popularity`, `style`, `trending` |
| `Subset` | `subset` | Unicode subset, e.g. `latin`, `cyrillic`, `greek-ext` |
| `Category` | repeated `category=` | e.g. `sans-serif`, `serif`, `display`, `handwriting`, `monospace` |
| `Family` | repeated `family=` | scope the response to specific families |
| `Capability` | comma-joined `capability=` | common values: `VF` (variable fonts), `WOFF2`, `COLRV1` |

## Non-goals

- No ETag / cache revalidation: results are small enough (≈ few hundred KB)
  that in-process or short-lived TTL caching on the caller side is fine.
- No retry / backoff: callers should wrap `Client` with their own retry
  policy; HTTP 429 responses surface as plain errors.
- No OAuth or service-account auth: the API does not require it.
