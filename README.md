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

> **Status: release pending.** The features below are built and tested on Windows; the first tagged release has not been published yet, so build from source until it is. What is planned is listed under [Roadmap](#roadmap).

---

## Features

- **Seven overlays** for OBS browser sources: now playing, result, history, state, alert, lane keys and judgment curve (see [Overlays](#overlays))
- **11 built-in themes** and an accent color, plus [custom themes](docs/THEMES.md) and [custom overlays](docs/OVERLAYS.md) you drop in a folder
- **A local dashboard**: live status, overlay links with a theme picker, an OBS setup guide, your play history and settings, in English, French and German
- **A JSON and live-event API** for your own overlays and scripts
- **Judgment timeline**: the accuracy curve of a saved play, built from the score database
- **Degraded mode**: if Quaver is not found, `qsc` still starts and tells you what to fix
- **Opt-in lane keys and KPS** (Windows only), off by default

---

## Safe by design

QuaverStreamCompanion only reads files that Quaver already writes itself:

| File | Used for |
|------|----------|
| `Logs/runtime.log` | Live game state (menu, select, loading, gameplay, results) and the "score saved" signal |
| `quaver.db` | Map being played and the result of each play (opened **read-only**) |

The one optional exception is the lane keys overlay (see [Lane keys](#lane-keys)), which is off by default and works on Windows only. Apart from it, `qsc` never reads game memory, never modifies or injects into the game, never talks to the game's internal ports, and never writes anything to the Quaver folder. There is no telemetry, no account and no online service: nothing is ever sent anywhere, and the server listens on `127.0.0.1` only (requests with another `Host` header are rejected). `qsc` never opens a browser by itself: it only does so with `--open`.

---

## Compatibility

| Platform | Status |
|----------|--------|
| Windows 10 and later | Tested |
| Linux with Steam and Proton | Best effort. Steam layouts (native, Flatpak, Snap, extra libraries) are covered by tests that simulate them, but it has not been run on a real Linux machine yet |
| macOS | Untested |

Lane keys and KPS work on Windows only for now. See the [installation guide](docs/INSTALL.md#linux-and-proton) for the Linux details.

---

## Quick Start

**1. Download `qsc` for your platform** from the [releases page](https://github.com/kushiemoon-dev/quaver-stream-companion/releases) (see [Install](#install)), or [build from source](#build-from-source)

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
| Alert | 520 x 120 | `/overlays/alert/` |
| Lane keys | 480 x 200 (560 x 200 for 7K) | `/overlays/keys/` |
| Judgment curve | 520 x 180 | `/overlays/curve/` |

Each link starts with `http://127.0.0.1:7878`. Use the size as the width and height of the OBS Browser source.

### Run at startup

`qsc` never opens a browser unless you pass `--open`, so it can start with your computer without opening anything. It is a console program, so its window stays open: set the shortcut to run minimized. If it cannot find the Quaver folder it still starts in a degraded mode: the dashboard, the overlays and the settings work, the game state stays `unknown`, and the Status page explains what to do. Set the folder in Settings, with `quaver_dir` in the config file or with `--quaver-dir <folder>`; `qsc` re-checks every 15 seconds and starts reading Quaver by itself as soon as the folder is valid. `qsc doctor` still exits with an error in that case and prints the same advice.

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
| **Settings** | Quaver folder, port, dashboard language and the opt-in lane keys card |

A **Twitch** tab is shown as "coming soon"; nothing behind it exists yet. The dashboard is available in English, French and German.

---

## Themes

Overlays come with 11 built-in themes, and you can add your own.

<p align="center">
  <img src="docs/screenshots/themes.png" alt="The result overlay in each of the 11 built-in themes" width="800">
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
| `neon-cosy` | Original, dark violet with soft neon glow |

**Make your own.** A theme is one small CSS file of variables. Drop it in the `themes` folder next to your config file and select it with `?theme=<id>`. See the [theme guide](docs/THEMES.md).

Palettes are credited to Catppuccin, Tokyo Night, Dracula, Nord, Gruvbox and Rose Pine (all MIT). The Pixel theme uses VT323 by Peter Hull (SIL Open Font License 1.1). Details in the [theme guide](docs/THEMES.md#credits-and-licenses).

---

## Screenshots

All screenshots use synthetic data.

<p align="center">
  <img src="docs/screenshots/status.png" alt="Status page showing the live game state, the last play and the setup checks" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/overlays.png" alt="Overlays page with the browser source link, the theme picker, accent color and a preview" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/overlays-together.png" alt="Now playing, result, judgment curve, lane keys, alert and history overlays shown together" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/obs.png" alt="OBS page with the step by step guide and the link and size of every overlay" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/history.png" alt="History page listing recent plays with accuracy, max combo and judgments" width="800">
</p>
<p align="center">
  <img src="docs/screenshots/settings.png" alt="Settings page with the Quaver folder, port, language and the lane keys card" width="800">
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
| `GET /api/scores/{id}/timeline` | Accuracy curve of one saved play, from the score database |
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
  "overlay": { "theme": "neutral-light", "accent": "" },
  "keys": { "enabled": false }
}
```

An empty `quaver_dir` means auto-detection from your Steam libraries. `language` is `en`, `fr` or `de`. `accent` is a hex color without `#`. A missing file gives the defaults. A change of port, or of the Quaver folder once it is in use, takes effect the next time `qsc` starts; a folder saved while `qsc` is still looking for Quaver is picked up within 15 seconds. In a hand-edited JSON path, double each backslash (`\\`) or use forward slashes.

#### Lane keys

The lane keys overlay shows a bar per lane that lights up while its key is held, a press counter per lane, the keys per second (KPS) and the peak of the map. The numbers come from raw key state, they are not note accurate judgments.

**Privacy.** Lane keys are off by default. Only the lane keys you choose are read, and only while a map is being played. Nothing is stored or logged: no key names, no key history. The key state is read system wide, regardless of the focused window, so lane key timing may be sampled while you type elsewhere during gameplay. It works on Windows only for now, and a tap shorter than the polling interval may be missed. Overlays only receive per lane on/off states, counters and KPS.

To turn it on, use the Keys card on the dashboard Settings page, or edit the file: set `"keys": { "enabled": true, "layout": "auto", "k4": ["A","S","K","L"], "k7": ["A","S","D","SPACE","J","K","L"] }`. `layout` is `auto` (from the map's key count), `4k` or `7k`; each list holds exactly that many different key names (`A` to `Z`, `0` to `9`, `SPACE`, the arrows, `SHIFT_L`, `CTRL_L`, `ALT_L` and their `_R` twins, `;` `,` `.` `/` `'`, `F1` to `F12`). A change takes effect the next time `qsc` starts. The overlay stays blank until data arrives; if the feature is off or unsupported it shows a short hint instead.

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
- [x] Judgment curve of the last play, from the score database
- [ ] Replay analysis after each play (early / late split). Parked: it needs reading replay files
- [ ] Quaver public API: profile, rank (parked: it would be the first online feature, so it needs an explicit opt-in design)
- [ ] Optional Twitch module: chat commands (`!map`, `!last`, `!pb`) and personal best / full combo alerts
- [x] Custom overlays folder, lane keys and KPS overlay (Windows), degraded mode without Quaver
- [x] 11 ready-made themes (Catppuccin, Tokyo Night, Dracula, Nord, Gruvbox, Rose Pine, Pixel, Neon Cosy...) and a guide to make your own
- [ ] Windows `.exe`, Linux binaries and AppImage releases (release workflow added, first release pending)

---

## Install

Releases come as a Windows zip, Linux `.tar.gz` archives (amd64 and arm64), a Linux AppImage and a `checksums.txt` file. There is no installer: unzip, check the checksum, run `qsc`. The [installation guide](docs/INSTALL.md) covers verification, starting with your computer on Windows and Linux, updating and uninstalling. The AppImage is built by the release workflow and has not been run by the author on Linux yet.

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

## Guides

| Guide | What it covers |
|-------|----------------|
| [Installation](docs/INSTALL.md) | Download, checksums, startup on Windows and Linux, updating, uninstalling |
| [OBS setup](docs/OBS.md) | Adding the overlays to OBS, sizes, troubleshooting |
| [Overlays](docs/OVERLAYS.md) | Overlay options, the data reference and how to build your own overlay |
| [Themes](docs/THEMES.md) | Making a theme, the tokens, credits and licenses |

---

## Contributing

Issues and pull requests are welcome. Run `gofmt -l .`, `go vet ./...` and `go test ./...` before sending a change. To add a built-in theme, follow [Contributing a built-in theme](docs/THEMES.md#contributing-a-built-in-theme). The web files are plain HTML, CSS and JavaScript with no build step and no remote resources.

---

## License and credits

`qsc` is MIT licensed. See [LICENSE](LICENSE). Theme palettes come from Catppuccin, Tokyo Night, Dracula, Nord, Gruvbox and Rose Pine (all MIT), and the Pixel theme uses the VT323 font by Peter Hull under the SIL Open Font License 1.1 ([license text](web/themes/fonts/OFL.txt)). The full table is in the [theme guide](docs/THEMES.md#credits-and-licenses).
