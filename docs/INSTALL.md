# Install QuaverStreamCompanion

`qsc` is a single program with no installer and no dependencies. Releases are published on the [releases page](https://github.com/kushiemoon-dev/quaver-stream-companion/releases).

## Files in a release

| File | For |
|------|-----|
| `qsc_<version>_windows_amd64.zip` | Windows 10 or newer, 64 bit |
| `qsc_<version>_linux_amd64.tar.gz` | Linux, 64 bit Intel or AMD |
| `qsc_<version>_linux_arm64.tar.gz` | Linux, 64 bit ARM |
| `qsc_<version>_linux_x86_64.AppImage` | Linux, 64 bit Intel or AMD, no extraction needed |
| `checksums.txt` | SHA-256 of every file above |

Each archive holds the program, the license, the README, the guides in `docs/` and, on Linux, the `qsc.service` file.

## Download and verify

Put the file you want and `checksums.txt` in the same folder, then check the file.

Windows (PowerShell), replacing the file name:

```powershell
(Get-FileHash .\qsc_v0.1.0_windows_amd64.zip -Algorithm SHA256).Hash.ToLower()
Select-String qsc_v0.1.0_windows_amd64.zip checksums.txt
```

The two hashes must be identical.

Linux:

```sh
sha256sum --ignore-missing -c checksums.txt
```

The line for your file must say `OK`.

## Run

Windows: unzip the archive anywhere you like, for example `C:\Tools\qsc`, then open a terminal in that folder:

```
.\qsc.exe doctor
.\qsc.exe --open
```

Linux archive:

```sh
tar -xzf qsc_v0.1.0_linux_amd64.tar.gz
cd qsc_v0.1.0_linux_amd64
./qsc doctor
./qsc --open
```

`qsc doctor` checks that the Quaver folder is found. `qsc --open` starts the server and opens the dashboard at `http://127.0.0.1:7878/`. Without `--open`, `qsc` never opens a browser. `qsc serve` is the same as `qsc`. See the README for `--port`, `--config` and `--quaver-dir`.

## AppImage

```sh
chmod +x qsc_v0.1.0_linux_x86_64.AppImage
./qsc_v0.1.0_linux_x86_64.AppImage
```

Double clicking it in a file manager works too, no terminal needed. It runs `qsc serve --open`, so the dashboard opens in your browser. Extra options can be added after the file name, for example `--port 7979`. AppImages need FUSE 2 on most distributions; if yours has none, run it with `--appimage-extract-and-run`.

**Untested so far.** The AppImage is built by the release workflow on GitHub. The author builds on Windows and has not run this AppImage on a Linux machine yet, and the release workflow itself has not been run before the first tag. If it does not start for you, use the `.tar.gz` and please open an issue.

## Start with your computer

### Windows

1. Press `Win+R`, type `shell:startup` and press Enter. The Startup folder opens.
2. Right click inside it, choose New, then Shortcut.
3. For the location, enter the full path to `qsc.exe` followed by ` serve`, for example `"C:\Tools\qsc\qsc.exe" serve`. Finish the wizard.
4. Right click the new shortcut, choose Properties, and set Run to `Minimized`. Click OK.

The terminal window of `qsc` then starts minimized at each login. The dashboard is not opened by itself because the shortcut has no `--open`. To stop `qsc`, close its window from the taskbar.

### Linux (systemd user service)

Copy the program somewhere permanent, for example `~/.local/bin/qsc`, and copy `qsc.service` from the archive:

```sh
mkdir -p ~/.local/bin ~/.config/systemd/user
cp qsc ~/.local/bin/qsc
cp qsc.service ~/.config/systemd/user/qsc.service
```

The unit runs `%h/.local/bin/qsc serve`, where `%h` is your home folder. If the program is elsewhere, edit the `ExecStart=` line. Add options on that line if you need them, for example `ExecStart=%h/.local/bin/qsc serve --quaver-dir /path/to/Quaver`. Then:

```sh
systemctl --user daemon-reload
systemctl --user enable --now qsc.service
systemctl --user status qsc.service
journalctl --user -u qsc.service
```

The service does not open a browser. If Quaver is not found yet when you log in, `qsc` keeps looking for it and picks it up within 15 seconds once it appears. These unit instructions have not been tried on a real Linux machine by the author.

## Update

Stop `qsc`, replace the program (or the AppImage) with the new one, start it again. Your settings are kept because they live in the config folder, not next to the program. A change of version is shown by `qsc version`.

## Where files live

| What | Windows | Linux |
|------|---------|-------|
| Program | where you unzipped it | where you copied it |
| Config | `%APPDATA%\quaver-stream-companion\config.json` | `~/.config/quaver-stream-companion/config.json` |
| Custom themes and overlays | next to the config file | next to the config file |

`qsc` writes nothing to the Quaver folder.

## Uninstall

1. Stop `qsc`.
2. Windows: delete the shortcut from the `shell:startup` folder and the folder you unzipped. Linux: run `systemctl --user disable --now qsc.service`, then delete `~/.config/systemd/user/qsc.service` and the program or AppImage.
3. To also remove your settings, delete the `quaver-stream-companion` folder shown in the table above.
