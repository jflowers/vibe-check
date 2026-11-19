# Error-budget measurement results

Method and commands: [`README.md`](README.md). Measured 2026-11-19.

> **Phase A.** "main" is a Release source build of redhat-et/ripwire `main` @
> `1edb6fd6a` (contains #533; reports `0.6.10-dev.65+g1edb6fd6a`). Task 7.4
> re-runs every row against the published release and updates this file and
> `budget.go` if any figure moves.

| Binary | `--version` |
|--------|-------------|
| 0.6.9 (baseline, previous pin) | `ripwire 0.6.9 (Release, AppleClang 21.0.0.21000101, emit=std::print, built_from=bedc0cdcf)` |
| 0.6.10 (reference: gaps 2–4, not gap 1) | `ripwire 0.6.10 (Release, AppleClang 21.0.0.21000101, emit=std::print, built_from=ac869640e)` |
| main (Phase A pin candidate) | `ripwire 0.6.10-dev.65+g1edb6fd6a (Release, AppleClang 21.0.0.21000334, emit=std::print, built_from=1edb6fd6a)` |

## Python

Directory grain is what vibe-check consumes (`--arch` Ca/Ce per directory
module). File-level figures are count-based lower bounds (see
`measure_python.py`).

| Target | Binary | GT edges | Dirs | Ca exact (MAE) | Ce exact (MAE) | File lost ≥ | File spurious ≥ |
|--------|--------|---------:|-----:|----------------|----------------|------------:|----------------:|
| pip `src/` | 0.6.9 | 1,792 | 53 | 79.2% (0.26) | 77.4% (0.34) | 4.80% | 0.78% |
| pip `src/` | 0.6.10 | 1,792 | 53 | 81.1% (0.25) | 81.1% (0.25) | 4.91% | 0.00% |
| pip `src/` | **main** | 1,792 | 53 | **100.0% (0.00)** | **100.0% (0.00)** | **0.11%** | **0.00%** |
| django | 0.6.9 | 10,200 | 673 | 93.9% (0.61) | 59.0% (0.60) | 14.18% | 0.11% |
| django | 0.6.10 | 10,200 | 673 | 96.6% (0.55) | 60.6% (0.55) | 13.72% | 0.01% |
| django | **main** | 10,200 | 673 | **100.0% (0.00)** | **100.0% (0.00)** | **0.00%** | **0.01%** |

Residue on main, all outside directory-grain coupling:

- pip: 2 file edges lost, both intra-directory in vendored code
  (`pip/_vendor/pygments/__main__.py`, `pip/_vendor/rich/tree.py`).
- django: 1 spurious edge from
  `tests/test_runner_apps/tagged/tests_syntax_error.py`, a deliberately
  invalid test fixture the `ast` ground truth cannot parse.

0.6.10's django Ce stays near 60% because it lacks gap 1: Django code
overwhelmingly imports subpackages in the wide form (`from django.db import
models`).

Comparison with the Phase 0 spike (0.6.9; pip Ca 81% / Ce 85% exact, 1.9% lost
/ 2.2% spurious edges): the spike used a different ground-truth convention and
an edge-identity reconstruction from `--deps <inc t=>` text that no longer
holds once one import resolves to two files (#533). The like-for-like baseline
is the 0.6.9 row above, measured with this harness.

## TypeScript (rollup @ `ad39ddc`)

| Binary | Import rows | Relative (in-tree) rows | `imports_unresolved` | Unresolved / all | Unresolved / relative |
|--------|------------:|------------------------:|---------------------:|-----------------:|----------------------:|
| 0.6.9 | 8,579 | 5,973 | 14 | 0.16% | 0.23% |
| 0.6.10 | 8,579 | 5,973 | 14 | 0.16% | 0.23% |
| **main** | 8,579 | 5,973 | 14 | **0.16%** | 0.23% |

Unchanged across all three binaries; matches the spike's 14 / 8,529 (0.16%).

## Gate (design D6)

Every main figure is equal to or better than its 0.6.9 baseline on the same
target with the same harness — **gate passes**.

Two ground-truth conventions were corrected during the measurement, each
validated on the fixture before re-running every binary:

1. `from X import n` loads `X` as well as `X.n` (the wide form): the first
   run counted only the member edge.
2. Absolute imports also resolve from the importer's package-chain root (how
   Django's `tests/` and `scripts/` run), and imports into a denylist-pruned
   directory are excluded rather than attributed to the parent package. The
   first run of this rule showed django spurious rising 0.17% → 0.58%; every
   extra ripwire edge proved to be a real import under these roots.

## Figures carried into `budget.go`

- Python: directory Ca/Ce 100% exact on pip and django; file edges ≥ 0.11%
  lost, ≥ 0.01% spurious (worst of the two targets).
- TypeScript: 0.16% of import targets unresolved (rollup).
