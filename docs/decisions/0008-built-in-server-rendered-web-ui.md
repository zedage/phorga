# ADR-0008: Optional built-in, server-rendered web UI with HTMX

- **Status:** accepted
- **Date:** 2026-06-06
- **Specs:** WEB-001 … WEB-006

## Context
Editing YAML on a NAS is inconvenient. Users asked for a simple UI to view and edit the configuration, in German, English, Italian and French.

## Decision
- The UI runs as an optional HTTP server **inside the existing `phorga` process**. It is off by default.
- Pages are **rendered on the server** with Go `html/template`. **HTMX** handles partial updates for validate and save. There is no SPA and no JSON API.
- Form data maps to a separate **form/view model** (`internal/webui/model.go`), not directly to `config.Config`.
- Translations live in **server-side message catalogs** keyed by message ID, with one template for all languages.
- v1 is for **local or trusted networks only**: there is no authentication, and the default bind address is `127.0.0.1:8080`.

## Consequences
- It stays one binary, with no frontend build chain.
- Anyone who can reach the port can change the config. Exposing it publicly needs an external auth proxy.
- HTMX is loaded from a CDN. Offline, the forms still work as full-page POSTs.

## Alternatives considered
A separate admin service or sidecar was rejected because of the extra operational effort. An SPA with a JSON API was rejected as too heavy. Mandatory authentication was deferred to keep v1 small.
