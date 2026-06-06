## 1. Define Multilingual Lookup Maps

- [x] 1.1 Add German month name map `monthDE` to `internal/metadata/targetpath.go`
- [ ] 1.2 Add German day name map `dayDE` to `internal/metadata/targetpath.go`
- [x] 1.3 Add German year constant handling to resolver
- [x] 1.4 Add French month name map `monthFR` to `internal/metadata/targetpath.go`
- [ ] 1.5 Add French day name map `dayFR` to `internal/metadata/targetpath.go`
- [x] 1.6 Add Italian month name map `monthIT` to `internal/metadata/targetpath.go`
- [ ] 1.7 Add Italian day name map `dayIT` to `internal/metadata/targetpath.go`

## 2. Update Token Validation

- [x] 2.1 Extend `validTokens` map in `targetpath.go` to include all language-specific tokens: `monat`, `mois`, `mese`, `tag`, `jour`, `giorno`, `jahr`, `année`, `anno`, `monthNumber`, `dayNumber`, plus uppercase variants (`Monat`, `Mois`, etc.)
- [x] 2.2 Create `isValidToken()` function that validates tokens case-insensitively against registry
- [x] 2.3 Update `ResolveTargetPath()` to call validation and reject unknown tokens with clear error message

## 3. Implement Case-Sensitive Token Rendering

- [x] 3.1 Create `normalizeCase()` function that applies case transformation to string based on placeholder case: lowercase, title-case (Monat), UPPERCASE
- [x] 3.2 Extract placeholder case from token name (e.g., `{Monat}` = title-case, `{monat}` = lowercase, `{MONAT}` = uppercase)
- [x] 3.3 Apply case normalization in `ResolveTargetPath()` for all month/day name tokens

## 4. Implement Token Resolution Engine

- [x] 4.1 Refactor `ResolveTargetPath()` to handle all 4 month tokens: `{month}`, `{monat}`, `{mois}`, `{mese}` with correct language maps
- [x] 4.2 Add resolution for all 4 day tokens: `{day}`, `{tag}`, `{jour}`, `{giorno}` with correct language maps
- [x] 4.3 Add resolution for all 4 year tokens: `{year}`, `{jahr}`, `{année}`, `{anno}` (all render as 4-digit year)
- [x] 4.4 Add numeric placeholders: `{monthNumber}` → zero-padded month (02, 03, ... 12)
- [x] 4.5 Add numeric placeholders: `{dayNumber}` → zero-padded day (01, 02, ... 31)
- [x] 4.6 Test that case variants work: `{Monat}` vs `{monat}` vs `{MONAT}` produce correct casing

## 5. Configuration Validation

- [x] 5.1 Add validation function `ValidateTargetStructure()` in `config/validate.go` to check all tokens in targetStructure are recognized
- [x] 5.2 Call validation during config load in `config/loader.go` for each organizer's targetStructure
- [x] 5.3 Return clear error message on invalid token with hint about supported tokens

## 6. Test Coverage

- [x] 6.1 Add unit tests for German month rendering: `{year}/{Monat}` with various dates
- [x] 6.2 Add unit tests for French month rendering: `{year}/{mois}` with various dates
- [x] 6.3 Add unit tests for Italian day rendering: `{year}/{anno}/{mese}/{giorno}` with various dates
- [x] 6.4 Add unit tests for case-sensitivity: `{monat}`, `{Monat}`, `{MONAT}` produce correct casing
- [x] 6.5 Add unit tests for numeric placeholders: `{monthNumber}`, `{dayNumber}` always zero-padded (02, 05, 31)
- [x] 6.6 Add unit tests for mixed tokens: `{year}/{monthNumber}-{Monat}/{dayNumber}` → `2024/02-Februar/05`
- [x] 6.7 Add unit tests for invalid token detection: `{invalidToken}` raises validation error
- [x] 6.8 Add unit tests for edge dates: Feb 5, Feb 29 (leap year), Dec 31, Jan 1
- [ ] 6.9 Add integration test: Full organizer run with multilingual config produces correct folder structure

## 7. Integration Tests & Data Setup

- [x] 7.1 Add test config with German targetStructure: `{year}/{Monat}/{Tag}`
- [x] 7.2 Add test config with French targetStructure: `{year}/{mois}/{jour}`
- [x] 7.3 Add test config with mixed tokens: `{year}/{monthNumber}-{Monat}`
- [x] 7.4 Verify test data generation includes dates that exercise all months and days
- [x] 7.5 Run full integration test with all language variants

## 8. Documentation Updates

- [x] 8.1 Update `README.md`: Add section documenting all supported targetStructure tokens (English, German, French, Italian) with examples
- [x] 8.2 Update `README.md`: Explain case-sensitivity behavior with examples
- [x] 8.3 Update `README.md`: Show example configs for German, French, Italian users
- [x] 8.4 Update `data/config.yaml`: Add commented examples with multilingual tokens
- [x] 8.5 Update `docs/arc42/arc42.adoc`: Document multilingual token support in section on targetStructure resolution
- [x] 8.6 Update `openspec/specs/photo-organizer/spec.md`: Sync delta spec into main spec with all new multilingual requirements

## 9. Verification & Cleanup

- [x] 9.1 Run `go test ./...` to verify all tests pass
- [x] 9.2 Run `go build ./cmd/phorga` to verify no build errors
- [x] 9.3 Run organizer with test configs using all language variants in dry-run mode
- [x] 9.4 Verify log messages show correct folder targets for multilingual paths
- [x] 9.5 Review code for edge cases: uppercase vs lowercase validation, month name variations

## 10. Skills Compliance

- [x] 10.1 Use `always-test-development` skill: Run tests after each implementation step
- [x] 10.2 Use `keep-readme-current` skill: Update README with multilingual examples
- [x] 10.3 Use `keep-arc42-up-to-date` skill: Update arc42 with multilingual token documentation

