# ADR-0023: Optional web UI password from the environment, cross-site writes rejected

- **Status:** accepted
- **Date:** 2026-11-21
- **Specs:** WEB-006, WEB-020, WEB-021, WEB-022

## Context
ADR-0008 left the web UI without authentication and expected it to be reached from the same machine only. Two things changed. First, a NAS has no browser: on a Synology the UI has to be reached from another device on the network, or it cannot be used at all. Second, since ADR-0022, a saved configuration applies at once, and the YAML editor (WEB-018) accepts any configuration. Anyone who can reach the port can therefore make Phorga move files between any mounted folders within seconds.

Independently of the network, a web page open in the user's browser could post a form to the UI (a plain form POST needs no CORS permission) and change the configuration through the user's browser.

## Decision
- **One optional password**, read only from the environment variable `PHORGA_WEB_UI_PASSWORD` at start-up. If it is set, every request needs HTTP Basic credentials with that password; the user name is ignored. It never appears in the config file, the form or the YAML editor, so the UI cannot change or reveal it. Changing it needs a restart.
- **Basic auth**, not a login form with sessions. It needs no cookies, session store or logout handling, and it works unchanged behind a reverse proxy.
- **Cross-site writes are rejected** with Go's `http.CrossOriginProtection` (`Sec-Fetch-Site`, falling back to `Origin` vs. `Host`), with or without a password. Browsers send stored Basic credentials with cross-site requests, so the password alone would not stop this.
- Without a password, Phorga logs a warning at start-up when the UI listens on more than loopback (always the case inside Docker).
- The guide (WEB-022) keeps the container port on `127.0.0.1` on a Synology and opens it to the network through the DSM reverse proxy, which adds HTTPS. On another computer, it publishes the port on the home network.

## Consequences
- A Synology user can reach the UI from a laptop or phone with HTTPS and a password, using only DSM settings.
- Over plain HTTP, the password crosses the network readable. The guide recommends HTTPS and accepts plain HTTP only in a trusted home network.
- There is a single shared password, no user accounts and no lockout after failed attempts. The guide asks for a long password and never to expose the port to the internet.
- Tools that post without browser headers (`curl`) are not affected by the cross-site check.

## Alternatives considered
- **No login, network restriction only** (DSM access control profile): keeps Phorga smaller, but everyone on the LAN, including guests and compromised devices, could change what Phorga moves.
- **Password in the config file:** the UI and the YAML editor would show and change it, and a hash format would be needed.
- **Login form with session cookie:** nicer to look at, but needs session handling, a CSRF token and logout, for no gain in protection over Basic auth with HTTPS.
- **Relying only on the DSM reverse proxy:** DSM's reverse proxy has no password option, only IP rules.
