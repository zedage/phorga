## Why

The project is currently named "Phorganizer" but this name conflicts with existing photo organizing solutions. Renaming to "Phorga" avoids confusion, establishes a unique identity, and should be done now before the project gains users or external references.

## What Changes

- **BREAKING**: Rename all references from `phorganizer` / `Phorganizer` to `phorga` / `Phorga`
- Rename Go module from `github.com/zedage/phorganizer` to `github.com/zedage/phorga`
- Rename binary from `phorganizer` to `phorga`
- Rename `cmd/phorganizer/` directory to `cmd/phorga/`
- Update all import paths across the codebase
- Update Docker image name, config examples, documentation, and arc42
- Update GitHub repository name (manual step)

## Capabilities

### New Capabilities

_(none — this is a rename, not a feature change)_

### Modified Capabilities

_(no spec-level behavior changes — only naming/branding updates)_

## Impact

- **Code**: All Go files with import paths, `go.mod` module path
- **Build**: `cmd/phorganizer/` → `cmd/phorga/`, binary name, Dockerfile
- **Config**: `config.example.yaml`, `data/config.yaml`, documentation references
- **Docs**: `README.md`, `docs/arc42/arc42.adoc`, all openspec specs and archived changes
- **Docker**: image name in `Dockerfile` and `docker-compose.yml`
- **Repository**: GitHub repo URL (manual rename after code changes)

