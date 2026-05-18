## Context

The project is named "Phorganizer" across all code, config, docs, and Docker artifacts. A full rename to "Phorga" is needed to avoid conflicts with existing solutions. The rename touches every layer but changes zero behavior.

## Goals / Non-Goals

**Goals:**
- Rename all occurrences of `phorganizer`/`Phorganizer` to `phorga`/`Phorga`
- Maintain identical behavior — no functional changes
- Keep git history clean (single commit rename)

**Non-Goals:**
- Renaming the GitHub repository (done manually after merge)
- Changing any feature behavior, config schema keys, or file formats
- Backward compatibility with old name (no aliases or redirects)

## Decisions

### D1: Rename Go module path
- **Decision**: Change `github.com/zedage/phorganizer` → `github.com/zedage/phorga` in `go.mod` and all imports
- **Rationale**: Module path must match the repository name. All internal imports update accordingly.
- **Alternative**: Use a `replace` directive — rejected, adds unnecessary complexity for a fresh project.

### D2: Rename cmd directory
- **Decision**: `cmd/phorganizer/` → `cmd/phorga/`
- **Rationale**: Binary name derives from the cmd directory by convention.

### D3: Single-pass rename approach
- **Decision**: Global search-and-replace across all files, then verify with `go build` and `go test`.
- **Rationale**: The project is small enough that a single-pass rename is safe and complete. No staged migration needed.
- **Alternative**: Gradual rename with aliases — rejected, unnecessary for a pre-release project with no external consumers.

### D4: Case-sensitive replacements
- **Decision**: Three replacement passes:
  1. `Phorganizer` → `Phorga` (title case — headings, docs)
  2. `phorganizer` → `phorga` (lowercase — code, paths, imports)
  3. `PHORGANIZER` → `PHORGA` (uppercase — env vars)
- **Rationale**: Covers all naming conventions used in the project.

## Risks / Trade-offs

- **Broken imports after rename** → Mitigated by running `go build ./...` and `go test ./...` after rename.
- **Missed occurrences** → Mitigated by `grep -ri phorganizer` after rename to catch stragglers.
- **GitHub repo URL mismatch** → Repo rename is a manual step after push. Document in tasks.

