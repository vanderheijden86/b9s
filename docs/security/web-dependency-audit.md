# Web dependency audit: bd-969k

Evidence date: 2026-11-04. Base: `main` at
`3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`. This triages the
`npm ci` findings recorded by predecessor task `bd-e5u3.20.14`.

## Result and scope

The installed DOMPurify dependency is patched from **3.4.16 to 3.4.17** within
Mermaid's supported `^3.3.3` constraint. The lockfile change affects only its
version, tarball URL and integrity. No override or major upgrade is used.

**This does not patch the shipped Mermaid browser asset.** That asset includes
its own DOMPurify **3.4.12** and KaTeX **0.16.47**. The two remaining low audit
entries and the embedded dependencies are dispositioned below and tracked in
**bd-w8yz**, deferred for Polly's decision about a supported runtime upgrade.
This triage does not impose a zero-vulnerability gate or approve that migration.

| Audit scope | Exit | Critical | High | Moderate | Low | Total package entries |
| --- | --- | --- | --- | --- | --- | --- |
| Original committed lockfile | 1 | 1 | 0 | 0 | 2 | 3 |
| Patched lockfile | 1 | 0 | 0 | 0 | 2 | 2 |
| Patched lockfile, `--omit=dev` | 1 | 0 | 0 | 0 | 2 | 2 |

The raw [before](bd-969k-audit-before.json) and
[after](bd-969k-audit-after.json) JSON reports are retained beside this record.
An audit exit of 1 means findings remain. It is not a passing security check.
The three original package entries represent **four distinct advisories**:
DOMPurify has critical, high and low advisories grouped under its highest
severity, and Mermaid inherits KaTeX's low advisory.

## Installed and shipped dependency paths

```text
b9s-web
  mermaid 11.17.2 (direct production dependency, exact pin)
    dompurify 3.4.16 -> 3.4.17 (transitive production dependency)
    katex 0.16.47              (transitive production dependency)

web/build.mjs
  copies node_modules/mermaid/dist/mermaid.min.js verbatim
    pkg/web/dist/mermaid.min.js, embedded in the Go binary
      DOMPurify 3.4.12
      KaTeX 0.16.47
```

`npm ls mermaid dompurify katex --all` establishes the installed paths.
The browser asset's DOMPurify version assignment and license identify 3.4.12.
Mermaid's published `mermaid.min.js.map` includes
`katex@0.16.47/node_modules/katex/dist/katex.mjs`, whose source identifies 0.16.47.
Both the installed prebuilt asset and the committed asset have SHA-256:

```text
581ed7d74bd9048d0e3a91363927d72ef22942d7722546b27f7cc29e35390eb8
```

`web/src/mermaid.ts` loads that local asset when issue Markdown contains a
Mermaid fence. It passes diagram text to `mermaid.render` and inserts returned
SVG into the document. Thus these components process potentially untrusted
issue content **at browser runtime**, even though Node is used to build the app.
They are not development-only findings. Neither package runs in the Go server.
The installed DOMPurify patch protects consumers of the installed module, but
the current build only copies the prebuilt asset and uses Mermaid's types.

## Per-advisory disposition

| Package and advisory | Affected versions / fixed version | Exposure evidence and disposition |
| --- | --- | --- |
| DOMPurify, [GHSA-cg2g-xh64-v4q7](https://github.com/advisories/GHSA-cg2g-xh64-v4q7), critical | `>=3.4.4 <=3.4.16` / `3.4.17` | Installed and embedded versions are affected. The advisory requires `SAFE_FOR_TEMPLATES` with a DOM-returning mode and attached shadow-root input. The inspected b9s path supplies strings and does not request those modes. Patch the installed module. Keep embedded version maintenance in bd-w8yz. No current exploit was established. |
| DOMPurify, [GHSA-mrv2-3pqr-j5jr](https://github.com/advisories/GHSA-mrv2-3pqr-j5jr), high | `<=3.4.16` / `3.4.17` | Requires `IN_PLACE` and a hook detaching a parent or sibling before traversal. The inspected Mermaid sanitizers receive strings. Its anchor hooks change attributes, not parent/sibling ownership. Patch the installed module. Embedded version maintenance remains in bd-w8yz. |
| DOMPurify, [GHSA-c6gr-qvv4-5v3f](https://github.com/advisories/GHSA-c6gr-qvv4-5v3f), low | `<=3.4.16` / `3.4.17` | Requires XHTML parsing and permitted SVG animation attributes. b9s does not configure XHTML parsing. Patch the installed module. Embedded version maintenance remains in bd-w8yz. |
| KaTeX, [GHSA-238p-pmpm-9mq7](https://github.com/advisories/GHSA-238p-pmpm-9mq7), CVE-2026-103923, low | `>=0.11.0 <0.18.2` / `0.18.2` | Installed and embedded 0.16.47 are affected. Exploitation needs pre-existing prototype pollution or attacker control of an options prototype. No such primitive was established here. Strict-mode Mermaid sanitizes the final SVG separately. Retain the audit finding, and assess a supported runtime upgrade in bd-w8yz. |
| Mermaid, propagated low package entry | Audit range `10.9.0-rc.1 - 12.1.0`; npm proposes `12.3.0` | This entry inherits KaTeX's advisory. It is not a fifth independent advisory. Mermaid 11.17.2 requires KaTeX `^0.16.47`, excluding patched 0.18.2. Disposition is the same runtime upgrade assessment in bd-w8yz. |

The mode assessment uses `web/src/mermaid.ts` and the published source map's
`src/mermaidAPI.ts` and `src/diagrams/common/common.ts` callsites.
Mermaid's strict final SVG sanitizer uses `ADD_TAGS`, `ADD_ATTR` and
`HTML_INTEGRATION_POINTS`, with a string result. These observations narrow
exposure. They do not prove all diagram configuration, browser behavior, or
prototype-pollution combinations safe.

## Supported remediation and decision boundary

Registry metadata and `npm audit fix --dry-run --json` identify the compatible
DOMPurify patch. A fresh `npm ci` installs 3.4.17 from the changed lockfile.
Rebuilding refreshes `pkg/web/dist/source.sha256`, which hashes the lockfile.
The existing `TestEmbeddedBundleIsCurrent` guards that build input contract.
The prebuilt Mermaid asset's bytes and embedded versions remain unchanged.

`npm view mermaid@11 version dependencies --json` returned 11.17.2 as the latest
11.x version. `npm view mermaid@12.3.0 dependencies --json` reports
DOMPurify `^3.4.17` and KaTeX `^0.18.0`. That is the audit's supported major
upgrade candidate, not evidence that its **prebuilt** browser asset is patched.
bd-w8yz requires inspecting the candidate asset's actual versions and checking
ADR 0031's lazy local loading, strict rendering, source fallback, math and
browser compatibility before selecting a release.

An incompatible KaTeX override, `npm audit fix --force`, a replacement bundle
pipeline, and a major Mermaid migration are outside this narrow triage.
Changing the lockfile cannot rewrite code inside a published prebuilt asset.
No audit findings are suppressed, and no advisory is reclassified as fixed in
the shipped runtime.

## Reproduction and validation

Run from the repository root with Node 22.23.4, npm 10.9.9 and Go 1.25.5
(observed Linux/amd64). Audit results depend on registry advisory updates.
Compare the committed snapshots when reproducing at a later date.

```sh
npm --prefix web ci
npm --prefix web ls mermaid dompurify katex --all
npm --prefix web audit --json
npm --prefix web audit --omit=dev --json
npm view dompurify@3.4.17 version dist.integrity --json
npm view mermaid@11 version dependencies --json
npm view mermaid@12.3.0 dependencies --json
npm --prefix web run typecheck
npm --prefix web run build
npm --prefix web run test:guards
make test-stable
go build ./...
go vet ./...
```

The DOMPurify update command was
`npm --prefix web update dompurify --package-lock-only --ignore-scripts`.
Both baseline and patched `npm ci` exited 0, installing 122 packages and
auditing 123. Both full audits exited 1 with the measured counts above.
No application logic changes require a new test-first regression.
Existing bundle freshness and Mermaid browser tests cover the dependency build.

Observed validation on the patched lockfile:

| Check | Exit | Passed | Failed | Skipped |
| --- | --- | --- | --- | --- |
| Web typecheck | 0 | n/a | n/a | n/a |
| Web build | 0 | n/a | n/a | n/a |
| `test:guards` | 0 | 16 | 0 | 0 |
| `GOFLAGS=-v make test-stable`, top-level tests | 0 | 2085 | 0 | 18 |
| Same stable run, including subtests | 0 | 3447 | 0 | 20 |
| `go build ./...` | 0 | n/a | n/a | n/a |
| `go vet ./...` | 0 | n/a | n/a | n/a |
| Pinned browser smoke | 0 | 3 | 0 | 0 |
| Mermaid browser tests, Pixel Chromium and iPhone WebKit | 0 | 10 | 0 | 0 |

Stable tests ran alone, without a base comparison, under a 660-second outer
deadline and the Makefile's ten-minute package limit. There were 22 passing
packages. Skips require optional real BD binaries, S3, Dolt, Memory
preview/research, stress environments, or absent real-project fixtures.
`TestEmbeddedBundleIsCurrent` and documentation link tests passed.

Browser smoke used the installed Playwright 1.63.0 and all three image browsers.
The suite command was
`npm run test:e2e -- tests/mermaid.spec.ts --project pixel --project iphone --workers=1 --reporter=list,json`,
through `lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 420`.
The project's 30-second test timeout was unchanged. A fresh JSON report measured
10 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted and 0 unrun.
The script's guard precheck also passed all 16 tests.
The malicious-label and large-diagram tests assert no browser page errors.
These compatibility checks are not proofs against every advisory prerequisite.
