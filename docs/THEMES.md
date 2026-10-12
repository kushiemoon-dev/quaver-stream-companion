# Themes

A theme is a small CSS file that sets a few variables (tokens) on `:root`. The overlay reads those tokens for its colors, border, corners, shadow and font, so one file changes the whole look. QuaverStreamCompanion ships 11 built-in themes and also loads your own.

## Tokens

| Token | Meaning | Example |
|-------|---------|---------|
| `--bg` | Card background | `#1e1e2e` |
| `--fg` | Main text color (title, artist) | `#cdd6f4` |
| `--muted` | Secondary text color (difficulty, details) | `#a6adc8` |
| `--accent` | Highlight color (stars, key count). Can be overridden per source with `?accent=` | `#cba6f7` |
| `--border` | Full border shorthand: width, style and color | `1px solid #45475a` |
| `--radius` | Corner radius of the card | `12px` |
| `--shadow` | Card shadow, or `none` | `0 2px 10px rgb(0 0 0 / 40%)` |
| `--font` | Font stack | `system-ui, "Segoe UI", sans-serif` |
| `--scale` | Optional size multiplier for the whole card, `1` when missing | `1.3` |

## Minimal example

```css
:root {
  --bg: #1e1e2e;
  --fg: #cdd6f4;
  --muted: #a6adc8;
  --accent: #cba6f7;
  --border: 1px solid #45475a;
  --radius: 12px;
  --shadow: 0 2px 10px rgb(0 0 0 / 40%);
  --font: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
}
```

## Where to put your themes

Create a `themes` folder next to your `config.json`:

| OS | Folder |
|----|--------|
| Windows | `%APPDATA%\quaver-stream-companion\themes\` |
| Linux | `~/.config/quaver-stream-companion/themes/` |

The exact folder is shown on the **Overlays** page of the dashboard.

Rules:

- The file is named `my-theme.css`, and its id is `my-theme`.
- The id uses only lowercase letters, digits and hyphens.
- A file is a regular file of up to 64 KiB. At most 100 files are listed.
- A file with the same id as a built-in theme replaces that built-in theme.

Files are read each time they are requested, so edits show up on the next load. Refresh the OBS browser source (right-click it, then Refresh, or use "Refresh cache of current page") to see a change. If a default theme file is deleted, the overlay falls back to `neutral-light`.

## Using a theme

- In OBS, add `?theme=<id>` to the browser source URL, for example `http://127.0.0.1:7878/overlays/now-playing/?theme=my-theme`.
- Or pick a theme on the Overlays page and use "Save as default". Sources without `?theme=` then use that default.

Size the OBS browser source larger than the card, at least 540 x 140 for the default card, so a large shadow (like the hard shadow of the Pixel theme or the glow of Neon Cosy) is not clipped.

## Contrast

Readable text matters on a stream. Aim for at least 4.5:1 for `--fg` on `--bg`, and 3:1 for `--muted` and `--accent` on `--bg` (WCAG AA).

## Privacy

A custom theme is your own CSS. An `@import` or a `url()` that points to a remote host makes OBS fetch that address every time the overlay loads, which lets that host see your requests. Prefer local-only themes with no remote resources.

## Fonts

Custom themes cannot ship font files in this version. Use fonts installed on your system, or the system font stack shown above.

## Contributing a built-in theme

Open a pull request that:

1. Adds `web/themes/<id>.css` with the eight tokens (`--scale` is optional).
2. Adds its display name to the map in `web/dashboard/pages/theme-model.js` and its id to the order list in `internal/themes`.
3. Meets the contrast requirements enforced by the tests.
4. Credits the source palette, with its license, in this file and in the README.

Built-in themes must not use remote resources.

## Credits and licenses

| Theme | Palette | License |
|-------|---------|---------|
| Catppuccin Mocha, Catppuccin Latte | [Catppuccin](https://github.com/catppuccin/catppuccin) | MIT |
| Tokyo Night | [Tokyo Night](https://github.com/folke/tokyonight.nvim) | MIT |
| Dracula | [Dracula](https://draculatheme.com) | MIT |
| Nord | [Nord](https://www.nordtheme.com) | MIT |
| Gruvbox | [Gruvbox](https://github.com/morhetz/gruvbox) | MIT |
| Rose Pine | [Rose Pine](https://rosepinetheme.com) | MIT |
| Neutral Light, Neutral Dark | Original | MIT (this project) |
| Pixel | Original palette, font VT323 by Peter Hull | SIL Open Font License 1.1 (font) |
| Neon Cosy | Original | MIT (this project) |

The Pixel font license is in [`web/themes/fonts/OFL.txt`](../web/themes/fonts/OFL.txt).
