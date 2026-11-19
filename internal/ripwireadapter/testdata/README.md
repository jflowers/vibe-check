# Recorded ripwire Outputs

Recorded outputs of the pinned ripwire binary against the fixture projects,
plus the `--arch` rules file used as a capture input. These recordings are
the golden inputs for the fake-ripwire test harness, the parser tests, and
the hand-computed `ModuleGraph` goldens (`recorded/golden-notes.md`). The
fixtures themselves are documented in [`FIXTURES.md`](FIXTURES.md).

History: first captured from ripwire 0.6.9 @ `bedc0cd` by
`openspec/changes/ripwire-adapter` (tasks 1.2–1.4); re-captured for the pin
move by `openspec/changes/bump-ripwire-pin` (tasks 1.1–1.4, 7.1–7.3).

## Layout

```text
recorded/
  arch-rules.txt               # --arch rules file (capture input, committed)
  golden-notes.md              # value-by-value ledger for the goldens
  python/
    version.txt                # `ripwire --version` output
    arch.txt                   # --arch metrics table (XML, root element <arch>)
    deps.xml                   # --deps file-to-file dependency graph (XML)
    cc.json                    # --export=cc.json per-file metrics (CodeCharta)
    golden.json                # hand-computed ModuleGraph golden
  typescript/
    version.txt, arch.txt, deps.xml, cc.json, golden.json
  python-mixed/                # ARCH-HONEST surface: python/ plus one Go file
    version.txt, arch.txt, deps.xml, cc.json
  python-zero-edge/            # synthetic: python/ with an edgeless deps.xml
    version.txt, arch.txt, deps.xml, cc.json
```

Recorded outputs deliberately live **outside** the fixture project
directories: ripwire indexes JSON as a config language, so a recorded
`cc.json` inside a crawl root would change every subsequent capture of
that root and break byte-stability.

## Binary provenance — the validated (version, commit) pair

> **PROVISIONAL (Phase A).** The recordings below were captured from a
> Release **source build** of ripwire `main`, not a published release:
> `openspec/changes/bump-ripwire-pin` design D8 develops against `main`
> until upstream publishes the first release containing
> redhat-et/ripwire#533. The committed `version.txt` files are therefore a
> **hand-written release-shaped line** — the real `--version` line with
> its version token replaced by the expected release number — so the
> fake harness passes the provisional pin in `version.go`. Phase B
> (tasks 7.1–7.3) replaces this table, every `version.txt`, and the pin
> with the published release's values, and re-captures every recording.

| Field | Phase A value |
|-------|---------------|
| Real `--version` line | `ripwire 0.6.10-dev.65+g1edb6fd6a (Release, AppleClang 21.0.0.21000334, emit=std::print, built_from=1edb6fd6a)` |
| Committed `version.txt` (provisional) | `ripwire 0.6.11 (Release, AppleClang 21.0.0.21000334, emit=std::print, built_from=1edb6fd6a)` |
| Source commit | `1edb6fd6a` (redhat-et/ripwire `main`; contains #533's merge `0b8f0bf8`) |
| Build | `cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build --target ripwire` |
| Platform | `macos-arm64` (AppleClang 21) |
| Binary sha256 | `bf8cd107b515d1a4d36cdc2fd6ce7530042c0334bf30f302cc8f0732b01d3570` |

The binary was built and run **outside the repository** and must never be
committed or copied into the repo.

## Capture commands (exact, as run)

All commands run from the repository root. `$RIPWIRE` is the binary from
the provenance table. Each subshell `cd`s into the crawl root and passes `.`
so the recorded `root=` attribute is `"."` and row paths are
root-relative, making the recordings independent of the checkout path.
`version.txt` is written per the provenance note above.

### Python fixture (src layout — crawl from `src/`, never the project root)

```bash
(cd internal/ripwireadapter/testdata/python/src && $RIPWIRE . --arch=../../recorded/arch-rules.txt --no-cache) > internal/ripwireadapter/testdata/recorded/python/arch.txt

(cd internal/ripwireadapter/testdata/python/src && $RIPWIRE . --deps --limit=1000 --no-cache) > internal/ripwireadapter/testdata/recorded/python/deps.xml

(cd internal/ripwireadapter/testdata/python/src && $RIPWIRE . --export=cc.json --no-cache) > internal/ripwireadapter/testdata/recorded/python/cc.json
```

### TypeScript fixture (crawl from the directory holding `tsconfig.json`/`package.json`)

```bash
(cd internal/ripwireadapter/testdata/typescript && $RIPWIRE . --arch=../recorded/arch-rules.txt --no-cache) > internal/ripwireadapter/testdata/recorded/typescript/arch.txt

(cd internal/ripwireadapter/testdata/typescript && $RIPWIRE . --deps --limit=1000 --no-cache) > internal/ripwireadapter/testdata/recorded/typescript/deps.xml

(cd internal/ripwireadapter/testdata/typescript && $RIPWIRE . --export=cc.json --no-cache) > internal/ripwireadapter/testdata/recorded/typescript/cc.json
```

### Mixed-language variant (temporary tree, never committed)

```bash
MIXED=$(mktemp -d)
cp -R internal/ripwireadapter/testdata/python "$MIXED/tree"
printf 'package helper\n\nfunc Help() int { return 1 }\n' > "$MIXED/tree/src/fixturepy/cycle_a/helper.go"
RULES=$PWD/internal/ripwireadapter/testdata/recorded/arch-rules.txt
OUT=$PWD/internal/ripwireadapter/testdata/recorded/python-mixed

(cd "$MIXED/tree/src" && $RIPWIRE . --arch="$RULES" --no-cache) > "$OUT/arch.txt"
(cd "$MIXED/tree/src" && $RIPWIRE . --deps --limit=1000 --no-cache) > "$OUT/deps.xml"
(cd "$MIXED/tree/src" && $RIPWIRE . --export=cc.json --no-cache) > "$OUT/cc.json"
```

The temporary tree is not a git checkout, so its `cc.json` `churn` values
are `0` by construction.

### Zero-edge variant (derived, not captured)

`python-zero-edge/` copies `python/`'s `version.txt`, `arch.txt`, and
`cc.json` verbatim. Its `deps.xml` is `python/deps.xml` with the window
emptied: the root's `files=`, `shown=`, `total=`, and `next_offset=` set to
`0`; `<godfiles>` emptied to `total="0" shown="0"`; every `<f>` file row and
any `<cycles>` element removed; a trailing newline added. `<health>` and
`<pruned_packages>` are kept as captured. It drives the zero-coupling guard
test (an edgeless crawl over a non-empty corpus).

## Flag choices and why

- **`--limit=1000` (`--deps` only)** — the fixtures contain at most 16
  in-corpus files, so 1000 can never truncate; the recorded roots carry
  `capped="0" has_more="0"` plus the full paging attributes
  (`total= offset= limit=`), proving nothing was cut. In the pinned release
  the same limit also sizes the `<cycles>` listing (max(20, `--limit`)).
  `--limit`/`--offset` are honored **only** by the paginated verbs —
  `--arch` and `--export` refuse them with exit 1.
- **`--no-cache`** — forces a cold parse so no per-root cache state can
  influence a recording.
- **No `--exclude`** — the crawl root (the positional argument) already is
  the package root for every fixture. ripwire itself hard-prunes source
  directories named `build`/`dist`/`out`/`target`/`vendor`/`third_party`/
  `captures`.
- **`--arch` rules file** — `arch-rules.txt` defines five layers and no
  `deny` rules, so no violation can fire (`--arch` exits 2 on a violation;
  these captures expect exit 0). The rules-file path is not embedded in
  the output. Do not add `--baseline`/`--baseline-update` — those write a
  `.ripwire_arch_baseline` sidecar into the working directory.

## Byte-stability verification (method and results)

Method: every capture command was executed **twice**, each run writing to
its own scratch directory outside the repo, every invocation asserted to
exit 0, and the two runs compared with `diff -r`. The fixture trees were
checksummed before and after all captures and are byte-unchanged; no
`.ripwire*` sidecar files were created.

stderr is empty for every invocation **except** `python-mixed` `--arch`,
which prints one expected disclosure line (exit 0):
`ripwire arch: 1 file(s) in go: this build resolves almost none of their
imports to files, so an edge through one was mostly never judged:
violations= is a floor`. The adapter's runner ignores stderr on success.

The Python recordings were captured **after** the fixture files were
committed (`cc.json` records git `churn`, which is `0` for untracked files).

Results — every recording byte-identical across the two runs:

| Recording | Bytes |
|-----------|-------|
| `python/arch.txt` | 3144 |
| `python/deps.xml` | 2758 |
| `python/cc.json` | 3086 |
| `typescript/arch.txt` | 2172 |
| `typescript/deps.xml` | 2123 |
| `typescript/cc.json` | 1672 |
| `python-mixed/arch.txt` | 4077 |
| `python-mixed/deps.xml` | 3547 |
| `python-mixed/cc.json` | 3504 |
| `*/version.txt` (provisional, hand-written) | 92 |

## Reading the recordings (fixture-specific facts)

- **Row paths carry no `./` prefix** in the pinned release
  (`fixturepy/cycle_a`; 0.6.9 printed `./fixturepy/cycle_a`). The crawl
  root row is still `.`. The mapping accepts both forms.
- **Python `operations/build` is pruned and now disclosed.** ripwire
  hard-prunes directories named `build`; `arch.txt` and `deps.xml` carry
  `<pruned_packages count="1">` naming `fixturepy/operations/build`
  (it holds an `__init__.py`). The adapter's `denylist-pruned` warning
  lists the union of this disclosure and its own pre-check.
- **The three import-fix packages show at directory grain:** `wide`
  `ce="2"` (gap 1: `from fixturepy import cycle_b` reaches the `cycle_b`
  subpackage), `alias` `ce="1"` (gap 2), `collide` `ce="0"` (gap 4).
- **`--deps` records imports as written, not as resolved.** `wide/
  consumer.py` lists a single `<inc t="fixturepy"/>`; the resolved member
  edge to `cycle_b/__init__.py` shows only in that file's `afferent=` and
  in `cc.json` `fan_out=2`. The adapter's edge set (and so its cycle
  derivation) name-resolves `<inc t=>` text and therefore does not see it
  — see `openspec/changes/bump-ripwire-pin` design, Risks.
- **Python `deps.xml` has no `<cycles>` element.** The fixture's import
  cycle resolves file-level as `cycle_a/__init__.py → cycle_b/api.py` and
  `cycle_b/__init__.py → cycle_a/api.py` (each edge one-way at file grain).
  The two-directory cycle exists at directory grain and is the adapter's
  own SCC derivation. The TypeScript `deps.xml` carries a `<cycles>`
  element with the size-2 cycle and `<health shape="tangled"
  cyclic_files="2">` (0.6.9 printed `vertical`; the adapter does not
  consume shape).
- **Python `arch.txt` reports `abstract="0"`** for every module —
  ripwire's Python abstractness is structurally zero (Python abstractness
  comes from vibe-check's own light parse). TypeScript `src` reports
  `types="5" abstract="4"` (interface share including the nested
  `Shapes.Point`).
- **`python-mixed` `fixturepy/cycle_a`** reads `ca="2" ce="1" types="0"
  abstract="0" A="0.00" zone="unknown" no_file_edges="go"` — no `I`, `D`,
  or `reachable` — and the root carries `no_file_edges="go"
  unmeasured_files="1" graph_partial="1"`.
- **`tsconfig.json` and `package.json` are indexed** (config languages):
  they appear in the TypeScript `cc.json` tree and in `deps.xml`
  `health files="5"` (3 dep-capable `.ts` files).

## Re-capture procedure

1. Obtain the binary named in the provenance table (Phase B: download the
   release archive, verify its published sha256, extract outside the
   repository) and confirm it prints the expected `--version` line.
2. From the repository root, run every command in "Capture commands"
   twice into separate scratch directories, `diff -r` the pairs, and
   rebuild `python-zero-edge/deps.xml` as described.
3. Recordings are only valid if both runs are byte-identical, every
   invocation exits 0, and stderr is empty apart from the documented
   `python-mixed` line. A nonzero `--arch` exit means the committed rules
   file drifted (a violation fired).
4. Commit any fixture changes **before** capturing, so `cc.json` `churn`
   matches the committed tree.
