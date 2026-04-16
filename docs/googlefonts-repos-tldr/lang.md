# googlefonts/lang

<https://github.com/googlefonts/lang>

The canonical source of **languages, regions, and scripts** metadata used
to filter the Google Fonts catalog.

- **`Lib/gflanguages/languages_public.proto`** — vendored into
  `proto/googlefonts/v1/languages_public.proto`. Defines `LanguageProto`,
  `RegionProto`, `ScriptProto`, `SampleTextProto`, `ExemplarCharsProto`.
- The repo ships the actual **data** as `.textproto` files, not the `.proto`
  schema. We only consume the schema; downstream tools that want data should
  fetch the text-protos at build time.
- `SampleTextProto` is *also* defined in `gftools/fonts_public.proto` with
  the same name — our setup.sh keeps the two in sibling Go packages so
  they don't collide at compile time.

## Why this matters

- Language filtering on `fonts.google.com` and the `subset=` param on the
  developer API are driven by this data.
- If we want to surface "which fonts cover this language" in proto-font we
  load `languages_public` data + cross-reference against `CmapTable`
  coverage.
