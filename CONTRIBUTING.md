# Contributing

Thanks for helping. Issues and pull requests are welcome. By taking part you agree to follow the [code of conduct](CODE_OF_CONDUCT.md). For a security problem, do not open an issue: see [SECURITY.md](SECURITY.md).

## Development setup

You need [Go](https://go.dev) 1.26 or newer (the `toolchain` line in `go.mod` pins the exact patch version). No C compiler and no Node build step are needed.

```sh
git clone https://github.com/kushiemoon-dev/quaver-stream-companion.git
cd quaver-stream-companion
go build -o qsc ./cmd/qsc
./qsc doctor
```

The web files in `web/` are plain HTML, CSS and JavaScript with no build step. They are embedded in the binary, so rebuild after editing them.

## Checks before sending a change

```sh
gofmt -l .
go vet ./...
go test ./... -count=1
```

`gofmt -l .` must print nothing, and `go vet` and the tests must pass. Tests use synthetic data, free ports (`127.0.0.1:0`) and temporary folders (`t.TempDir()`). They must never read your real config file, your Quaver folder or the real keyboard, and they must not leave a server running. Add a test that fails without your change.

## Measuring coverage

Go: `go test -cover ./...`.

JavaScript in `web/` (dashboard and overlay pages):

```sh
node scripts/js-coverage.mjs
```

The script runs `go test ./web` with V8 coverage, then prints a per-file table and a total. It needs Go, Node 20 or later and network access for `npx` on the first run. It writes `coverage-js/`, which is ignored by git. Add `--lines N`, `--branches N` or `--functions N` (or set `QSC_JS_COV_LINES`, `QSC_JS_COV_BRANCHES`, `QSC_JS_COV_FUNCTIONS`) to fail below a threshold.

## Testing the pages

The page scripts run unchanged in `node:vm` against a small browser stand-in, `web/testsupport/dom.js`: `openPage` loads a page and its scripts, `createWindow` gives bare globals. Tests drive a page through its `fetch` route table (`opts.routes`, answers built with `json`, `text` or `deferred`), its event streams (`win.eventSources`, with `open`, `emit` and `fail`), the fake clock (`await win.advance(ms)` also moves `Date`), `win.clipboard`, storage and `win.logs`, then assert the rendered text, attributes and requests (`win.fetchCalls`). Selectors support tag, id, class, attribute (spaces allowed in quoted values), descendant, child, lists and `:not()`, `:first-child`, `:last-child`, `:disabled`, `:checked`, `:empty`; anything else throws. Test scripts live in `web/testdata/` (the dashboard ones share `dashboard/helpers.js`) and run through the Go tests in `web/`, which skip when `node` is missing.

## Code rules

- Keep comments rare: a short doc comment on exported names, and the occasional comment for a non-obvious reason.
- Web code builds the DOM with `createElement` and `textContent`, never `innerHTML` with data. No remote resources: no CDN, no remote fonts, no external scripts.
- UI texts go through the language catalogs for English, French and German. The tests check that the three stay in sync.
- Do not use em dashes or en dashes in code, docs or tests. If a test must check for them, build the string from rune codes: `string([]rune{0x2014, 0x2013})`.
- Backslash pitfall: some editors collapse doubled backslashes in Go string literals, which silently changes a test. Build backslash-heavy values with `bs := string(rune(92))` and concatenate, instead of typing them.

## Commit style

Each commit message is one line, in the form `<type>: <description>`, with no body. The types are `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf` and `ci`. For example: `fix: keep the lane key stream same-origin`. Keep a commit to one change.

## Pull requests

Use the pull request template. Say what changed and why, and tick the checks above. Small, focused pull requests are reviewed faster than large ones.

## Adding a theme

A theme is one small CSS file of variables. Follow [Contributing a built-in theme](docs/THEMES.md#contributing-a-built-in-theme) in the theme guide: it lists the tokens, the contrast requirements and how to credit a source palette.

## Releases

How a release is cut is described in [docs/RELEASING.md](docs/RELEASING.md). User visible changes go in [CHANGELOG.md](CHANGELOG.md) under `Unreleased`.
