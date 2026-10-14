# Security policy

## Supported versions

Only the latest release receives security fixes. While the project is in the `0.x` series, that means the newest tag on the [releases page](https://github.com/kushiemoon-dev/quaver-stream-companion/releases). Please update before reporting.

## Reporting a vulnerability

Please do not open a public issue for a security problem. Use GitHub private vulnerability reporting instead: open the repository Security tab, choose Report a vulnerability, and describe what you found, the version (`qsc version`) and the steps to reproduce it.

This is a small project maintained in spare time. You can expect an acknowledgement within about a week and a fix or a clear answer as soon as the maintainer can manage it. There is no bounty program.

## In scope

- The local server: the HTTP routes, the write API and its Host and Origin checks, the event stream.
- File serving: the `/themes/` and `/overlays/` routes, including path handling for custom themes and overlays.
- Key sampling: the lane key frames (`keys` in `/state`, `GET /events?keys=1`) must stay readable only by pages served by qsc itself and by tools that send no Origin header, such as OBS and curl.
- The release archives, their checksums and the build workflows.

## Out of scope

- Attacks that need control of the computer or the account that runs qsc, or write access to its config, themes or overlays folders.
- Exposing qsc to a network on purpose: it binds to 127.0.0.1 and is not meant to be reachable from other machines.
- Vulnerabilities in Quaver, OBS, browsers or other software that qsc only reads from or talks to.
- Findings in third party modules that do not affect how qsc uses them (report those upstream).
