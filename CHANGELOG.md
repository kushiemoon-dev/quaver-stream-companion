# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A static landing page in `site/` with the install steps, a live demo of the real overlays (theme and language picker, and a simulator that plays made-up maps) and the privacy statements, and a workflow that builds it with `scripts/build-site.mjs` and can publish it with GitHub Pages once Pages is enabled in the repository settings.
- The overlay option `feed=parent`, which lets a framing page of the same origin send the events instead of qsc; the website demo uses it.
- A **Preview with demo data** switch on the Overlays page of the dashboard. It adds `demo=1` to the previews only, never to the link you copy for OBS, and is remembered in the browser. It starts on when Quaver is not found or no map is known. The Status page points to it when Quaver is not found.
- A session overlay (`/overlays/session/`, 480 x 170) shows the session statistics as a card, with `scale`, `lang`, theme and `demo` options, and a **Reset the session** button on the Status page of the dashboard. It is the eighth built-in overlay.
- Session statistics in memory since qsc started or the last reset: plays, passes, fails, average and best accuracy, best rating, full combos and pass streaks. They are in `GET /state` as `session`, in a new `session.updated` event sent right after each `score.saved`, and `POST /api/session/reset` clears them.
- The map object (`map.started`, the snapshot and results) carries `creator`, `bpm`, `length_ms`, `regular_notes`, `long_notes`, `mines`, `ranked_status`, `times_played`, `map_id` and `map_set_id`. A game database without those columns still works and reports empty values.
- The now playing overlay has a `meta` option (`?meta=1`, a **Map details** checkbox on the Overlays page) that adds one line with the mapper, BPM, length and note counts. BPM and length follow the play rate once a score of the map is known.
- The `text_files` setting (off by default, one checkbox in Settings, applies right away) keeps small UTF-8 text files in a `text` folder next to the config file: `title.txt`, `artist.txt`, `difficulty.txt`, `mapper.txt`, `nowplaying.txt`, `last_grade.txt` and `last_accuracy.txt`, for OBS Text sources that read from a file. They follow `map.started` and `score.saved`.
- `GET /api/text/{file}` (for example `/api/text/nowplaying.txt`) returns the same texts as `text/plain; charset=utf-8`, for local tools and browser sources.

## [0.1.1] - 2026-10-15

Fixes for Linux and for every Quaver key mode, and data that follows the game more closely.

### Added

- Lane keys and the keys overlay support every Quaver key mode, from 1K to 10K, and the scratch lane of the 7K+1 modes. Each layout has its own lane keys in the config (`k1` to `k10`, next to the existing `k4` and `k7`, which keep working) and in the Settings page, and an unsupported key mode no longer falls back to 4K.
- A smoke test in CI starts qsc for real on Ubuntu (amd64 and arm64) and Windows against synthetic Steam folders (native, Flatpak, Snap, symlinked and extra libraries), serves every built-in overlay, runs `qsc doctor` and `qsc stop`, and checks the release archives and the AppImage before a release is drafted.
- Every Quaver screen is its own state: `editor`, `importing`, `download`, `lobby`, `multiplayer`, `music` and `theatre` join `menu`, `select`, `loading`, `gameplay`, `results`, `initialization` and `closed`. The dashboard Status page and the state overlay show them with translated labels, and the state overlay `hide` list accepts them.
- The `game.state` event carries a `screen` field with the raw lower case Quaver screen name when it is known.
- A new `play.ended` event (`{ "map": <map object>, "saved": false }`) tells when a play left `gameplay` without a saved score, such as an autoplay, a replay, the theatre, an editor test play or a quit. The Status page shows "Last play was not saved" under the last play card until the next play.
- Score results carry `local_rank`, `local_total` and `is_local_best`, which rank each play on the local leaderboard of its map the way Quaver does: across every play speed, mod set and local profile, the 50 plays with the highest score, failed plays last, then performance rating, then accuracy. The result overlay shows a `#k` chip and a `Best` badge, the alert overlay announces a new best, and the History page shows the rank and both badges.
- Score results and `/api/history` carry `played_at_raw`, the date text exactly as Quaver saved it, and the History page formats the play time by language.
- The `watch_quaver_process` setting (on by default, one checkbox in Settings) lets qsc look at the names of running programs to notice when Quaver has crashed or was killed: after about 20 to 30 seconds without a Quaver process the state becomes `closed`. Nothing is stored or sent, and the check stays off on systems where qsc cannot list processes.

### Changed

- The format of `played_at` changed, its type is still a string: it was the date text Quaver saved, such as `10/14/2026 20:30`, and it is now an RFC 3339 timestamp with the local offset of the computer running qsc, such as `2026-10-14T20:30:00+02:00`. Text qsc cannot read is kept unchanged. Consumers of the old text should read `played_at_raw`.
- `is_pb` keeps its meaning, the best play at the same speed and mod set, and is now documented as the same speed personal best next to the local leaderboard fields.
- A mods value of `-1` (Quaver "None") counts as no mods, and mod bits qsc does not know stay in `mods_raw` without being listed in `mods`.
- The release binaries are built reproducibly: the same tag and Go version give the same file.

### Fixed

- qsc finds the native Linux build of Quaver (the executable is named `Quaver`), not only `Quaver.exe`.
- The AppImage runs every command (`doctor`, `stop`, `version`), not only the server.
- Custom theme CSS without spaces after the semicolons no longer loses colors in the dashboard swatches.
- A saved setting can no longer be overridden by a duplicate key in `config.json` that differs only by letter case.

## [0.1.0] - 2026-10-14

First stable release. The program is the same as `0.1.0-rc2`: this release updates the documentation and the release notes. See the entries below for everything it contains.

## [0.1.0-rc2] - 2026-10-14

Second release candidate, published as a pre-release.

### Added

- `qsc doctor --report` prints a redacted support report, and the Status page has a Copy diagnostics button (`GET /api/diagnostics`, same-origin only).
- A rolling `qsc.log` next to the config file, and `-v` / `--verbose` for debug lines.
- `qsc -h`, a clear message when the port is busy, and error windows that wait for Enter on Windows.
- The Settings page shows the config, themes and overlays paths and the version.
- Third party notices (`THIRD_PARTY_NOTICES.md`) in every archive, a security policy, a code of conduct, contributing guide and issue templates.
- An offline banner in the dashboard, translated error messages, a skip link, and numbers formatted by language (for example `97,42 %` in French).
- Overlay event streams reconnect by themselves, including after a refusal.

### Changed

- Result, alert and curve overlays stay in sync across reconnects, and a restart seeds the last score and map from the database.
- Overlays keep their layout with very long titles, a missing difficulty no longer breaks the now playing overlay, and the OBS page fits small screens.
- A score announced before its database row exists is retried in the background without delaying the next map, and score ids are remembered in a bounded set so a restored database does not hide new plays.
- Score announcements and map detection are more robust: slow database writes no longer block events, the log tailer is bounded, and the score store is reopened when the database file is replaced.
- The play history is answered with a single query.
- Unknown keys in the config file are kept when settings are saved.
- Release workflow: pinned actions and toolchain, and release notes taken from `docs/releases/<tag>.md` when it exists.

### Security

- The lane key stream (`keys` in `/state` and `GET /events?keys=1`) is same-origin only.
- At most 64 event streams at once, with write deadlines, and slow subscribers are disconnected.
- Built-in overlays are served with a Content Security Policy.
- The database path is built as a URL, and a Quaver folder reached through a link is checked after resolving it.

## [0.1.0-rc1] - 2026-10-13

First release candidate, published as a pre-release.

### Added

- Local server on `127.0.0.1` that follows Quaver through `Logs/runtime.log` and a read-only `quaver.db`: game state, map and score events, a `/state` snapshot and a Server-Sent Events stream at `/events`.
- Seven overlays for OBS browser sources: now playing, result, history, state, alert, lane keys and judgment curve.
- 11 built-in themes, an accent color, custom themes and custom overlays loaded from the config folder.
- A dashboard in English, French and German with status, overlay links and a theme picker, an OBS setup guide, play history and settings.
- Real score data: rate, mods, personal bests per map, speed and mod set, failed and paused plays.
- Judgment timeline of a saved play, built from the score database.
- Degraded mode when Quaver is not found, with the reason and what to fix.
- Opt-in lane keys and keys per second on Windows.
- A Quit button in the dashboard settings and a `qsc stop` command.
- `qsc doctor`, `qsc version`, and release archives for Windows and Linux (amd64, arm64), a Linux AppImage and `checksums.txt`.

[Unreleased]: https://github.com/kushiemoon-dev/quaver-stream-companion/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/kushiemoon-dev/quaver-stream-companion/releases/tag/v0.1.1
[0.1.0]: https://github.com/kushiemoon-dev/quaver-stream-companion/releases/tag/v0.1.0
[0.1.0-rc2]: https://github.com/kushiemoon-dev/quaver-stream-companion/releases/tag/v0.1.0-rc2
[0.1.0-rc1]: https://github.com/kushiemoon-dev/quaver-stream-companion/releases/tag/v0.1.0-rc1
