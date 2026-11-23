<!--
  [P] marks tasks eligible for parallel execution.
  Stacked on opsx/wide-form-cycle-edges (draft PR #98, itself on #96).
  Recordings are captured with the same ripwire binary as those changes
  (testdata/README.md provenance table).
-->

## 1. Resolution (design D1, D2)

- [ ] 1.1 Add `relativeBaseDir(sourceDir, target string) (string, bool)` to `mapping.go`, and a Python leading-dot branch in `resolveTarget`: longest module prefix not shorter than the anchor, otherwise unresolved, and never external or name-resolved. Widen the `ambiguity-rate` message per D2. Update GoDoc on `resolveTarget` and `mappedModule.ExternalOut`. Verify: `go vet ./...`.
- [ ] 1.2 Table tests: `TestRelativeBaseDir_*` (`.`, `..`, `...`, `.x`, `..b.api`, a root-level file, climbing above the root) and `TestResolveTarget_Relative*` (base edge, bare `..` to the anchor, climb above the root → unresolved with no external count, missing anchor with no ancestor guess, and relative targets not setting name-resolved). Verify: `go test -race -count=1 ./internal/ripwireadapter/ -run 'RelativeBaseDir|ResolveTarget'`.

## 2. Member edges (design D3)

- [ ] 2.1 Extend `pyFromImportRegex` and the scanner's base validation to accept relative bases verbatim. Update the GoDoc that says relative bases are ignored (`memberImport`, `scanPythonMemberImports`, `pythonMemberImportScan`). Verify: `go build ./...`.
- [ ] 2.2 In the `mapToModules` member pass, resolve a relative base through `relativeBaseDir` (exact module match, skip on climb). Update the `mapToModules` GoDoc. Verify: `go vet ./...`.
- [ ] 2.3 Tests: replace the "relative bases ignored" scanner cases with verbatim-relative expectations, add malformed relative bases (`.a..b`, `..x.`) as ignored, and add `TestMapToModules_RelativeMemberEdge` plus `_RelativeMemberUnanchored` and `_RelativeMemberClimb`. Verify: `go test -race -count=1 ./internal/ripwireadapter/ -run 'MemberImport|MapToModules'`.

## 3. Fixture and recordings (design D4)

- [ ] 3.1 Add `testdata/python/src/fixturepy/rel_a/__init__.py` and `rel_b/{__init__.py,api.py}` per D4, with docstrings in the `ring_*` style, and document them in `testdata/FIXTURES.md`. Commit them before capturing. Verify: `ripwire --arch` shows `fixturepy/rel_a ce=1` and `fixturepy/rel_b ca=1 ce=2`.
- [ ] 3.2 Re-capture `recorded/python`, `python-zero-edge` (derived), and `python-mixed` per `testdata/README.md`, twice each with `cmp`, leaving `version.txt` unchanged. Update the README byte table and history line. Verify: the pairs are byte-identical, exit 0, the stderr is as documented, and `deps.xml` lists `<inc t=".."/>` for `rel_b/__init__.py`.
- [ ] 3.3 Update `golden-notes.md` and the Python `golden.json`. Update every test that hardcodes the Python module set, edges, line totals, or file counts. Add a cycle assertion that `[fixturepy.rel_a fixturepy.rel_b]` is derived from the recordings with members, and absent with `nil` members. Verify: `go test -race -count=1 ./...` with ripwire off PATH. The real-binary integration test passes with a PATH wrapper that prints the committed `version.txt`.

## 4. Documentation

- [ ] 4.1 [P] CHANGELOG `Unreleased`: a Fixed entry. Python relative imports now produce edges and are no longer counted external, and a `diff` across the upgrade can report a previously missed cycle as new. Verify: the entry is present.
- [ ] 4.2 [P] Assess README and AGENTS.md (the `cycle-approximation` and `ambiguity-rate` rows, and the project structure). Update them where they describe relative imports or the ambiguity message. Verify: `grep -rn "relative" README.md AGENTS.md` is accurate.
- [ ] 4.3 [P] In `wide-form-cycle-edges/tasks.md` 6.1, record this change as the follow-up. Verify: the link is present.
- [ ] 4.4 File the documentation issue for the user-facing change (the AGENTS.md documentation gate) and record its number here.

## 5. Gates

- [ ] 5.1 CI parity: run every command in `.github/workflows/ci.yml` locally with ripwire off PATH. Verify: all pass, and coverage floors are only raised.
- [ ] 5.2 Run the review council (Adversary, Architect, Guard, Tester). Resolve every REQUEST CHANGES. Verify: all reviewers APPROVE.
- [ ] 5.3 Push `opsx/relative-import-edges` and open a draft PR based on `opsx/wide-form-cycle-edges`, stating that it cannot merge before #98 and #96. Verify: the PR is open.
- [ ] 5.4 After #98 and #96 merge, rebase onto `main`. If Phase B re-captured with a different binary, repeat 3.2–3.3 and 5.1. Mark the PR ready.
