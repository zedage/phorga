# ADR-0013: Folder picker limited to configured roots, read-only, off by default

- **Status:** accepted
- **Date:** 2026-11-11
- **Specs:** WEB-010, CFG-006

## Context
Typing container paths such as `/photos/organized` by hand is error-prone. A browser's own folder dialog does not help, because it shows the folders of the computer running the browser, not those of the NAS or the container. A useful picker has to list folders on the server. The web UI has no authentication (WEB-006), so anyone who can reach it could use such a listing to look around the filesystem.

## Decision
- The server lists folders through `GET /ui/fs/list` and `GET /ui/fs/suggest`, used by a "Browse…" dialog and by autocomplete on every path field.
- Listing is limited to the folders in the new `webUI.browseRoots`. Paths are checked after `..` cleanup and symlink resolution, and anything outside the roots gets `403`.
- Without `browseRoots`, the picker is **off**: the controls are hidden and the endpoints answer `404`. There is no fallback to `/` or to folders derived from the config.
- The picker is **read-only**. It cannot create folders. Missing target folders are created by the organizer on its first run anyway.
- Only folder names are returned, never file names or contents. Hidden (dot) folders are not shown.

## Consequences
- Users must set `browseRoots` once (in Docker, typically the mounted media folder such as `/photos`) before the picker appears.
- Someone who can reach the UI can see the folder tree below the roots, but nothing outside them, and cannot change anything.
- Folder names are exposed. This is acceptable for the intended local or trusted network.

## Alternatives considered
Listing the whole filesystem was rejected as too open for an unauthenticated UI. Deriving the roots from the config paths was rejected, because it makes choosing a new location impossible. A "New folder" button was rejected so that the UI never writes to the media filesystem.
