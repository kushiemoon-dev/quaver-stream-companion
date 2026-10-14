# Releasing

This page is for maintainers.

## Cut a release

1. Make sure `main` is green: the `ci` workflow passes on Ubuntu and Windows, and `gofmt -l .`, `go vet ./...` and `go test ./... -count=1` pass locally.
2. In `CHANGELOG.md`, move the entries under `Unreleased` to a new section `## [<version>] - <date>` (the version without the leading `v`) and update the links at the bottom.
3. Write the release notes in `docs/releases/<tag>.md`, for example `docs/releases/v0.1.0.md`. Start from the changelog entry and the previous notes file. Keep them in English, without dashes. Check that the file is not empty (`test -s docs/releases/v0.1.0.md`).
4. Commit those two files and push `main` first, so the notes file is in the commit the tag points to. Then create and push an annotated tag. The tag must look like `v0.1.0` or `v0.1.0-rc2`:

   ```sh
   git push origin main
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

5. The `release` workflow runs on the tag. It runs the tests, builds the Windows zip, the Linux archives and the AppImage, writes `checksums.txt`, and creates a **draft** release. A tag with a hyphen (for example `v0.1.0-rc2`) is marked as a pre-release.
6. The workflow reads `docs/releases/<tag>.md` as the release notes when that file exists and is not empty, and falls back to generated notes otherwise. Open the draft on GitHub, read the notes, check the assets and `checksums.txt`, then publish it. Edit the notes in the draft if something is wrong, and fix `docs/releases/<tag>.md` afterwards so the repository stays in sync.

The tag is validated by the workflow before any path is built from it. A tag that does not match `v<major>.<minor>.<patch>` with an optional suffix stops the release.

## Repository settings to check

These are set by hand on GitHub. Nothing in this repository changes them.

- Enable private vulnerability reporting (Settings, Code security): `SECURITY.md` and the issue template link to it.
- Set the topics (for example `quaver`, `obs`, `stream-overlay`, `go`) and the homepage in the About box of the repository.
- Add a branch protection rule for `main` that requires a pull request and the status checks of the `ci` workflow. The workflow has one job, `test`, with an operating system matrix, so GitHub offers two checks: `test (ubuntu-latest)` and `test (windows-latest)`. Require both (they only appear in the list after the workflow has run once).
- Keep Actions limited to the default read-only token: the workflows ask for the write permission they need.
- Keep Dependabot alerts and security updates on.
