# AGENTS.md: Working on Phorga

This is the entry point for everyone who changes Phorga, people and AI agents alike. It explains where things are, how to build and test, and how a change is done. Claude Code loads it automatically through [`CLAUDE.md`](CLAUDE.md).

**Phorga** is a headless photo/video organizer and duplicate detector written in Go. It runs as a daemon (usually in Docker on a NAS), sorts media into date-based folders, finds duplicates across folders, and can optionally serve a small configuration web UI. → [docs/product.md](docs/product.md)

## Documentation map

| I want to … | Read | Audience |
|---|---|---|
| understand what Phorga does and why | [docs/product.md](docs/product.md) | humans |
| install, configure and run it | [README.md](README.md), [config.example.yaml](config.example.yaml) | users |
| know the exact required behavior of a feature | [docs/specs/](docs/specs/README.md) | AI + reviewers |
| understand the architecture, runtime and deployment | [docs/arc42/arc42.adoc](docs/arc42/arc42.adoc) | humans |
| know why something was decided | [docs/decisions/](docs/decisions/README.md) | humans + AI |
| run the local demo | [data/README.md](data/README.md) | developers |

How the layers relate:

- `product.md` explains a feature in prose and links to its spec file.
- Each spec requirement (`ORG-002`, …) links back to `product.md`, to arc42 and to ADRs, and lists the code and tests that implement it.
- Tests name the requirement they cover with `// Spec: ORG-002`.

So `grep -rn ORG-002` finds the requirement, the code reference, and the test.

## Repository layout

```
cmd/phorga/          main → run(ctx, args, logOut): flags, config, scheduler + web UI lifecycle (tested)
internal/
  config/            load (YAML/JSON), defaults, validate, resolve path, save
  organizer/         organizer run: walk sources, resolve date, transfer, duplicates
  dedup/             duplicate detection run: scan, pHash/metadata compare, act
  metadata/          date extraction (EXIF → filename → path → mtime), targetStructure tokens
  fileops/           atomic copy/move, random suffixes
  state/             JSON state per profile target and source (resume)
  scheduler/         cron wrapper with overlap prevention
  notify/            ntfy.sh client (fire-and-forget)
  webui/             HTTP server, form model, i18n, embedded templates
docs/                product, specs, arc42, decisions (+ docs_test.go)
testdata/tools/      generator for demo images in data/
data/                demo config; generated folders are git-ignored
```

Package details and diagrams: [arc42 §5](docs/arc42/arc42.adoc).

## Commands

```bash
go build ./...                                  # build everything
go test ./...                                   # all tests, incl. docs consistency checks
go vet ./...
go run ./cmd/phorga --config data/config.yaml   # run the demo (from repo root)
go run ./cmd/phorga --config data/config.yaml --web-ui --web-ui-addr 127.0.0.1:8080
go run testdata/tools/generate.go               # (re)generate demo images in data/
docker build -t phorga .                        # container image (CI pushes mnrd001/phorga after every merge to main)
```

The demo config uses repo-relative paths and schedules every minute. Stop it with Ctrl+C.

## Conventions

- **Go:** standard `gofmt`, small packages under `internal/`, no CGO (Phorga must cross-compile for ARM NAS).
- **Logging:** `log/slog` with the JSON handler. Use structured keys (`"source", path`), not formatted strings.
- **Safety first:**
  - All file writes are atomic (temp file + rename) via `internal/fileops`.
  - `move` deletes the source only after a successful, verified write.
  - Transfers keep the file's modification time and never overwrite an existing file, except with the duplicate action `replace`. Use `fileops.UniquePath` for new destinations.
  - `dryRun` must never change a file, including state files. The one exception is the duplicate detection fingerprint cache, which never touches a photo ([ADR-0015](docs/decisions/0015-dedup-fingerprint-cache-written-in-dry-runs.md)).
  - An error on one file is logged and counted, and never aborts a run.
  - Jobs get a `context.Context` and MUST check it between files, so shutdown stays fast ([RUN-001](docs/specs/runtime.md)).
- **Config changes** always involve `config.go` (fields), `loader.go` (defaults), `validate.go` (rules), `webui/model.go` + templates (if the field is editable in the UI), `config.example.yaml`, and the README.
- **targetStructure tokens** are defined twice, in `internal/config/validate.go` and `internal/metadata/targetpath.go`. Change both.
- **Naming:** `Phorga` / `phorga` / `PHORGA_`, never the old name ([ADR-0007](docs/decisions/0007-rename-to-phorga.md)).
- **Git:** conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`). Use a feature branch and open a PR to `main`. Direct pushes to `main` are blocked by a ruleset, and only @zedage (repo admin) can merge. CI ([ci.yml](.github/workflows/ci.yml)) runs build, vet and test on every PR and must be green. Code owners are requested as reviewers automatically ([.github/CODEOWNERS](.github/CODEOWNERS)).
- **Docs language:** English.

## Change workflow

Every behavior change follows **spec → code + tests → human docs**.

1. **Spec first.** Find the affected requirement in [`docs/specs/`](docs/specs/README.md), or add a new one with the next free ID. For larger or uncertain changes, add it as `**Status:** proposed` and agree on it before coding. Never renumber or reuse IDs. Retired requirements get `**Status:** removed`.
2. **Code and tests.** Implement the change, then write or update tests that exercise the scenarios. Put `// Spec: <ID>` above each covering test function.
3. **Close the spec.** Set the status to `implemented` and update the `**Code:**` and `**Tests:**` lines. If something is not tested, write `none` and add a `**Gap:**` line.
4. **Human docs.** Update [`docs/product.md`](docs/product.md) if users would notice the change. Update [arc42](docs/arc42/arc42.adoc) if structure, runtime flow or deployment changed. Update the [README](README.md) and `config.example.yaml` if usage or config changed.
5. **Decision?** If you made a real trade-off, add an ADR in [`docs/decisions/`](docs/decisions/README.md) and link it from the spec and from arc42 §9.

Pure refactorings with no behavior change need only step 2 (tests stay green) and possibly updated `Code:` paths.

When the code and a spec disagree, do not silently "fix" either one. Find out which one is intended (ask if unsure), fix the other, and say so in the PR.

## Definition of Done

- [ ] `go build ./...`, `go vet ./...` and `go test ./...` are green (this includes `docs/docs_test.go`)
- [ ] The affected spec requirements are updated, with correct status, scenarios, and `Code`/`Tests` references
- [ ] Tests carry `// Spec: <ID>` comments
- [ ] product.md, arc42, README and `config.example.yaml` are updated where affected
- [ ] An ADR is added if a real trade-off was made
- [ ] The PR description lists the spec IDs that were added or changed

## Notes for AI agents

- Read the relevant spec file **before** changing a feature. It is shorter and more precise than the code and lists the tests to run.
- Skills in [`.claude/skills/`](.claude/skills/) cover the recurring duties: `keep-specs-current`, `always-test-development`, `keep-arc42-up-to-date`, `keep-readme-current`.
- Specs record known open questions in `**Notes:**` lines. Do not resolve them on your own; ask.
- Do not edit generated demo folders under `data/` or commit binaries.
