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

Each archive holds the program, the license, the third party notices, the README, the guides in `docs/` and, on Linux, the `qsc.service` file.

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

## First launch on Windows

Windows may stop the program the first time because it is unsigned. A code signing certificate costs money every year and this is a free, spare time project, so the binary is not signed. Check that the download is the real one with the SHA-256 steps above, then:

1. Before you unzip: right click the zip, choose Properties, tick Unblock at the bottom of the General tab, click OK. This removes the "downloaded from the internet" mark from every file inside.
2. If a blue SmartScreen window says Windows protected your PC, click More info, then Run anyway.
3. Double click `qsc.exe`. A console window stays open while qsc runs. If it cannot start, it prints why and waits for Enter so you can read it.
4. Open the dashboard at `http://127.0.0.1:7878/`, go to the OBS page, and add the overlays.

If the port is used, qsc says whether another qsc is running (open its dashboard or run `qsc stop`) or another program holds it (choose another port with `--port` or in the config file, then update your OBS links). A damaged config file is never overwritten: qsc prints its path, and deleting that file resets the settings.

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

## Linux and Proton

Quaver is a Windows game, so on Linux it runs through Proton. Steam still keeps the game folder at `steamapps/common/Quaver` inside a Steam library, and `qsc` finds Quaver by that folder (it must contain `Quaver.exe`). It looks in these Steam folders, plus every local library listed in their `steamapps/libraryfolders.vdf`:

- `~/.steam/steam`
- `~/.local/share/Steam`
- `~/.var/app/com.valvesoftware.Steam/.local/share/Steam` (Flatpak)
- `~/snap/steam/common/.local/share/Steam` (snap)

Symlinked Steam folders are followed once and a folder reached by two paths is only searched once. Library entries that are relative, contain `..`, or point to a network share are ignored. If Quaver is somewhere else, set `quaver_dir` in Settings or the config file, or start `qsc` with `--quaver-dir <folder>`.

Linux file systems tell `Logs` from `logs`. `qsc` first looks for `Logs/runtime.log` and `quaver.db` with the exact spelling Quaver uses on Windows, then falls back to a case-insensitive match (ASCII letters only) for `Logs`, `runtime.log` and `quaver.db` only, reading at most 4096 directory entries.

What is not verified: the author has not run Quaver under Proton and has not confirmed where Proton places `Logs`, `quaver.db`, `Data` or `quaver.cfg`. The detection assumes they sit in the game folder, as in the Windows install. If your install keeps them elsewhere (for example inside the Wine prefix under `steamapps/compatdata`), point `quaver_dir` at the folder that holds them, or open an issue. This layout code is covered by tests on simulated Steam trees, not by a run on a real Linux machine.

Lane key sampling (the opt-in keys overlay) is Windows only for now. On Linux the Settings card and `qsc doctor` report it as not supported, and everything else works. A Linux version based on evdev is not implemented.

## Stopping qsc

- Dashboard: open Settings, press Quit qsc at the bottom, then Quit now. The page says when qsc is stopped and the tab can be closed. The OBS overlays go blank until you start qsc again.
- Command line: `qsc stop` asks the running qsc on this computer to stop and waits until the port is free. It prints `qsc stopped`, or `qsc is not running on port N` when nothing was running. Use `--port` or `--config` if you changed the port. It only talks to `127.0.0.1`.
- Console: press `Ctrl+C` in the terminal window that runs `qsc`.
- Linux service: `systemctl --user stop qsc`. A quit from the dashboard or from `qsc stop` ends the program cleanly, and the unit uses `Restart=on-failure`, so systemd does not restart it. It starts again at your next login or with `systemctl --user start qsc`.
- Windows Startup shortcut: the Quit button or `qsc stop` closes the minimized window by itself. The shortcut is not touched, so `qsc` starts again at your next login. Delete the shortcut from the Startup folder (`shell:startup`) to stop that.

## Update

Quit `qsc` first (the Quit button in Settings, or `qsc stop`), then replace the program (or the AppImage) with the new one, start it again. Your settings are kept because they live in the config folder, not next to the program. A change of version is shown by `qsc version`.

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

What `qsc` leaves behind: that config folder (`config.json`, `qsc.log` and its older copy `qsc.log.1`, and your custom `themes` and `overlays` folders) and, on Linux, the service file you copied. Nothing is written to the registry or to the Quaver folder.

## Reporting a problem

Run `qsc doctor --report` (or press Copy diagnostics on the Status page). It prints the version, your system, the settings without key bindings, the doctor checks, the names of your custom themes and overlays, and the last 50 lines of `qsc.log` and of Quaver's `runtime.log`. Your user folder name is replaced by `~`.

**Review the report before you share it.** The Quaver `runtime.log` excerpt is copied as the game wrote it and can contain your Quaver player name, map titles and replay paths. `qsc` cannot filter those, so read the report and delete any line you do not want to publish before you paste it into a new issue on the repository. Your user folder is replaced by `~` in paths, but other names are kept. If `qsc` crashed it says so and points to `qsc.log`; the report includes that log.

A crash in `qsc` itself, in a background task or while answering a request, is written to `qsc.log` with its stack. Only a crash of the program before the log file is open, or one forced by the system (for example running out of memory), is not.
