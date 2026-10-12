<div align="center">

<img src="docs/banner.png" alt="QuaverStreamCompanion" width="600">

# QuaverStreamCompanion

### Live Quaver data for your stream overlays, read safely from the files the game already writes

[![GitHub Release](https://img.shields.io/github/v/release/kushiemoon-dev/quaver-stream-companion?style=flat-square&color=e5a00d)](https://github.com/kushiemoon-dev/quaver-stream-companion/releases/latest)
[![Stars](https://img.shields.io/github/stars/kushiemoon-dev/quaver-stream-companion?style=flat-square&color=a855f7)](https://github.com/kushiemoon-dev/quaver-stream-companion/stargazers)
[![License](https://img.shields.io/github/license/kushiemoon-dev/quaver-stream-companion?style=flat-square&color=gray)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)

![Windows](https://img.shields.io/badge/Windows-10+-0078D6?style=flat-square&logo=windows&logoColor=white)
![Linux](https://img.shields.io/badge/Linux-Steam%20%2B%20Proton-FCC624?style=flat-square&logo=linux&logoColor=black)

</div>

---

## Overview

**QuaverStreamCompanion** (`qsc`) is a small local app that follows your [Quaver](https://quavergame.com) games and serves what happens to your stream overlays: the game state, the map you are playing, and the result of every play.

It runs next to the game, keeps everything on your machine, and gives OBS a ready-made browser source plus a documented JSON and live-event API for anyone who wants to build their own overlay. A local dashboard in your browser shows the live status, gives you the overlay links, lists your recent plays and edits your settings.

> **Unofficial project.** Not affiliated with Quaver, and not related to the osu! StreamCompanion tool despite the similar name.

> **Status: early development (v0).** The core (game state, maps, scores, local server, a first overlay) is being built first. Everything under [Roadmap](#roadmap) is planned, not shipped yet.

---

## Safe by design

QuaverStreamCompanion only reads files that Quaver already writes itself:

| File | Used for |
|------|----------|
| `Logs/runtime.log` | Live game state (menu, select, loading, gameplay, results) and the "score saved" signal |
| `quaver.db` | Map being played and the result of each play (opened **read-only**) |

It never reads game memory, never modifies or injects into the game, never talks to the game's internal ports, and never writes anything to the Quaver folder. There is no telemetry, and the server listens on `127.0.0.1` only (requests with another `Host` header are rejected). `qsc` never opens a browser by itself: it only does so with `--open`.

---

## Quick Start

**1. Download `qsc` for your platform** (releases are coming; for now [build from source](#build-from-source))

**2. Check that it finds your game**

```
qsc doctor
```

**3. Start it and open the dashboard**

```
qsc --open
```

This opens `http://127.0.0.1:7878/` in your browser. Without `--open`, `qsc` only starts the server and never opens a browser.

**4. Add the overlay to OBS.** On the **Overlays** page, copy the overlay link and paste it into an OBS *Browser* source. The link looks like this:

```
http://127.0.0.1:7878/overlays/now-playing/
```

Then play a map. The overlay appears when you start playing and hides in the menu.

The full walkthrough, with troubleshooting, is in the [OBS setup guide](docs/OBS.md) and on the dashboard OBS page. To build an overlay of your own, see the [overlay guide](docs/OVERLAYS.md).

### Overlays

| Overlay | Recommended size | Link |
|---------|------------------|------|
| Now playing | 540 x 140 | `/overlays/now-playing/` |
| Result | 520 x 230 | `/overlays/result/` |
| History | 520 x 180 | `/overlays/history/` |
| State | 280 x 64 | `/overlays/state/` |

Each link starts with `http://127.0.0.1:7878`. Use the size as the width and height of the OBS Browser source.

### Run at startup

`qsc` never opens a browser unless you pass `--open`, so it can start with your computer without opening anything. It is a console program, so its window stays open: set the shortcut to run minimized. If it cannot find the Quaver folder it exits with an error, so pass `--quaver-dir <folder>` (or set `quaver_dir` in the config file) when auto-detection does not work.

- **Windows:** press Win+R, run `shell:startup`, and put a shortcut to `qsc.exe` in that folder. In the shortcut properties set Run to Minimized.
- **Linux:** run it from a systemd user service, or from your desktop autostart entries.
- **macOS:** add `qsc` to Login Items in System Settings.

---

## Dashboard

The dashboard is served at `http://127.0.0.1:7878/` and needs nothing but a browser.

| Page | What it does |
|------|--------------|
| **Status** | Live game state, current map, and the health of your setup |
| **Overlays** | Copy the OBS browser source link, pick a theme from the visual theme picker (built-in or your own custom themes) and an accent color, preview the result, and save it as your default look |
| **OBS** | A step by step guide to add the overlays to OBS, with the recommended sizes, copyable links and a connection check |
| **History** | Your recent plays: map, difficulty with rate and mods, grade, accuracy, max combo and judgments |
| **Settings** | Quaver folder, port and dashboard language |

A **Twitch** tab is shown as "coming soon"; nothing behind it exists yet. The dashboard is available in English, French and German.

---

## Themes

The overlay comes with 10 built-in themes, and you can add your own.

<p align="center">
  <img src="docs/screenshots/themes.png" alt="The ten built-in overlay themes shown side by side on the Overlays page" width="800">
</p>

| Theme id | Palette |
|----------|---------|
| `neutral-light` | Original |
| `neutral-dark` | Original |
| `catppuccin-mocha` | Catppuccin |
| `catppuccin-latte` | Catppuccin |
| `tokyo-night` | Tokyo Night |
| `dracula` | Dracula |
| `nord` | Nord |
| `gruvbox` | Gruvbox |
| `rose-pine` | Rose Pine |
| `pixel` | Original, with the VT323 font |

**Make your own.** A theme is one small CSS file of variables. Drop it in the `themes` folder next to your config file and select it with `?theme=<id>`. See the [theme guide](docs/THEMES.md).

Palettes are credited to Catppuccin, Tokyo Night, Dracula, Nord, Gruvbox and Rose Pine (all MIT). The Pixel theme uses VT323 by Peter Hull (SIL Open Font License 1.1). Details in the [theme guide](docs/THEMES.md#credits-and-licenses).

---

## Screenshots

<p align="center">
  <img src="docs/screenshots/status.png" alt="Status page showing the live game state, the current map and the setup checks" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/overlays.png" alt="Overlays page with the browser source link, theme and accent color choices, and a preview" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/history.png" alt="History page listing recent plays with accuracy, max combo and judgments" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/settings.png" alt="Settings page with the Quaver folder, port and language" width="800">
</p>

---

## How It Works

### Endpoints

| Endpoint | What it gives you |
|----------|-------------------|
| `GET /` | The dashboard |
| `GET /state` | JSON snapshot: game state, current map, last score |
| `GET /events` | Live event stream (Server-Sent Events) |
| `GET /overlays/now-playing/` | Ready-made overlay: title, artist, difficulty, key count |
| `GET /api/status` | Setup health: Quaver folder, files and version |
| `GET /api/config`, `PUT /api/config` | Read and save the settings file |
| `GET /api/history` | Recent plays (`?limit=`, 1 to 200) |
| `GET /api/themes` | Available overlay themes |

### Events

Every event is wrapped in the same versioned envelope so your overlay keeps working when new events are added:

```json
{ "v": 1, "type": "score.saved", "time": "2026-10-12T13:06:06Z", "data": { } }
```

| Type | When |
|------|------|
| `snapshot` | Sent once when you connect, with the full current state |
| `game.state` | The game moved to `menu`, `select`, `loading`, `gameplay` or `results` (also `initialization` while the game starts, and `closed` when Quaver is closed). The `/state` snapshot reports `unknown` before the first log line |
| `map.started` | A map started: title, artist, difficulty, stars, key count |
| `score.saved` | A play finished: judgments, accuracy, grade, max combo, performance `rating`, `is_pb`, `first_play`, `full_combo`, `failed`, play `rate`, `mods`, `mods_raw`, `paused` and `autoplay` |

Score details:

- `rate` is the speed of the play (`1.0` without a speed mod).
- `mods` lists the other mods by name (`[]` when none), and `mods_raw` is the raw value stored in `quaver.db`.
- `paused` is `true` when you paused during the play, and `autoplay` is `true` for an autoplay run.
- `is_pb` and `first_play` follow the rules in [Personal bests](#personal-bests).
- The map `difficulty` of a score is the difficulty at the played rate, not at 1.0x. `map.started` reports the 1.0x difficulty, since the rate is not known yet.

### Personal bests

A play is compared only with earlier plays of the same map, at the same play speed, with the same mod set. A 1.15x play is never compared with a 1.0x play, and a play with a given mod is never compared with one without it. NoFail and NoPause do not separate plays. Paused and Autoplay plays never count, and neither do failed plays.

### Why the result arrives at the end of the map

Quaver writes nothing about individual notes while you play, so there are no live judgments from the files it exposes. Game state and map changes are instant; the result of a play arrives the moment the game saves it. Live per-note judgments are deliberately out of scope, because getting them would mean reading the game's memory or modifying it.

### Overlay options

Overlays take URL parameters:

| Parameter | Example | Effect |
|-----------|---------|--------|
| `theme` | `?theme=neutral-light` | Pick a theme by id (a built-in one or your own); without it, the default saved on the Overlays page is used |
| `accent` | `?accent=ff6ac1` | Override the accent color (hex) |

---

## Config file

Settings are stored in a JSON file, written by the Settings and Overlays pages or edited by hand:

| OS | Location |
|----|----------|
| Windows | `%APPDATA%\quaver-stream-companion\config.json` |
| Linux | `~/.config/quaver-stream-companion/config.json` |

```json
{
  "quaver_dir": "",
  "port": 7878,
  "language": "en",
  "overlay": { "theme": "neutral-light", "accent": "" }
}
```

An empty `quaver_dir` means auto-detection from your Steam libraries. `language` is `en`, `fr` or `de`. `accent` is a hex color without `#`. A missing file gives the defaults. A change of port or Quaver folder takes effect the next time `qsc` starts. In a hand-edited JSON path, double each backslash (`\\`) or use forward slashes.

Precedence: command-line flag, then config file, then defaults. Use `--config <path>` to read another file.

---

## Commands

```
qsc                      start the server
qsc doctor               check the Quaver folder, files and port
qsc version              print the version
qsc --quaver-dir <dir>   use a specific Quaver folder (auto-detected from your Steam libraries otherwise)
qsc --port 7878          change the local port
qsc --config <path>      use another config file
qsc --open               also open the dashboard in your browser
```

The server only answers requests whose Host is `127.0.0.1:<port>` or `localhost:<port>`, so port forwarding or a reverse proxy gets a 403.

---

## Roadmap

- [x] Game state, map and score events, local server, first overlay
- [x] Settings dashboard in the browser
- [ ] Replay analysis after each play (timing graph, early / late split)
- [ ] Quaver public API: profile, rank
- [ ] Optional Twitch module: chat commands (`!map`, `!last`, `!pb`) and personal best / full combo alerts
- [x] 10 ready-made themes (Catppuccin, Tokyo Night, Dracula, Nord, Gruvbox, Rose Pine, Pixel...) and a guide to make your own
- [ ] Windows `.exe` and Linux AppImage releases

---

## Build from source

You need [Go](https://go.dev) 1.26 or newer. No C compiler is required.

```sh
git clone https://github.com/kushiemoon-dev/quaver-stream-companion.git
cd quaver-stream-companion
go build -o qsc ./cmd/qsc
go test ./...
```

---

## License

MIT. See [LICENSE](LICENSE).
