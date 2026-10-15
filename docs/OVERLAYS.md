# Build your own overlay

An overlay is a web page that OBS shows in a Browser source. QuaverStreamCompanion serves seven of them and lets you add your own: a folder with an `index.html` is enough. This guide lists the built-in overlays and their options, describes the data the companion publishes, and walks through two complete examples.

Everything below uses the default port. Replace `7878` if you changed it in the settings.

## 1. The built-in overlays and their options

| Overlay | Link | Recommended size |
|---------|------|------------------|
| Now playing | `http://127.0.0.1:7878/overlays/now-playing/` | 540 x 140 |
| Result | `http://127.0.0.1:7878/overlays/result/` | 520 x 230 |
| History | `http://127.0.0.1:7878/overlays/history/` | 520 x 180 |
| State | `http://127.0.0.1:7878/overlays/state/` | 280 x 64 |
| Alert | `http://127.0.0.1:7878/overlays/alert/` | 520 x 120 |
| Lane keys | `http://127.0.0.1:7878/overlays/keys/` | 480 x 200 (560 x 200 for 7K, 640 x 200 for 10K) |
| Judgment curve | `http://127.0.0.1:7878/overlays/curve/` | 520 x 180 |

Options go in the query string, for example `/overlays/result/?theme=nord&autohide=20&scale=1.2`. A value that cannot be read is ignored and the default is used (`theme`, `accent`, `lang`, `autohide`, `hide`, `demo`). A number outside its range is clamped to the nearest limit (`scale`, `n`), and a `scale` or `n` that is not a number uses the default.

| Option | Applies to | Values | Default |
|--------|-----------|--------|---------|
| `theme` | all | A theme id: lowercase letters, digits and hyphens | The default saved on the Overlays page, else `neutral-light` |
| `accent` | all | 3 to 8 hex digits without the `#`, for example `ff8800` | The theme accent |
| `lang` | all | `en`, `fr` or `de` | The browser language, else `en` |
| `scale` | all | A number from 0.5 to 3 | 1 |
| `demo` | all | `1` shows example data and does not connect | off |
| `autohide` | now playing, result, alert, curve | Seconds from 1 to 3600 until the card hides | never (alert: 8) |
| `n` | history | Number of plays from 5 to 50 | 20 |
| `best` | history | `1` draws a dashed line at the best accuracy of the plays shown | off |
| `sound` | alert | `1` plays a short synthesized chime when an alert appears | off |
| `counts` | keys | `0` hides the press counter under each lane | on |
| `peak` | keys | `0` hides the peak keys per second of the map | on |
| `layout`, `lanes`, `scratch` | keys | Only with `demo=1`, to preview a layout: `layout=7k` or `lanes=7` (1 to 10 key lanes), and `scratch=1` to add a scratch lane | 4 lanes |
| `hide` | state | Comma separated state names (see section 4) for which the badge is hidden, for example `menu,closed` | none |

The result overlay shows the grade, accuracy and judgments, the play speed and mods, a `#k` chip with the position of the play on the local leaderboard when it has one, and the badges `PB` (`is_pb`, the same speed personal best), `Best` (`is_local_best`), `FC`, failed, paused and autoplay. The History page of the dashboard shows the same rank and badges.

The lane keys overlay draws one lane bar per lane, from 1K to 10K plus the scratch lane of maps that have one (each lit while the key is down, with a smooth release), a press counter per lane, the KPS and the peak. It is hidden until lane data arrives (except with `demo=1`). When the feature is off or unsupported it shows a short hint line instead. Use 480 x 200 for 4K, 560 x 200 for 7K and 640 x 200 for 10K: the lanes share the width, so a narrower source still works but gets thinner bars. It needs lane keys turned on in the settings, see Lane keys below.

The alert overlay stays hidden until a saved play is a personal best, a new best on the local leaderboard or a full combo (and was not failed, paused or played with autoplay). It then shows "New personal best", "New best", "Full combo" or a pair of them, with the map title and accuracy; "New best" replaces "New personal best" when both apply. It never replays an old play when it connects.

The curve overlay stays hidden until a play is saved. It then reads the judgment timeline of that play (see `GET /api/scores/{id}/timeline`) and draws the accuracy after each judgment as a line, with a red tick at the bottom for every miss and the final accuracy at the top right. The horizontal axis is the judgment index, not time: the database stores no timestamps. It never replays an old play when it connects. Use 520 x 180.

## 2. Combining overlays and sizes

Add one Browser source per overlay and layer them in your scene. Each source has a transparent background, so you can place them anywhere over your game capture. Use the recommended size as the width and height of the source, and make it a little larger when the theme has a big shadow. Set a custom overlay to the size you declare in its `overlay.json` (see section 5). The [OBS setup guide](OBS.md) covers the OBS settings.

## 3. Themes

The built-in overlays read their colors, border, corners, shadow and font from CSS variables (tokens) set by a theme file. Your own overlay can use the same tokens, see example 2 below. How to pick, create and install themes is in [THEMES.md](THEMES.md).

## 4. Data reference

The companion publishes read-only data on `127.0.0.1`. All numbers are JSON numbers and all text is UTF-8. A request whose `Host` header is not `127.0.0.1:<port>` or `localhost:<port>` is rejected with 403.

| Route | Cross-origin readable | Use |
|-------|-----------------------|-----|
| `GET /state` | yes | The current snapshot as JSON |
| `GET /events` | yes | A live stream of events (Server-Sent Events) |
| `GET /api/history` | no, same-origin pages only | The latest plays |
| `GET /api/scores/{id}/timeline` | no, same-origin pages only | The judgment timeline of one saved play |
| `GET /api/status` | no, same-origin pages only | Version and health checks |
| `GET /api/overlays` | no, same-origin pages only | The custom overlays folder and its overlays |
| `GET /api/diagnostics` | no, same-origin pages only | The support report behind the Copy diagnostics button |

"Cross-origin readable" means the response carries `Access-Control-Allow-Origin: *`. A custom overlay runs with an opaque origin (see section 5), so it can use `/state` and `/events` but cannot read the `/api/` routes. Tools outside the browser (curl, scripts, bots) can read all of them.

### GET /state

```json
{
  "game": "menu",
  "map": null,
  "last_score": null,
  "keys": null
}
```

- `game`: the game state, one of the values listed below. It is `unknown` until the first state is seen.
- `keys`: the last lane keys frame, or `null` while key sampling is off or unsupported (see Lane keys below).
- `map`: the map being played, or `null`. See the map object below.
- `last_score`: the latest result, or `null`. See the result object below.

The snapshot only holds what the companion has seen since it started. After a restart, `map` and `last_score` stay `null` until the next play.

### GET /events

The response is `text/event-stream`. Each event has an `event:` name and a `data:` line holding one JSON envelope:

```json
{
  "v": 1,
  "type": "game.state",
  "time": "2026-10-12T21:42:16.7547593Z",
  "data": { "state": "menu" }
}
```

- `v`: the schema version, `1`.
- `type`: the event type, equal to the `event:` name.
- `time`: when the event was published, an RFC 3339 UTC timestamp.
- `data`: the payload, which depends on the type.

A comment line `: ping` is sent every 15 seconds to keep the connection open. There is no replay: a new connection first receives a `snapshot` event and then live events only. Events are not buffered for a reader that falls far behind. A browser `EventSource` reconnects by itself and receives a fresh `snapshot`, so rebuild your state from it.

| Type | `data` |
|------|--------|
| `snapshot` | The same object as `GET /state`: `game`, `map`, `last_score`, `keys`. Always the first event. |
| `game.state` | `{ "state": "<value>", "screen": "<value>" }`. `screen` is the raw lower case Quaver screen name and is left out when the state does not come from a screen (`closed`). |
| `map.started` | The map object, sent when gameplay begins |
| `score.saved` | The result object, sent when a play is saved |
| `play.ended` | `{ "map": <map object>, "saved": false }`, sent when a play left the `gameplay` screen and no score was saved for it. See Plays without a saved score below. |
| `keys.frame` | Lane keys, only when the optional key sampling is on: `{ "layout": "4k", "lanes": 4, "scratch": false, "down": [bool], "presses": [int], "kps": n, "peak": n }`. See Lane keys below. |

#### Game states

`state` (and `game` in the snapshot) is one of:

| Value | Meaning |
|-------|---------|
| `initialization` | The game is starting |
| `menu` | Main menu |
| `select` | Song select |
| `loading` | A map is loading |
| `gameplay` | A map is being played |
| `results` | Results screen |
| `editor` | The map editor |
| `importing` | Maps are being imported |
| `download` | The map download screen |
| `lobby` | A multiplayer lobby list |
| `multiplayer` | Inside a multiplayer room |
| `music` | The music player |
| `theatre` | Watching a replay |
| `closed` | The game has closed: Quaver wrote its exit line, or `qsc` found no Quaver process on three checks in a row, about 20 to 30 seconds (see Crash detection in the README) |
| `unknown` | Nothing seen yet (only in a snapshot) |

The game can report other screen names. They are passed through in lower case, so treat any value you do not know as "something else" instead of failing. The built-in overlays show it as `unknown`.

#### Plays without a saved score

Quaver counts a map as played as soon as the `gameplay` screen opens, but saves no score for an autoplay, a replay, spectating, the theatre, a test play in the editor, a quit, or a play with no hit. The `game.state` of such a play is still `gameplay` and `map.started` is still sent: qsc does not guess. When the screen leaves `gameplay` and no score was saved for that play, `play.ended` is sent with the map of the play and `"saved": false`. A retry (a new `gameplay` screen straight after another one) counts as leaving. `play.ended` always follows the `game.state` of the screen that was entered, and it is never sent for a saved play: `score.saved` already says so. When the score row is slow to appear, `play.ended` can arrive up to about 15 seconds after the screen change, after later events (even after the `map.started` of the next play), so match it to a play by `map.md5` rather than by position. The `snapshot` does not carry it, and the built-in overlays ignore it; the dashboard Status page shows "Last play was not saved" under the last play card until the next play starts.

```json
{ "v": 1, "type": "play.ended", "time": "2026-10-15T13:06:06Z", "data": { "map": { "title": "Song" }, "saved": false } }
```

#### Lane keys

Lane keys are off by default. Only the lane keys you choose are read, and only while a map is being played. Nothing is stored or logged: no key names, no key history. The key state is read system wide, regardless of the focused window, so lane key timing may be sampled while you type elsewhere during gameplay. It works on Windows only for now, and a tap shorter than the polling interval may be missed. Only per lane booleans, press counters and keys per second are published, never key names. Turn it on in the Keys card of the dashboard settings or with `keys.enabled` in the config file (see the README).

`keys.frame` is not sent to every client: open the stream with `GET /events?keys=1` to receive it. A plain `GET /events` (or `keys=0`) never carries these frames. The `snapshot` event and `GET /state` have the `keys` member (except for cross-origin readers, see below): the last frame, or `null` until one exists. The lane key data is same-origin only: a request that carries an `Origin` header different from its `Host` (any other web page, and a custom overlay, which runs in an opaque origin) gets `403` on `GET /events?keys=1`, receives no `keys` member in `GET /state` or in the `snapshot` event, and the `?keys=1` response never has `Access-Control-Allow-Origin`. Requests with no `Origin` header (OBS browser sources loading a built-in overlay, curl) are not affected. The built-in lane keys overlay uses `qscConnect(handlers, { keys: true })`.

A custom overlay therefore cannot read the keys stream. This is on purpose: it keeps other web pages in your browser from reading what you press.

`qsc` accepts at most 64 event streams (`/events`) at once and answers `503` beyond that. The built-in overlays reconnect on their own after 5 seconds, then 10, 20 and 30 seconds, until a slot is free. A custom overlay that uses `EventSource` directly must recreate it after a `503`, because the browser does not retry a stream that ended with an error status. A stream that cannot keep up is closed by the server, and `EventSource` reconnects with a fresh `snapshot`.

`keys.frame` is sent at most 30 times per second and only when something changed, with one final frame when you leave `gameplay`; counters and peak restart on every `map.started`.

| Field | Meaning |
|-------|---------|
| `layout`, `lanes`, `scratch` | `1k` to `10k`, or `unknown` when the layout is `auto` and the mode of the map is unknown (then there is no lane). `lanes` is the key count, plus one when `scratch` is `true`: the scratch lane is then the first lane. At most 11 lanes. Quaver has two scratch bindings per mode, both unset by default, while qsc reads a single scratch key (`keys.scratch`, default `SHIFT_L`). A scratch key you set must not be one of the lane keys of the layout in use; the default is simply ignored for a layout that already uses it as a lane key |
| `down` | One boolean per lane, true while the key is held |
| `presses` | Presses per lane since the map started |
| `kps` | Presses over the last second, all lanes together |
| `peak` | Highest `kps` of the current map |

These numbers come from raw key state: they are not note-accurate judgments. Only Windows is supported for now; on other systems the feature reports itself as unsupported.

#### The map object

Used by `map.started`, `map` in the snapshot, and `map` in a result.

| Field | Type | Meaning |
|-------|------|---------|
| `md5` | string | MD5 checksum of the map |
| `artist` | string | Artist |
| `title` | string | Title |
| `difficulty_name` | string | Name of the difficulty |
| `difficulty` | number | Star rating. In `map.started` it is the rating at 1.0x. In a result it is the rating at the rate that was played, when the game has it |
| `keys` | number | Lane count of the mode, from 1 to 10 (1K to 10K), without the scratch lane. `0` when the mode is unknown |
| `scratch` | boolean | `true` when the map has an extra scratch lane next to its `keys` lanes |
| `last_played_ms` | number | When the map was last played, in milliseconds since the Unix epoch |

#### The result object

Used by `score.saved` and `last_score`.

| Field | Type | Meaning |
|-------|------|---------|
| `score` | object | See the score object below |
| `map` | object | The map object, with the difficulty at the played rate |
| `is_pb` | boolean | A new personal best at the same play speed and mod set. Never true for a failed, paused or autoplay play |
| `local_rank` | number | Position of the play in the map's local leaderboard, counted from `1`, as the leaderboard stood right after the play. `0` when the play is not among its 50 entries |
| `local_total` | number | How many plays that leaderboard holds, at most `50` |
| `is_local_best` | boolean | The play leads the local leaderboard over at least one earlier play. Never true for a failed play |
| `first_play` | boolean | The first saved score on this map |
| `full_combo` | boolean | No miss, not failed and not autoplay |
| `failed` | boolean | The play was failed or quit |
| `paused` | boolean | The play was paused |
| `autoplay` | boolean | Autoplay was on |

The local leaderboard fields (`local_rank`, `local_total`, `is_local_best`) rank the plays of the map the way Quaver orders its local scores, across every play speed, mod set and local profile: Quaver mixes the local profiles in that leaderboard, and qsc follows it. The 50 plays with the highest `total_score` are kept (the earliest play wins a tie), then failed plays go last, then a higher performance `rating` goes first, then a higher `accuracy`; plays that still tie keep the earlier order. Quaver recalculates stale performance ratings when it opens a leaderboard, while qsc reads the stored value, so the rank of an old play can differ until the game has recomputed it. `is_pb` is the separate same speed rule: see Personal bests in the README. In `/api/history` the fields are flat, like the others.

The score object:

| Field | Type | Meaning |
|-------|------|---------|
| `id` | number | Score id in the game database |
| `grade` | string | `X`, `SS`, `S`, `A`, `B`, `C`, `D`, `F`, or `-` when unknown. A failed play is `F` |
| `accuracy` | number | Percent, for example `97.5` |
| `max_combo` | number | Highest combo |
| `judgments` | object | Counts: `marv`, `perf`, `great`, `good`, `okay`, `miss` |
| `total_score` | number | The score points |
| `played_at` | string | The time of the play as an RFC 3339 timestamp with the local offset of the computer running qsc, for example `2026-10-14T20:30:00+02:00`. If the game wrote a date qsc cannot read, this holds the original text |
| `played_at_raw` | string | The date text exactly as the game saved it, for example `10/14/2026 20:30` (local time, no zone) |
| `mods` | array of strings | Names of the active mods. The rate is not in this list |
| `mods_raw` | number | The raw mod bit field of the game. A negative value (Quaver "None") is reported as `0`. Bits this version does not know stay in the value but are not listed in `mods` |
| `rate` | number | Playback rate, `1` for normal speed |
| `rating` | number | Performance rating of the play |

#### What is not available

There is no per-note data and no live judgment, combo or accuracy while a map is being played. The companion reads only the game log and the score database, and a score is written to the database when the play ends. So you get the map when it starts and the full result when it is saved, but nothing in between. An overlay that needs live judgments cannot be built on this data. The one thing available afterwards is the judgment timeline of a saved play, see `GET /api/scores/{id}/timeline`.

### GET /api/history

For pages served by the companion itself (the built-in History overlay uses it). `limit` is an optional query parameter, from 1 to 200, default 50; anything else gives a 400 with `{"error": "...", "field": "limit"}`. The answer is an array, newest first, of:

```json
{
  "id": 1,
  "played_at": "2026-10-12T13:06:06+02:00",
  "played_at_raw": "2026-10-12 13:06:06",
  "title": "Some Title",
  "artist": "Some Artist",
  "difficulty_name": "Hard",
  "difficulty": 15.7,
  "keys": 4,
  "rate": 1.1,
  "mods": [],
  "grade": "SS",
  "accuracy": 97.5,
  "max_combo": 500,
  "judgments": { "marv": 400, "perf": 90, "great": 10, "good": 0, "okay": 0, "miss": 0 },
  "failed": false,
  "paused": false,
  "is_pb": false,
  "full_combo": true,
  "autoplay": false,
  "local_rank": 3,
  "local_total": 12,
  "is_local_best": false
}
```

The fields mean the same as in the result object, but the map and score fields are flattened into one object.

### GET /api/scores/{id}/timeline

Same-origin only. `id` is the `score.id` of a saved play (the `id` of an `/api/history` entry). It returns the judgments of that play as an accuracy curve:

```json
{
  "score_id": 12,
  "total": 1200,
  "points": [100, 99.13, 98.5],
  "misses": [140, 410, 411],
  "counts": [[6, 2, 1, 0, 0, 1], [8, 1, 0, 1, 0, 0], [9, 0, 1, 0, 0, 0]]
}
```

- `total` is the number of judgments of the play.
- `points` holds at most 200 values. Each one is the accuracy in percent after a segment of judgments: segment `i` ends at judgment `(i + 1) * total / len(points)`. Accuracy uses Quaver's standard weights (Marvelous 100, Perfect 98.25, Great 65, Good 25, Okay -100, Miss -50), divided by the number of judgments so far and never below 0. It is the formula behind the accuracy stored in `quaver.db`, so the last point equals the final accuracy of the play.
- `counts` has one entry per point: the judgments inside that segment, in the order Marvelous, Perfect, Great, Good, Okay, Miss.
- `misses` lists the 0-based judgment indexes of misses, at most 500 (evenly thinned when there are more).

The judgments come in the order they were recorded, and the database keeps no timestamps, so the curve is labeled by judgment index, not time. The answer is a 404 with `{"error": "..."}` when the score does not exist or has no usable judgment data (older scores, or an unreadable value), and a 405 for any method other than GET.

### GET /api/status

Also same-origin only. It returns the keys `version`, `game`, `map`, `last_score` (as in the snapshot), `checks`, `config_path` and `uptime_s`. Each entry of `checks` is `{ "id", "ok", "detail" }`, with `id` one of `quaver_dir`, `log`, `database` and `port` (and `keys` while key sampling is on). It also returns `keys`: `{ "enabled", "supported", "layout" }`. Use it to verify that the companion is healthy.

## 5. Your own overlay folder

Custom overlays live in an `overlays` folder next to the settings file:

| OS | Folder |
|----|--------|
| Windows | `%APPDATA%\quaver-stream-companion\overlays\` |
| Linux | `~/.config/quaver-stream-companion/overlays/` (or `$XDG_CONFIG_HOME/quaver-stream-companion/overlays/` when that variable is set) |
| macOS | `~/Library/Application Support/quaver-stream-companion/overlays/` |

The exact path is shown on the **Overlays** page and on the **OBS** page of the dashboard. If you start `qsc` with `--config`, the folder is next to that file. It is not available when the settings file is on a network share.

Create one folder per overlay. The folder name is the id and is part of the link:

```
overlays/
  mini-card/
    index.html
    overlay.json
    style.css
```

This overlay is served at `http://127.0.0.1:7878/overlays/mini-card/`. Files are read at each request, so edit and refresh. In OBS, right-click the source and choose "Refresh cache of current page". Overlays appear on the Overlays page (reload the page after adding one), with a preview, a link to copy and the size from `overlay.json`.

### overlay.json (optional)

```json
{
  "name": "Mini card",
  "description": "The map title and the accuracy of the last play.",
  "width": 520,
  "height": 90
}
```

`name` (up to 40 characters, default: the folder name), `description` (up to 160 characters) and the recommended size `width` (100 to 3840, default 500) and `height` (40 to 2160, default 200); sizes outside the range are clamped. The file is at most 8 KiB. A missing, invalid or oversized file gives the defaults. Unknown keys are ignored.

### Rules and limits

- The folder name uses only lowercase letters, digits and hyphens. A name used by a built-in overlay (`now-playing`, `result`, `history`, `state`, `alert`, `keys`, `curve`) is ignored: the built-in one always wins.
- `index.html` is required. At most 100 overlays are listed.
- An overlay holds at most 200 files and folders, nested at most 3 levels deep (for example `index.html`, `css/style.css`, `img/icons/star.svg`).
- A file is at most 2 MiB. Names use only letters, digits, `.`, `_` and `-`, and do not start with a dot.
- Allowed extensions: `.html`, `.htm`, `.css`, `.js`, `.mjs`, `.json`, `.svg`, `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.ico`, `.woff2`, `.ttf`, `.txt`. Other files are not served.
- Only regular files are served. Links (symbolic links) are refused, and the name must match the file on disk exactly, including its case.
- Only `GET` is accepted. The link `/overlays/mini-card` redirects to `/overlays/mini-card/`.

### The sandbox

Each file of a custom overlay is served with `Content-Security-Policy: sandbox allow-scripts`. The page runs scripts, but it has an opaque origin. In practice:

- `fetch('/state')` and `new EventSource('/events')` work, because those routes allow cross-origin reads. The lane keys data does not: the `keys` field is left out and `/events?keys=1` answers `403` for a sandboxed page.
- The `/api/` routes cannot be read, and the write API is unreachable. A custom overlay cannot change your settings.
- `localStorage`, `sessionStorage`, cookies and IndexedDB are not available (using them throws an error).
- Forms and pop-ups are blocked, and so is top-level navigation from the sandboxed page (it cannot send the source to another address).
- Custom overlays do not get the saved default theme of the Overlays page. Add `?theme=<id>` to the link when you use the shared theme files (see example 2).

The sandbox does not block network access, so a page can still contact other sites.

### Trust and secrets

An overlay is code that runs in your browser source. Only install overlays you wrote or have read. Custom overlay files are served with `Access-Control-Allow-Origin: *`, so any web page open in your browser can read them while the companion runs. Do not rely on privacy here: never put secrets (tokens, keys, passwords) in an overlay folder.

### Shared files

A custom overlay can load these files from the companion:

- `/overlays/overlay.css`: base styles with the `.card`, `.title`, `.sub` and `.grade` classes and the colors `.gold`, `.green`, `.blue`, `.amber`, `.red` and `.muted`.
- `/overlays/i18n.js`, `/overlays/model.js` and `/overlays/common.js`, in that order. `common.js` reads the options of section 1 (`theme`, `accent`, `lang`, `scale`, `demo`, `autohide`, `hide`, `n`, `best`, `sound`, `counts`, `peak`), loads the theme file and sets the tokens `--bg`, `--fg`, `--muted`, `--accent`, `--border`, `--radius`, `--shadow`, `--font` and `--scale` on the page. It provides:
  - `qscConnect(handlers)`: opens `/events` and calls `handlers[type](data)` with the `data` of each event. It returns the `EventSource`. It does not catch errors: a payload that cannot be parsed throws inside the handler, and the handler's own errors are yours to catch.
  - `qscEl(tag, className, text)`: creates an element and sets its text with `textContent`.
  - `qscParams`: the parsed options `{ demo, lang, scale, autohide, hide, n, best, sound, counts, peak }`.
  - `qscT(key)` and `qscAutohide(onHide)`: translated labels of the built-in overlays and the `autohide` timer.
  - `qscModel`: helpers shared with the built-in overlays, such as `formatAccuracy`, `gradeText`, `gradeClass` and `resultView`.
  - These helpers belong to the built-in overlays and can change between versions. For a long lived overlay, rely only on the routes of section 4.

## 6. Example 1: a minimal overlay

This overlay shows the map title and the accuracy of the last play. It connects to `/events`, keeps its state from the `snapshot`, ignores bad payloads, shows a status while reconnecting and writes text with `textContent` only. It does not use any shared file.

Save it as `overlays/mini-card/index.html` and `overlays/mini-card/overlay.json`. The files are also in [`docs/examples/mini-card`](examples/mini-card).

`index.html`:

```html
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Mini card</title>
<style>
  body { margin: 0; font: 600 28px system-ui, sans-serif; color: #fff; text-shadow: 0 2px 6px #000; }
  #status { font-size: 14px; opacity: 0.7; }
</style>
</head>
<body>
<div id="title"></div>
<div id="acc"></div>
<div id="status"></div>
<script>
  const $ = (id) => document.getElementById(id);
  let map = null;
  let last = null;

  function render() {
    $('title').textContent = map && typeof map.title === 'string' ? map.title : 'Waiting for Quaver';
    const acc = last && last.score ? Number(last.score.accuracy) : NaN;
    $('acc').textContent = Number.isFinite(acc) ? 'Last play: ' + acc.toFixed(2) + '%' : '';
  }

  function on(es, type, apply) {
    es.addEventListener(type, (ev) => {
      try {
        apply(JSON.parse(ev.data).data);
        render();
      } catch (err) {
        console.warn('Ignored a bad ' + type + ' event', err);
      }
    });
  }

  function connect() {
    const es = new EventSource('/events');
    es.onopen = () => { $('status').textContent = ''; };
    es.onerror = () => {
      $('status').textContent = 'Reconnecting';
      if (es.readyState === EventSource.CLOSED) {
        es.close();
        setTimeout(connect, 2000);
      }
    };
    on(es, 'snapshot', (d) => { map = d.map; last = d.last_score; });
    on(es, 'map.started', (d) => { map = d; });
    on(es, 'score.saved', (d) => { last = d; });
  }

  render();
  connect();
</script>
</body>
</html>
```

`overlay.json`:

```json
{
  "name": "Mini card",
  "description": "The map title and the accuracy of the last play.",
  "width": 520,
  "height": 90
}
```

Never build HTML from event data with `innerHTML`: titles and names come from map files and may contain any character. Use `textContent`, as the examples do.

In OBS use `http://127.0.0.1:7878/overlays/mini-card/` with a 520 x 90 Browser source. A browser `EventSource` retries after a connection loss on its own. The example also recreates the connection when the browser gives up for good (for example after a server error).

## 7. Example 2: reuse the shared files and the theme

This overlay shows the last result as a card that follows the chosen theme and accent, and supports `?demo=1` to preview without the game. It uses `/overlays/overlay.css`, `qscConnect`, `qscEl`, `qscParams` and `qscModel`.

Save it as `overlays/theme-card/index.html` and `overlays/theme-card/overlay.json`. The files are also in [`docs/examples/theme-card`](examples/theme-card).

`index.html`:

```html
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Theme card</title>
<link rel="stylesheet" href="/overlays/overlay.css">
<script src="/overlays/i18n.js"></script>
<script src="/overlays/model.js"></script>
<script src="/overlays/common.js"></script>
</head>
<body>
<div id="card" class="card hidden"></div>
<script>
  const card = document.getElementById('card');

  function show(result) {
    const score = (result && result.score) || {};
    const map = (result && result.map) || {};
    card.replaceChildren(
      qscEl('div', 'title', typeof map.title === 'string' ? map.title : ''),
      qscEl('div', 'sub', qscModel.formatAccuracy(score.accuracy) + ' ' + qscModel.gradeText(score.grade)));
    card.classList.remove('hidden');
  }

  if (qscParams.demo) {
    show({ score: { accuracy: 98.76, grade: 'S' }, map: { title: 'Example Song' } });
  } else {
    qscConnect({
      'snapshot': (d) => { if (d.last_score) show(d.last_score); },
      'score.saved': show,
    });
  }
</script>
</body>
</html>
```

`overlay.json`:

```json
{
  "name": "Theme card",
  "description": "The last play, drawn with the theme colors.",
  "width": 540,
  "height": 100
}
```

Use the link `http://127.0.0.1:7878/overlays/theme-card/?theme=nord&accent=ff8800`. The card uses the theme tokens through the CSS of `overlay.css`; in your own CSS you can use them directly, for example `background: var(--bg); color: var(--fg); border: var(--border); border-radius: var(--radius);`. The page stays hidden for a moment until the theme file is loaded, to avoid a flash of unstyled content.

## 8. Using the data outside OBS

The routes work from any program on the same computer.

Read the current state with curl (on Windows PowerShell use `curl.exe`):

```
curl http://127.0.0.1:7878/state
```

Follow the live events (`-N` turns off buffering):

```
curl -N http://127.0.0.1:7878/events
```

- **Stream Deck or any HTTP button or plugin:** send a `GET` to `http://127.0.0.1:7878/state` and read `game`, `map.title` or `last_score.score.accuracy` from the JSON.
- **A Twitch bot:** read `/state` when a viewer types a command, or keep `/events` open and remember the last `score.saved`. The bot must run on the same computer, because the companion only listens on `127.0.0.1`.
- **OBS automation:** the companion does not control OBS. If you want a scene switch when `game.state` becomes `gameplay`, write your own script that follows `/events` and sends a `SetCurrentProgramScene` request through the OBS WebSocket server. Nothing like this is built in, and the script and its WebSocket password are yours to manage.

A Node.js script (version 18 or later), saved as `watch.mjs`. It reconnects after an error:

```js
// Prints every event of the companion. Run with: node watch.mjs
const decoder = new TextDecoder();

async function watch(base) {
  const res = await fetch(base + '/events');
  if (!res.ok) throw new Error('HTTP ' + res.status);
  let buffer = '';
  for await (const chunk of res.body) {
    buffer += decoder.decode(chunk, { stream: true });
    let end;
    while ((end = buffer.indexOf('\n\n')) >= 0) {
      const block = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      const data = block.split('\n').filter((l) => l.startsWith('data: ')).map((l) => l.slice(6)).join('\n');
      if (data) console.log(JSON.parse(data).type, data);
    }
  }
}

for (;;) {
  try {
    await watch('http://127.0.0.1:7878');
  } catch (err) {
    console.error('Disconnected:', err.message);
  }
  await new Promise((resolve) => setTimeout(resolve, 2000));
}
```

The same in Python 3 with the standard library only, saved as `watch.py`:

```python
# Prints every event of the companion. Run with: python watch.py
import http.client
import json
import time
import urllib.request

URL = "http://127.0.0.1:7878/events"


def watch():
    data = []
    with urllib.request.urlopen(URL) as res:
        for raw in res:
            line = raw.decode("utf-8").rstrip("\r\n")
            if line.startswith("data: "):
                data.append(line[6:])
            elif line == "" and data:
                event = json.loads("\n".join(data))
                data = []
                print(event["type"], event["data"])


while True:
    try:
        watch()
    except (OSError, http.client.HTTPException, ValueError) as err:
        print("Disconnected:", err)
    time.sleep(2)
```

Both files are also in [`docs/examples/clients`](examples/clients). Replace `7878` with your port.

## 9. Troubleshooting

- **The page is blank in OBS.** Open the link in a normal browser first. Open the link in Chrome or Edge with the developer tools console open to see script errors. Many overlays show nothing until the first event, so also start a map or play one. The built-in overlays accept `demo=1`; your own overlay has to handle that option itself, as example 2 does.
- **The overlay is not listed on the Overlays page.** The folder name must use only lowercase letters, digits and hyphens, must not be a built-in name, and must hold a readable `index.html`. The file size, file count, depth and extension limits of section 5 apply. Reload the dashboard page after adding a folder.
- **A file returns 404.** Check the extension list, the exact name and case, and that the file is a regular file and not a link.
- **Nothing changes after an edit.** In OBS, right-click the source and choose "Refresh cache of current page".
- **`localStorage` or a form fails.** Those are blocked by the sandbox (section 5).
- **`fetch('/api/...')` is blocked.** The `/api/` routes are not readable from a custom overlay. Use `/state` and `/events`.
- **The theme is always `neutral-light`.** Custom overlays cannot read the saved default theme. Pass `?theme=<id>` in the link.
- **403 forbidden host.** Use `127.0.0.1` or `localhost` and the right port in the link. See the Status page and the connection check on the OBS page.
- **No `map.started` or `score.saved`.** Run `qsc doctor`, and check the Status page: the game folder, its log and its score database must be readable.
- **The data looks stale after the computer slept.** Refresh the source. A script of your own can also reconnect when it has seen no event and no `: ping` line for more than 30 seconds.
