# Google Fonts FAQ — TL;DR

Source: <https://developers.google.com/fonts/faq>

Distilled notes that are load-bearing for anyone shipping proto-font on top
of the Google Fonts catalog.

## License

- Every font in the catalog is OSS.
- Dominant license is **SIL Open Font License (OFL 1.1)**. A handful use
  **Apache 2.0** or the **Ubuntu Font License**.
- Use is unrestricted for commercial work: web, print, app bundles, logos.
- OFL forbids selling the font file standalone; it is **fine** to embed the
  font in products you sell. Reserved Font Name provisions mean if you
  modify a font you must rename it.
- Inclusion bar: only OFL-licensed fonts are accepted for new onboarding.

## Hosting options

| option | best for | gotchas |
| ------ | -------- | ------- |
| Google Fonts CSS API (`fonts.googleapis.com`) | public web, easy updates | third-party request; pins old files without periodic refresh |
| Self-host via `github.com/google/fonts` | offline distribution, bundled apps | you're responsible for staying current; check license per-family |
| `github.com/google/fonts/archive/main.zip` | one-shot snapshot of everything (~1 GB) | 1 GB is a lot; prefer `git clone` + shallow pull |

## File formats

- API serves **WOFF2** by default and can emit **TTF/OTF** with
  `capability=WOFF2` turned off (or `/download` variants).
- Many families ship as **variable fonts** with one or two files covering
  every weight/width combination, so filter `capability=VF` when you want
  one file per family.

## Noto and language coverage

- The **Noto** super-family covers 1000+ languages and 150+ scripts.
- The "Language" filter on the web UI and the `subset=` query param on the
  developer API (e.g. `subset=latin`, `subset=cyrillic-ext`) scope the
  response to writing systems.

## Privacy

- The CSS API does not set cookies on end users.
- Requests are logged for aggregate usage reporting; see the Privacy & Terms
  page linked from the FAQ for specifics.

## Contribution

- Submissions go through `google/fonts` as PRs. See upstream
  `CONTRIBUTING.md`.
- Fonts must pass **Font Bakery** checks before review.
- Material Icons has been frozen; new icon work lands in **Material
  Symbols**.

## What to watch

- The FAQ is a living page. Two things to re-check before relying on them:
  1. Any updated language counts for Noto.
  2. Whether `capability=COLRV1` is still "feature-preview" or GA — it was
     new as of 2024 and is surfaced by our `gfapi.FontFamily.ColorCapabilities`.
