# Build your own overlay

An overlay is a web page that OBS shows in a Browser source. QuaverStreamCompanion serves four of them and lets you add your own: a folder with an `index.html` is enough. This guide lists the built-in overlays and their options, describes the data the companion publishes, and walks through two complete examples.

Everything below uses the default port. Replace `7878` if you changed it in the settings.

## 1. The built-in overlays and their options

| Overlay | Link | Recommended size |
|---------|------|------------------|
| Now playing | `http://127.0.0.1:7878/overlays/now-playing/` | 540 x 140 |
| Result | `http://127.0.0.1:7878/overlays/result/` | 520 x 230 |
| History | `http://127.0.0.1:7878/overlays/history/` | 520 x 180 |
| State | `http://127.0.0.1:7878/overlays/state/` | 280 x 64 |

Options go in the query string, for example `/overlays/result/?theme=nord&autohide=20&scale=1.2`. A value that cannot be read is ignored and the default is used (`theme`, `accent`, `lang`, `autohide`, `hide`, `demo`). A number outside its range is clamped to the nearest limit (`scale`, `n`), and a `scale` or `n` that is not a number uses the default.

| Option | Applies to | Values | Default |
|--------|-----------|--------|---------|
| `theme` | all | A theme id: lowercase letters, digits and hyphens | The default saved on the Overlays page, else `neutral-light` |
| `accent` | all | 3 to 8 hex digits without the `#`, for example `ff8800` | The theme accent |
| `lang` | all | `en`, `fr` or `de` | The browser language, else `en` |
| `scale` | all | A number from 0.5 to 3 | 1 |
| `demo` | all | `1` shows example data and does not connect | off |
| `autohide` | now playing, result | Seconds from 1 to 3600 until the card hides | never |
| `n` | history | Number of plays from 5 to 50 | 20 |
| `hide` | state | Comma separated state names (see section 4) for which the badge is hidden, for example `menu,closed` | none |

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
| `GET /api/status` | no, same-origin pages only | Version and health checks |
| `GET /api/overlays` | no, same-origin pages only | The custom overlays folder and its overlays |

"Cross-origin readable" means the response carries `Access-Control-Allow-Origin: *`. A custom overlay runs with an opaque origin (see section 5), so it can use `/state` and `/events` but cannot read the `/api/` routes. Tools outside the browser (curl, scripts, bots) can read all of them.

### GET /state

```json
{
  "game": "menu",
  "map": null,
  "last_score": null
}
```

- `game`: the game state, one of the values listed below. It is `unknown` until the first state is seen.
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
| `snapshot` | The same object as `GET /state`: `game`, `map`, `last_score`. Always the first event. |
| `game.state` | `{ "state": "<value>" }` |
| `map.started` | The map object, sent when gameplay begins |
| `score.saved` | The result object, sent when a play is saved |

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
| `closed` | The game has closed |
| `unknown` | Nothing seen yet (only in a snapshot) |

The game can report other screen names. They are passed through in lower case, so treat any value you do not know as "something else" instead of failing. The built-in overlays show it as `unknown`.

#### The map object

Used by `map.started`, `map` in the snapshot, and `map` in a result.

| Field | Type | Meaning |
|-------|------|---------|
| `md5` | string | MD5 checksum of the map |
| `artist` | string | Artist |
| `title` | string | Title |
| `difficulty_name` | string | Name of the difficulty |
| `difficulty` | number | Star rating. In `map.started` it is the rating at 1.0x. In a result it is the rating at the rate that was played, when the game has it |
| `keys` | number | `4` or `7`, or `0` when unknown |
| `last_played_ms` | number | When the map was last played, in milliseconds since the Unix epoch |

#### The result object

Used by `score.saved` and `last_score`.

| Field | Type | Meaning |
|-------|------|---------|
| `score` | object | See the score object below |
| `map` | object | The map object, with the difficulty at the played rate |
| `is_pb` | boolean | A new personal best on this map. Never true for a failed, paused or autoplay play |
| `first_play` | boolean | The first saved score on this map |
| `full_combo` | boolean | No miss, not failed and not autoplay |
| `failed` | boolean | The play was failed or quit |
| `paused` | boolean | The play was paused |
| `autoplay` | boolean | Autoplay was on |

The score object:

| Field | Type | Meaning |
|-------|------|---------|
| `id` | number | Score id in the game database |
| `grade` | string | `X`, `SS`, `S`, `A`, `B`, `C`, `D`, `F`, or `-` when unknown. A failed play is `F` |
| `accuracy` | number | Percent, for example `97.5` |
| `max_combo` | number | Highest combo |
| `judgments` | object | Counts: `marv`, `perf`, `great`, `good`, `okay`, `miss` |
| `total_score` | number | The score points |
| `played_at` | string | The time of the play as saved by the game, `YYYY-MM-DD HH:MM:SS` |
| `mods` | array of strings | Names of the active mods. The rate is not in this list |
| `mods_raw` | number | The raw mod bit field of the game |
| `rate` | number | Playback rate, `1` for normal speed |
| `rating` | number | Performance rating of the play |

#### What is not available

There is no per-note data and no live judgment, combo or accuracy while a map is being played. The companion reads only the game log and the score database, and a score is written to the database when the play ends. So you get the map when it starts and the full result when it is saved, but nothing in between. An overlay that needs live judgments cannot be built on this data.

### GET /api/history

For pages served by the companion itself (the built-in History overlay uses it). `limit` is an optional query parameter, from 1 to 200, default 50; anything else gives a 400 with `{"error": "...", "field": "limit"}`. The answer is an array, newest first, of:

```json
{
  "id": 1,
  "played_at": "2026-10-12 13:06:06",
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
  "autoplay": false
}
```

The fields mean the same as in the result object, but the map and score fields are flattened into one object.

### GET /api/status

Also same-origin only. It returns the keys `version`, `game`, `map`, `last_score` (as in the snapshot), `checks`, `config_path` and `uptime_s`. Each entry of `checks` is `{ "id", "ok", "detail" }`, with `id` one of `quaver_dir`, `log`, `database` and `port`. Use it to verify that the companion is healthy.

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

- The folder name uses only lowercase letters, digits and hyphens. A name used by a built-in overlay (`now-playing`, `result`, `history`, `state`) is ignored: the built-in one always wins.
- `index.html` is required. At most 100 overlays are listed.
- An overlay holds at most 200 files and folders, nested at most 3 levels deep (for example `index.html`, `css/style.css`, `img/icons/star.svg`).
- A file is at most 2 MiB. Names use only letters, digits, `.`, `_` and `-`, and do not start with a dot.
- Allowed extensions: `.html`, `.htm`, `.css`, `.js`, `.mjs`, `.json`, `.svg`, `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.ico`, `.woff2`, `.ttf`, `.txt`. Other files are not served.
- Only regular files are served. Links (symbolic links) are refused, and the name must match the file on disk exactly, including its case.
- Only `GET` is accepted. The link `/overlays/mini-card` redirects to `/overlays/mini-card/`.

### The sandbox

Each file of a custom overlay is served with `Content-Security-Policy: sandbox allow-scripts`. The page runs scripts, but it has an opaque origin. In practice:

- `fetch('/state')` and `new EventSource('/events')` work, because those routes allow cross-origin reads.
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
- `/overlays/i18n.js`, `/overlays/model.js` and `/overlays/common.js`, in that order. `common.js` reads the options of section 1 (`theme`, `accent`, `lang`, `scale`, `demo`, `autohide`, `hide`, `n`), loads the theme file and sets the tokens `--bg`, `--fg`, `--muted`, `--accent`, `--border`, `--radius`, `--shadow`, `--font` and `--scale` on the page. It provides:
  - `qscConnect(handlers)`: opens `/events` and calls `handlers[type](data)` with the `data` of each event. It returns the `EventSource`. It does not catch errors: a payload that cannot be parsed throws inside the handler, and the handler's own errors are yours to catch.
  - `qscEl(tag, className, text)`: creates an element and sets its text with `textContent`.
  - `qscParams`: the parsed options `{ demo, lang, scale, autohide, hide, n }`.
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
