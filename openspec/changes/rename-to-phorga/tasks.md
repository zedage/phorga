## 1. Go Module & Imports

- [x] 1.1 Rename module path in `go.mod` from `github.com/zedage/phorganizer` to `github.com/zedage/phorga`
- [x] 1.2 Update all import paths in `internal/` packages
- [x] 1.3 Update all import paths in `cmd/phorganizer/main.go`

## 2. Directory Rename

- [x] 2.1 Rename `cmd/phorganizer/` to `cmd/phorga/`

## 3. Code References

- [x] 3.1 Replace `PHORGANIZER_CONFIG` env var with `PHORGA_CONFIG` in `internal/config/resolve.go`
- [x] 3.2 Replace all string literals containing `phorganizer` / `Phorganizer` in Go source files

## 4. Configuration & Docker

- [x] 4.1 Update `Dockerfile` (binary name, paths)
- [x] 4.2 Update `docker-compose.yml` (image name, container name)
- [x] 4.3 Update `config.example.yaml` (any references)
- [x] 4.4 Update `data/config.yaml` and `data/config2.yaml`

## 5. Documentation

- [x] 5.1 Update `README.md` (project name, commands, paths)
- [x] 5.2 Update `docs/arc42/arc42.adoc` (all Phorganizer references)
- [x] 5.3 Update `openspec/specs/phorganizer.md` (rename file to `phorga.md`, update content)
- [x] 5.4 Update `data/README.md`

## 6. Test Data

- [x] 6.1 Update `testdata/tools/generate.go` (any phorganizer references)
- [x] 6.2 Update `testdata/tools/checkexif.go` (any phorganizer references)

## 7. State & Misc

- [x] 7.1 Update `.gitignore` (binary name)
- [x] 7.2 Update `go.sum` via `go mod tidy`

## 8. Verification

- [x] 8.1 Run `grep -ri phorganizer` to confirm zero remaining occurrences
- [x] 8.2 Run `go build ./...` to verify build
- [x] 8.3 Run `go test ./...` to verify all tests pass
- [x] 8.4 Run `go run testdata/tools/generate.go` to verify test data generation

