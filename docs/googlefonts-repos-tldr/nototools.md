# googlefonts/nototools

<https://github.com/googlefonts/nototools>

Python helpers specific to the **Noto** super-family: QA, packaging,
cross-family consistency checks (same x-height, consistent
vertical-metric choices across scripts).

Relevance for proto-font:

- Our validation fixtures come from Noto; nototools documents which files
  are canonical per script.
- `noto_lint` imposes tighter constraints than Font Bakery on
  cross-family-consistent fields — useful when we want to assert
  invariants that Font Bakery considers advisory.
