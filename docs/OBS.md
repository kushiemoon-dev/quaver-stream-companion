# OBS setup guide

The overlays are web pages served by `qsc`. In OBS each one is a Browser source. The dashboard has the same guide on its OBS page.

## Steps

1. **Keep qsc running.** Start `qsc` (or `qsc --open` to also open the dashboard) and leave it running while you stream. To start it with your computer, see [Run at startup](../README.md#run-at-startup).
2. **Add a Browser source.** In OBS, click the plus under Sources, then choose Browser.
3. **Paste the overlay link.** On the dashboard Overlays page, pick an overlay and a look, copy its link and paste it in the URL field.
4. **Set the width and height** to the recommended size of the overlay (see the table below), so nothing is clipped.
5. **Check the settings.**
   - FPS: 30 is enough.
   - Leave "Shutdown source when not visible" unchecked, so the overlay stays in sync with the game.
   - "Refresh browser when scene becomes active" is optional.
   - No custom CSS is needed: leave the field as OBS filled it in.
6. **Position and layer the sources.** Add one Browser source per overlay and arrange them in your scene. For example: the result card next to the game capture, the history below it, and the state badge in a corner.

## Overlays and recommended sizes

| Overlay | Size (width x height) | Link without options |
|---------|-----------------------|----------------------|
| Now playing | 540 x 140 | `http://127.0.0.1:7878/overlays/now-playing/` |
| Result | 520 x 230 | `http://127.0.0.1:7878/overlays/result/` |
| History | 520 x 180 | `http://127.0.0.1:7878/overlays/history/` |
| State | 280 x 64 | `http://127.0.0.1:7878/overlays/state/` |
| Alert | 520 x 120 | `http://127.0.0.1:7878/overlays/alert/` |
| Lane keys | 480 x 200 (560 x 200 for 7K, 640 x 200 for 10K) | `http://127.0.0.1:7878/overlays/keys/` |
| Judgment curve | 520 x 180 | `http://127.0.0.1:7878/overlays/curve/` |
| Session | 480 x 170 | `http://127.0.0.1:7878/overlays/session/` |

Replace the port if you changed it. Options such as `scale`, `autohide` or `lang` are added by the Overlays page. The "Preview with demo data" switch only changes the preview in the dashboard: the link you copy never contains `demo=1`.

### Lane keys overlay

The lane keys overlay needs the Keys card of the dashboard Settings page: turn it on, pick your keys and restart `qsc`. Lane keys are off by default. Only the lane keys you choose are read, and only while a map is being played. Nothing is stored or logged: no key names, no key history. The key state is read system wide, regardless of the focused window, so lane key timing may be sampled while you type elsewhere during gameplay. It works on Windows only for now, and a tap shorter than the polling interval may be missed. The key data is only served to pages loaded from `qsc` and to OBS; other web pages cannot read it. Until then the overlay shows a short hint saying why it is empty.

## Text source, read from file

An OBS Text source can show the map and the last score without a browser source, because it can read a text file. This is off by default.

1. **Turn it on.** In the dashboard Settings page, tick **Write the now playing text files**. It applies right away, no restart. The same setting is `"text_files": true` in the config file.
2. **Let qsc write the files.** The files are created when you tick the box and then follow the next map or score. With no map known they are empty.
3. **Add a Text source.** In OBS, click the plus under Sources, choose Text, tick **Read from file** and pick a file of the `text` folder next to the config file (`%APPDATA%quaver-stream-companion	ext` on Windows, `~/.config/quaver-stream-companion/text` on Linux). Use the `nowplaying.txt` file for a single line.
4. **Style it in OBS.** Font, color and size are the Text source settings; qsc only writes the words.

| File | Content | Example |
|------|---------|---------|
| `title.txt` | Map title | `Some Title` |
| `artist.txt` | Map artist | `Some Artist` |
| `difficulty.txt` | Difficulty name | `Insane` |
| `mapper.txt` | Map creator | `Some Mapper` |
| `nowplaying.txt` | Artist, title and difficulty | `Some Artist - Some Title [Insane]` |
| `last_grade.txt` | Grade of the last score | `S` |
| `last_accuracy.txt` | Accuracy of the last score, with a dot | `97.42%` |

The files are UTF-8 without a byte order mark and without a trailing newline. Control characters are removed and each file holds at most 256 bytes. A file is only rewritten when its text changes, through a temporary file that replaces it, so OBS never reads half a line.

The same texts are available over HTTP for tools and browser sources of your own: `http://127.0.0.1:7878/api/text/nowplaying.txt` answers `text/plain; charset=utf-8` with the text of the file of the same name, whether or not the files are written. It answers 404 for any other name and is only meant for pages served by `qsc` and local tools.

## Troubleshooting

- **Nothing shows.** Use the "Check connection" button on the OBS page and look at the Status page. The port in the link must match the one in Settings, and the host must be `127.0.0.1` or `localhost`.
- **The old style stays after editing a custom theme.** Right-click the source in OBS and choose "Refresh cache of current page".
- **The Pixel shadow looks blurry or clipped.** Make the browser source larger than the recommended size, so the shadow has room.
- **OBS runs on another PC.** The companion only listens on `127.0.0.1` and only answers requests for `127.0.0.1` or `localhost`, so run OBS on the same machine as `qsc`.
