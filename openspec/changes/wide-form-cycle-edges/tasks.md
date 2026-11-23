<!--
  [P] marks tasks eligible for parallel execution.
  Stacked on opsx/bump-ripwire-pin (draft PR #96). Recordings are captured
  with the same ripwire binary as that change (design D5).
-->

## 1. Scanner (design D2)

- [x] 1.1 Add `internal/ripwireadapter/memberimports.go` with `memberImport{Base, Name}`, the pure `scanPythonMemberImports(source string) []memberImport`, and `pythonMemberImportScan(crawlRoot string) (map[string][]memberImport, []metrics.Warning, error)` (denylist skip, `parse-error` degradation, slash-form root-relative keys, per-file sort + dedup). Full GoDoc, including documented limitations. Verify: `go build ./...`.
- [x] 1.2 Add table tests in `memberimports_test.go` (`TestScanPythonMemberImports_*`): single name; multiple names; `as` alias; parenthesized multi-line with trailing comma; backslash continuation; `;`-joined statements; indented (`if TYPE_CHECKING:`); `*` ignored; relative bases (`.`, `..b`, `.x`) ignored; `import a.b` ignored; triple-quoted docstring containing `from x import y` ignored; `#` comment stripped; malformed name rejected. Add `TestPythonMemberImportScan_Fixture` over `testdata/python/src` (keys and members for `wide/consumer.py`, `ring_a/__init__.py`, `cycle_a/__init__.py`) and `_UnreadableFile` (a 0o000 file yields `parse-error`, no entry; skip when running as root). Verify: `go test -race -count=1 ./internal/ripwireadapter/ -run MemberImport`.

## 2. Fixture and recordings (design D5)

- [x] 2.1 Add `testdata/python/src/fixturepy/ring_a/{__init__.py,api.py}` and `ring_b/__init__.py` per D5, with docstrings in the style of `cycle_a`/`cycle_b`. Document them in `testdata/FIXTURES.md`. Verify: `ripwire --arch` on the fixture shows `fixturepy/ring_a ce=2` (fixturepy, ring_b) and `fixturepy/ring_b ca=1 ce=1`.
- [x] 2.2 Re-capture `recorded/python`, `python-zero-edge` (derived per README), and `python-mixed` with the binary and commands `bump-ripwire-pin` used (`testdata/README.md`), twice each with `cmp`. Keep `version.txt` unchanged. Verify: pairs byte-identical, exit 0, empty stderr, and `deps.xml` lists `<inc t="fixturepy"/>` for `ring_a/__init__.py`.
- [x] 2.3 Update `golden-notes.md` hand computations and the Python `golden.json` for the new modules and edges. Verify: goldens regenerate byte-identically on a second run.

## 3. Mapping and assembly (design D3, D4)

- [x] 3.1 Extend `mapToModules` with the `members map[string][]memberImport` parameter. Factor edge insertion into one helper and add the anchored member-edge pass. Update GoDoc on `mapToModules`, `moduleMapping`, and `mappedModule` (the Internal* counters now include member edges). Pass `nil` at TypeScript call sites. Verify: `go vet ./...`.
- [x] 3.2 In `Analyze`, run `pythonMemberImportScan` for Python before mapping, fail closed on its error, and append its warnings. Verify: `go build ./...`.
- [x] 3.3 Update `TestMapToModules_RecordedPython`: remove the mismatch comment, set `fixturepy.wide` InternalOut=2 and `cycle_b` InternalIn=2, add the `ring_a`/`ring_b` rows, and add edges `wide → cycle_b`, `ring_a → fixturepy`, `ring_a → ring_b`, `ring_b → ring_a`. Add `TestMapToModules_MemberEdgeUnanchored` (member scanned but no matching `<inc t=>` adds no edge), `_MemberEdgeSymbol` (non-directory member adds nothing), `_MemberEdgeDedup` (member equal to an existing edge counted once), and `_CaCeUnchanged`. Verify: `go test -race -count=1 ./internal/ripwireadapter/ -run MapToModules`.
- [x] 3.4 Add a cycle test (`cycles_test.go` or an assembly test) asserting `[fixturepy.ring_a fixturepy.ring_b]` is among the derived cycles on the recorded fixture, with `cycle-approximation` present. Assert that without member imports (nil) the ring cycle is absent. Verify: the test passes, and it fails if the member pass is removed.
- [x] 3.5 Update the remaining tests that hardcode Python modules, edges, line totals, or warning sets (`assembly_test.go`, `integration_test.go`, `lightparse_test.go`, `lcom_test.go`, `parse_arch_test.go`, `parse_deps_test.go`, `degrade_test.go`, `fake_ripwire_test.go`) for the new fixture rows. Verify: `go test -race -count=1 ./...` passes with ripwire removed from PATH.

## 4. Documentation

- [x] 4.1 [P] README: drop the wide-form sentence from the `cycle-approximation` row. Verify: `grep -n "from pkg import subpkg" README.md` returns nothing.
- [x] 4.2 [P] CHANGELOG `Unreleased`: a Fixed entry saying Python cycle derivation now includes wide-form member edges, and that a `diff` across the upgrade can report a previously-missed cycle as new. Verify: the entry is present.
- [x] 4.3 [P] AGENTS.md project structure: add `memberimports.go` under `internal/ripwireadapter/`. Update `testdata/README.md` / `golden-notes.md` cross-references to the parent change's wide-form Risk. Verify: `grep -n memberimports AGENTS.md`.
- [x] 4.4 File the documentation issue for the user-facing change (AGENTS.md documentation gate) and record its number here. Filed: #97.

## 5. Gates

- [x] 5.1 CI parity: run every command in `.github/workflows/ci.yml` locally (build, `go test -race -count=1 -coverprofile=coverage.out ./...`, coverage ratchet, vet, golangci-lint, plus any others listed there) with ripwire removed from PATH. Verify: all pass. Coverage floors are not lowered, and the `ripwireadapter` floor is raised with `--update` only if it rises.
- [x] 5.2 Run `/uf.review-council`. Resolve every REQUEST CHANGES. Verify: all reviewers APPROVE.
- [x] 5.3 Push `opsx/wide-form-cycle-edges` and open a draft PR stacked on #96, stating that it cannot merge before #96. Verify: PR open.
- [ ] 5.4 After #96 merges, rebase onto `main`. If #96's Phase B re-captured with a different binary, repeat 2.2–2.3 and 5.1. Mark the PR ready.

## 6. Follow-up (not this change)

- [ ] 6.1 Propose a separate OpenSpec change for relative-import edge resolution (`from . import x`, `from ..b.api import f` are currently classified external). Verify: the change directory or issue exists and is linked from this change's PR.
