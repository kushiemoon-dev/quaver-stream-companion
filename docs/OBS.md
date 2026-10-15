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

Replace the port if you changed it. Options such as `scale`, `autohide` or `lang` are added by the Overlays page.

### Lane keys overlay

The lane keys overlay needs the Keys card of the dashboard Settings page: turn it on, pick your keys and restart `qsc`. Lane keys are off by default. Only the lane keys you choose are read, and only while a map is being played. Nothing is stored or logged: no key names, no key history. The key state is read system wide, regardless of the focused window, so lane key timing may be sampled while you type elsewhere during gameplay. It works on Windows only for now, and a tap shorter than the polling interval may be missed. The key data is only served to pages loaded from `qsc` and to OBS; other web pages cannot read it. Until then the overlay shows a short hint saying why it is empty.

## Troubleshooting

- **Nothing shows.** Use the "Check connection" button on the OBS page and look at the Status page. The port in the link must match the one in Settings, and the host must be `127.0.0.1` or `localhost`.
- **The old style stays after editing a custom theme.** Right-click the source in OBS and choose "Refresh cache of current page".
- **The Pixel shadow looks blurry or clipped.** Make the browser source larger than the recommended size, so the shadow has room.
- **OBS runs on another PC.** The companion only listens on `127.0.0.1` and only answers requests for `127.0.0.1` or `localhost`, so run OBS on the same machine as `qsc`.
