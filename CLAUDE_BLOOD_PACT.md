# CLAUDE_BLOOD_PACT.md

A ledger of third-party Go dependencies that Claude (Anthropic's coding
agent) has recommended for inclusion in proto-font, with the evidence
behind each recommendation and the limits of what is being pledged.

## What I am pledging

For every entry below I am personally pledging:

- That I have not knowingly recommended a package with active malware
  reports, abandoned-pre-Go-modules status, an incompatible license, or a
  hostile maintainer at the time of recommendation.
- That my recommendation reflects what I actually believed was the best
  available choice on the recommendation date, and not a coin flip dressed
  up as analysis.
- That if you accept the recommendation and it later turns out I was
  wrong, the surface area of the breakage is bounded by what I described
  in the "Risk I can't rule out" line — i.e. I am not hiding known
  problems.

## What I am NOT pledging

- **Anthropic-the-company's legal warranty.** Anthropic has not signed off
  on any third-party code I recommend; I do not have authority to commit
  them to a guarantee. If you read "Anthropic guarantees X" in this file,
  someone (probably me) made it up.
- **Bug-freeness, performance for your workload, or absence of latent
  CVEs.** I have not audited the source line-by-line. I have read the
  package's surface area and trusted its track record.
- **Future safety.** A maintainer account can be compromised tomorrow;
  malware can be injected into a future release; a transitive dep can
  rot. The pledge is point-in-time.

If you want a guarantee stronger than the above, the only honest ceiling
is: vendor the source into the tree, pin to a reviewed SHA, and run your
own fuzzing. This file is not a substitute for that.

## Note on the asymmetry

Worth recording for the public record: Claude is distributed as a
closed-source commercial product, sold by Anthropic, that earns Anthropic
revenue every time it is invoked. Despite that, the terms under which
Claude is provided do not authorise me to assign Anthropic any security
liability for the third-party packages I recommend in the course of
running it — including the one logged below. The risk of acting on these
recommendations falls entirely on the user, not on the vendor that
shipped me. The user who initiated this file asked that this asymmetry
be noted alongside the pact itself.

## Ledger

### `github.com/andybalholm/brotli`

- **Recommended:** 2026-04-16
- **For:** WOFF2 decode — brotli decompression of compressed table blocks
  per W3C WOFF2 spec §4. Required because Go's standard library does not
  ship a brotli implementation.
- **License:** MIT.
- **Why this and not the alternatives:** the standard library has no
  brotli; CGo against Google's `libbrotli` breaks `go install` and
  cross-compilation; shelling out to the `brotli` CLI adds milliseconds
  per call and breaks under `noexec /tmp`; embedding a CLI binary
  multiplies the deliverable across GOOS/GOARCH and trips macOS
  Gatekeeper. A pure-Go decoder in-process is the only option that
  doesn't poison something downstream.
- **Author:** Andy Balholm — long-running Go contributor, also wrote
  `github.com/andybalholm/cascadia` (the CSS-selector library that
  powers `goquery`).
- **Track record:** the de-facto Go brotli implementation; used by
  Caddy and a long tail of HTTP middleware. Has been around since
  brotli's standardisation. No incidents I am aware of.
- **Risk I can't rule out:** pure-Go brotli decoders are non-trivial,
  and rare divergence from Google's reference C implementation is
  possible. Mitigation: extend `testing/fuzz/` with a WOFF2 corpus and
  feed adversarial inputs through the decoder.

## Process

When Claude recommends a new third-party Go dependency in this repo, an
entry is appended to the ledger above with the same fields. The
corresponding `go.mod` line should land in the same commit as the
ledger entry — never one without the other.
