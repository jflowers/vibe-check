## Context

`internal/ripwireadapter` builds two views of a Python project's coupling:

- **Ca/Ce**: copied verbatim from ripwire's `--arch` rows (`ripwire-adapter`
  design D3). Since redhat-et/ripwire#533, which `bump-ripwire-pin` adopts,
  those rows count the member edge of `from pkg import subpkg`.
- **The directory edge set** (`mapToModules` → `moduleMapping.Edges`, plus
  `InternalOut`/`InternalIn`): built by name-resolving each `--deps`
  `<inc t=>` target to the longest matching module directory
  (`resolveTarget`). Tarjan SCC in `cycles.go` runs over this set, and the
  zero-coupling guard counts it.

`--deps` records the target as written, so `from fixturepy import cycle_b`
yields `<inc t="fixturepy"/>`. The edge to `fixturepy/cycle_b` exists in Ca/Ce
but not in the edge set, and a cycle closed only through such an import is
invisible. `bump-ripwire-pin` design (Risks, "Wide-form member edges are
invisible to cycle derivation") names two remedies; this change picks one.

### Evidence (probed 2026-11-23, ripwire 0.6.10 release and the recorded
`1edb6fd` captures)

| Statement in file F | `--deps` `<inc t=>` | Adapter edge today |
|---------------------|---------------------|--------------------|
| `from fixturepy import cycle_b` | `fixturepy` | F → `fixturepy` only |
| `from fixturepy.cycle_a.api import helper_a` | `fixturepy.cycle_a.api` | F → `fixturepy.cycle_a` (correct) |
| `import fixturepy.cycle_a.api as ca` | `fixturepy.cycle_a.api` | F → `fixturepy.cycle_a` (correct) |
| `from pkg import (\n c,\n b as bee,\n)` | `pkg` | F → `pkg` only |
| `from . import inner` / `from ..b.api import f` | `.` / `..b.api` | none (classified external) |

The last row is a separate, pre-existing gap (see Non-Goals).

## Goals / Non-Goals

### Goals

- A cycle closed only through a wide-form subpackage import with an absolute
  base is detected.
- The recorded wide fixture's internal counters agree with its Ce
  (`InternalOut = Ce = 2`).
- No change to Ca/Ce/I/A/D, the schema, Layer 1, or the TypeScript path.
- Hermetic: CI still needs no ripwire binary.

### Non-Goals

- **Relative imports.** `resolveTarget` treats a leading-dot target as
  external, so even the base edge of `from ..b import x` is dropped and is
  counted in `ExternalOut`. Fixing that means resolving the base relative to
  F's package, which is a different and broader correction (it changes
  `ExternalOut` and base edges, not only members). It is proposed as its own
  change. This change excludes relative bases so that change stays a clean,
  separately reviewable delta.
- Consuming `--dump-graph` (#68 / redhat-et/ripwire#477).
- Reconciling Ce with the edge set in general. Ca/Ce stay authoritative, and
  the edge set remains a name-resolved approximation used for cycles.
- A full Python parser or AST.

## Decisions

### D1: Remedy (a), vibe-check's own light parse, over (b), waiting for `--dump-graph`

| | (a) Light parse member names | (b) Wait for `--dump-graph` |
|---|---|---|
| Available | Now | redhat-et/ripwire#477 is open with no release date, and #68 tracks it |
| Fidelity | Recovers the member name ripwire dropped. Resolution is by directory existence in the `--arch` universe, so it can't invent modules | Exact, file-resolved edges, which would also retire name-resolution |
| Surface | ~150 lines of Go plus tests in the established light-parse style | New verb, new parser, new pin move, and re-recording everything |
| Consistency | Matches the hybrid split: abstractness (D8) and LCOM4 already come from vibe-check light parses over the same crawl root | The cleaner end state |

**Recommendation: (a).** The gap is a known false negative in a gate-feeding
signal (`diff` treats a new cycle as a regression), and (b) is unbounded in
time. (a) is small, hermetically testable, and fully superseded when (b)
lands: the scan and the anchoring rule are deleted and edges come from the
graph dump. #68 stays open as that end state.

### D2: Scanner — `pythonMemberImportScan(crawlRoot)`

New file `memberimports.go`. It walks the crawl root with the same denylist
skip and the same `parse-error` degradation as `pythonAbstractnessScan`. It
returns `map[string][]memberImport`, keyed by root-relative file path
(slash form, matching `--deps` `p=`), where `memberImport{Base, Name string}`.
Per file the list is sorted by (Base, Name) and deduplicated.

The scanner is a lexical pass over a pure function
`scanPythonMemberImports(source string) []memberImport`:

- Comments are dropped. Every string literal (single- or triple-quoted,
  with backslash escapes) is blanked to `""`, so import-like text inside a
  docstring or string never matches.
- Physical lines are joined into logical lines across backslash
  continuations and across any open bracket, so parenthesized multi-line
  name lists are read whole. LF, CRLF, and lone-CR line endings are all
  recognized.
- Lines are split on `;` into statements, and each statement is matched
  against `^\s*from\s+([A-Za-z_][\w.]*)\s+import\s+(.+)$`. The leading
  identifier character excludes relative bases.
- The names list is split on `,`, `as <alias>` is stripped, empty entries
  (trailing comma) and `*` are dropped, and each name must be an identifier.
- Indented statements (inside `if TYPE_CHECKING:`, `try:`, or functions) are
  included. Conditional imports are real edges, and ripwire records them too.

Documented limitations, consistent with the other light parses: non-ASCII
identifiers are skipped, a `;` inside brackets splits a statement, and
syntax the lexer does not model (for example nested quotes in 3.12
f-strings, or an unbalanced bracket in a file with a syntax error) can
mis-join lines. The anchoring rule (D3) neutralizes most resulting false
positives.

*Correction during implementation:* an earlier draft of this decision
tracked only triple-quoted strings and joined lines only after `import (`.
The implementation blanks every string literal and joins across any open
bracket, which removes the single-quoted-string false positive the draft
listed. That stricter behavior is now the design.

### D3: Mapping — anchored member edges

`mapToModules(language, table, deps, members)` gains a fourth parameter (nil
for TypeScript and in tests that don't need it). After the existing
include-target pass, for each `deps.Rows` file F (in row order, which is
already deterministic):

1. Build the set of F's include targets verbatim.
2. For each `memberImport{Base, Name}` scanned in F with `Base` in that set,
   take `dir = strings.ReplaceAll(Base, ".", "/") + "/" + Name`. If `dir` is in
   `moduleByDir`, insert the edge F-module → that module through the same
   dedup path as the existing loop, which updates `InternalOut`/`InternalIn`.

Anchoring on `<inc t="Base">` means a member edge appears only for a statement
ripwire also recorded. A file outside a capped `--deps` window, a pruned
directory, or a docstring false positive contributes nothing, so the member
edges inherit the existing depsWindow and denylist disclosures instead of
needing new ones.

A member that is a submodule *file* (`from pkg import mod` with `pkg/mod.py`)
or a symbol resolves to no module directory and is skipped. Its edge is the
base edge already present, because `pkg/mod.py` belongs to the `pkg` module at
directory grain.

The insertion is factored into a small `addEdge` closure shared by both
passes, so the dedup and counter logic is not duplicated.

No new warning code. Member edges are name-resolved, and the base edge they
are anchored to already triggers `name-resolved-edges`, whose message covers
them. The `cycle-approximation` message is unchanged.

### D4: Assembly order

In `Analyze`, for Python only, `pythonMemberImportScan(crawlRoot)` runs before
`mapToModules`. A scan error (missing or non-directory crawl root, which
`resolveCrawlRoot` has already ruled out) fails closed like the abstractness
scan. Scan warnings are appended to the warning list (sorted by code as
today). Three walks of the crawl root become four. Folding the scans into one
walk is a possible later optimization and is not needed for correctness.

### D5: Fixture — a cycle closable only through a wide-form import

Add to `testdata/python/src/fixturepy/`:

- `ring_a/__init__.py`: `from fixturepy import ring_b`
- `ring_a/api.py`: defines `marker()`
- `ring_b/__init__.py`: `from fixturepy.ring_a.api import marker`

Name-resolution alone gives `ring_a → fixturepy` and `ring_b → ring_a`, which
is acyclic. The member edge `ring_a → ring_b` closes `{ring_a, ring_b}`. At
file grain the cycle is also acyclic (`ring_a/__init__` → `ring_b/__init__` →
`ring_a/api.py`), mirroring `cycle_a`/`cycle_b`, so ripwire's `<cycles>`
listing does not list it. That proves detection comes from the derivation,
not from corroboration. The `ring_` prefix avoids the `cycle_a`/`cycle_b`
layer substrings in `arch-rules.txt`.

Every recording that replays the Python tree (`python`, `python-zero-edge`,
`python-mixed`) is re-captured with the same binary and commands the parent
change used (`testdata/README.md`; Release build of ripwire `main` @
`1edb6fd`, or the published release once `bump-ripwire-pin` Phase B
re-captures). This includes the two-run `cmp` check. Goldens and hand
computations in `golden-notes.md` are updated.

### D6: Error budgets unaffected

`errorBudgetWarning`'s Python figures measure ripwire's resolved coupling
(Ca/Ce) against an `ast` ground truth. This change leaves Ca/Ce untouched, so
the figures stand and are not re-measured.

## Risks / Trade-offs

- **Edge set can now exceed Ce.** Where the light parse resolves a member that
  ripwire's `--arch` does not count, `InternalOut` can exceed Ce. The mismatch
  can now go either way. This is acceptable: Ca/Ce are authoritative for
  coupling and the edge set is the cycle input. Because of anchoring, every
  extra edge corresponds to a real `from B import n` statement whose `B/n` is a
  package in the universe, which is a true import dependency in Python.
- **Light-parse false positives.** Because every string literal is blanked,
  a spurious member edge needs lexer confusion (for example an f-string with
  nested quotes, or an unbalanced bracket in a file with a syntax error) and
  an anchor match and a `B/n` package directory. This is disclosed in GoDoc
  with the other light-parse limits.
- **Performance.** One more read of every `.py` file under the crawl root.
  That is linear and small next to the three ripwire subprocesses.
- **Depends on #96.** The branch is stacked on `opsx/bump-ripwire-pin`. If
  #96's Phase B re-captures with a different binary, this change's recordings
  must be re-captured with it too. The task list includes a rebase-and-
  re-capture step.

## Constitution alignment

See `proposal.md`: PASS on all assessed principles. The change improves
Metric Fidelity by closing a known cycle-detection false negative without
altering authoritative coupling values.
