## Context

`mapToModules` builds the Python directory edge set that `cycles.go` runs
Tarjan SCC over. It has two passes:

1. **Include targets.** Each `--deps` `<inc t=>` is name-resolved by
   `resolveTarget`, which takes the longest dotted prefix that is a module
   directory. If no prefix matches, the target is external.
2. **Member edges** (`wide-form-cycle-edges`). For each `from B import n`
   found by the light parse in file F, where `B` is verbatim one of F's
   include targets, the pass adds an edge to the module `B/n`. Relative
   bases are excluded by the scanner regex.

ripwire records relative targets as written. A relative target has no
dotted prefix that names a directory, so pass 1 calls it external and pass 2
never sees it.

### Evidence (probed 2026-11-23, ripwire `1edb6fd6a` Release build)

Probe tree: `pk/__init__.py`, `pk/a/{__init__,inner}.py`,
`pk/a/deep/__init__.py`, `pk/b/{__init__,api}.py`.

| File | Statement | `--deps` `<inc t=>` | `--arch` effect |
|------|-----------|---------------------|-----------------|
| `pk/a/__init__.py` | `from ..b.api import f` | `..b.api` | `pk/a` ce counts `pk/b` |
| `pk/a/__init__.py` | `from . import inner` | `.` | intra-module (not in ce) |
| `pk/b/__init__.py` | `from .. import a` | `..` | `pk/b` ce=2 (`pk` and member `pk/a`) |
| `pk/b/__init__.py` | `from .api import f as g` | `.api` | intra-module |
| `pk/a/deep/__init__.py` | `from ... import b` | `...` | ce counts `pk` and member `pk/b` |
| `pk/a/deep/__init__.py` | `from .... import nope` | `....` | no edge (climbs above the crawl) |
| `pk/a/deep/__init__.py` | `from ..inner import x` | `..inner` | ce counts `pk/a` |

ripwire's Ca/Ce therefore already resolve relative bases and their members.
Only the adapter's edge set is missing them.

## Goals / Non-Goals

### Goals

- A cycle that closes only through relative imports (base or member form)
  is detected.
- A relative target is never counted in `ExternalOut`.
- No change to Ca/Ce/I/A/D, the schema, Layer 1, the TypeScript path, or the
  ripwire pin.
- Hermetic: CI still needs no ripwire binary.

### Non-Goals

- **Self-edges.** `from . import inner` resolves to the importing module
  itself. The adapter already adds self-edges for absolute same-package
  imports (`from pkg.a.x import y` inside `pkg/a`). Tarjan ignores size-1
  SCCs, and `InternalOut`/`InternalIn` are not emitted in the `ModuleGraph`.
  This change keeps that behavior for relative imports rather than changing
  self-edge policy for both forms. That would be a separate change if wanted.
- **Implicit relative imports** (Python 2 `import sibling`). Python 3 has
  none, so they are not handled.
- Consuming `--dump-graph` (#68). Relative resolution here and the member
  light parse would both be retired by it.

## Decisions

### D1: Resolve a relative target against the importing file's package

`resolveTarget` gains a branch for Python targets that start with `.`. It
delegates to a pure helper,
`relativeBaseDir(sourceDir, target string) (anchor, dir string, ok bool)`, which:

1. Splits `target` into `k` leading dots and remainder `R`. `R` can be empty
   (`.`, `..`).
2. Starts from `sourceDir`, the file's directory, which is its package for
   both `__init__.py` and module files. Python's relative imports are
   anchored to `__package__`, which for both file kinds is the directory.
   It then drops `k − 1` trailing path components. The crawl root `.` has
   zero components. If fewer than `k − 1` components exist, `ok` is false:
   the import climbs above the crawl root.
3. Returns the anchor and the anchor directory joined with `R`'s components
   (dots become `/`). The anchor bounds the prefix search below.

Then the resolution does the following:

- Take the longest prefix of the joined path that is in `moduleByDir`,
  **never shorter than the anchor**. So `from .x.y import z` lands on
  `anchor/x/y`, `anchor/x`, or `anchor`, and never on an ancestor of the
  anchor. That would be a guess about a package the statement does not
  name.
- If a prefix matches, the target is an internal edge to that module.
- If `ok` is false, or no prefix at or below the anchor matches (for
  example, a namespace-package anchor with no `--arch` row), the target is
  **unresolved**. It adds no edge and sets the `ambiguity-rate` warning.
- A relative target is **never external**. Python cannot reach a
  third-party or stdlib module through a relative import.

The anchor is the crawl-root-relative directory of the `--deps` `p=` path.
For the src layout, the crawl root is `src/`, so `fixturepy/rel_b` climbs
to `fixturepy` and never to `src/`.

Relative resolution does **not** set the `name-resolved-edges` flag. That
warning discloses package-name collisions in global name lookup. A relative
target is anchored to the file's own path, so a colliding name elsewhere
cannot capture it. A Python graph with any absolute internal import still
carries the warning.

### D2: Disclose climb-above-root under `ambiguity-rate`

The `ambiguity-rate` message currently says targets "land inside the crawl
but match no module directory". A relative import that climbs above the
crawl root does not land inside the crawl. The message is widened to cover
both cases: "some include targets (in-crawl names, or relative imports,
including those climbing above the crawl root) match no module directory;
their edges were dropped from the module graph". A new warning code would add schema-facing
surface for a case that is invalid Python in a correctly rooted crawl.

### D3: Member edges for relative bases

`pyFromImportRegex` accepts a base of one or more dots followed by an
optional dotted name: `\.+(?:[A-Za-z_][\w.]*)?`, alongside the existing
absolute form. The explicit post-match rejections stay for absolute bases.
A malformed relative base (`.a..b`, a trailing dot) is skipped the same way.
`memberImport.Base` keeps the text verbatim (`..`, `.sub`), because the
anchoring rule compares it byte-for-byte with ripwire's `<inc t=>`. The probe
confirms that ripwire writes the same text.

In the mapping's member pass, the member directory for a relative base is
`relativeBaseDir(sourceDir, Base)` + `/` + `Name`. For an absolute base it
stays `identityToDir(Base + "." + Name)`. The member directory must be
**exactly** a module: the existing rule, with no prefix fallback. If the
relative base climbs above the root, it is skipped. Its include target is
already unresolved under D1.

### D4: Fixture — a cycle closable only through relative imports

Add to `testdata/python/src/fixturepy/`:

- `rel_a/__init__.py`: `from ..rel_b.api import token`. This is a relative
  **base** edge, `rel_a → rel_b`, through D1.
- `rel_b/__init__.py`: `from .. import rel_a`. This is a relative **member**
  edge, `rel_b → rel_a`, through D3. Its base `..` also yields
  `rel_b → fixturepy` through D1.
- `rel_b/api.py`: defines `token()`.

Neither edge exists without this change, so the `{rel_a, rel_b}` SCC proves
both D1 and D3 end to end. The unit tests isolate each: mapping with `nil`
members yields `rel_a → rel_b` and `rel_b → fixturepy` but no cycle. At
file grain the pair is acyclic (`rel_b/__init__.py` → `rel_a/__init__.py` →
`rel_b/api.py`), so ripwire's `<cycles>` listing does not report it. That
proves the detection comes from the derivation. The `rel_` prefix matches no
`arch-rules.txt` layer substring.

Expected `--arch`: `fixturepy/rel_a ca=1 ce=1`, `fixturepy/rel_b ca=1 ce=2`.
`fixturepy` gains `rel_b` as a dependent.

The recordings that replay the Python tree (`python`, `python-zero-edge`,
`python-mixed`) are re-captured with the binary and commands in
`testdata/README.md`, twice with `cmp`. `version.txt` is unchanged. Goldens
and `golden-notes.md` are updated.

## Risks / Trade-offs

- **`ExternalOut` drops and edge counts rise** for packages that use
  relative imports. `ExternalOut` and the Internal* counters are internal
  to the mapping (not emitted), so the visible effect is on `Cycles` and on
  the zero-coupling guard. The guard can only stop firing: a crawl whose
  only edges were relative imports previously looked edgeless and wrongly
  tripped it.
- **Namespace-package anchors** (no `__init__.py`, no source files) have no
  `--arch` row. D1 marks such targets unresolved instead of guessing an
  ancestor. That can under-report, but it never invents an edge.
- **Depends on #98 and #96.** If #96's Phase B re-captures with a different
  binary, this change re-captures too. The task list carries the rebase
  step.

## Constitution alignment

See `proposal.md`: PASS on all assessed principles.
