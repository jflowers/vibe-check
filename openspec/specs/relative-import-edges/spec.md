# relative-import-edges Specification

## Purpose
Makes the ripwire adapter's Python edge set, the input to cycle derivation,
resolve relative import targets (`from . import x`, `from ..b.api import f`).
ripwire's `--deps` records those targets as written, and the adapter
previously classified them as external.

`openspec/specs/` holds no archived specs, so the refinements are ADDED here
and supersede:

| Requirement here | Supersedes | Was |
|------------------|------------|-----|
| Relative include targets resolve against the importing package | `ripwire-adapter` "Cycle detection over ripwire dependency edges" (Python name resolution) | a leading-dot target matched no prefix and was external |
| Member edges include relative bases | `wide-form-cycle-edges` "Cycle derivation includes wide-form member edges", the clause "Relative bases (leading `.`) … MUST NOT produce member edges" | relative bases were excluded |

## Requirements

### Requirement: Relative include targets resolve against the importing package

For Python, an include target with `k ≥ 1` leading dots and remainder `R` in
the `--deps` row of file F SHALL be resolved relative to F's directory. The
anchor is F's directory with `k − 1` trailing components removed, and the
candidate is the anchor joined with `R`'s dotted components. The target
SHALL resolve to the longest prefix of the candidate that is a module
directory and is not shorter than the anchor, producing a deduplicated edge
from F's module to that module.

A relative target MUST NOT be counted as an external dependency. A relative
target whose anchor would climb above the crawl root, or whose candidate has
no module prefix at or below the anchor, MUST add no edge and MUST cause the
`ambiguity-rate` warning. Relative resolution MUST NOT, by itself, cause the
`name-resolved-edges` warning. Ca and Ce MUST remain the `--arch` values.
Output MUST be deterministic, and TypeScript analysis MUST be unaffected.

#### Scenario: Relative base import adds an edge

- **GIVEN** `fixturepy/rel_a/__init__.py` containing `from ..rel_b.api import token`
- **WHEN** the recordings are mapped to modules
- **THEN** the edge set MUST contain `fixturepy.rel_a → fixturepy.rel_b`
- **AND** that target MUST NOT increase `fixturepy.rel_a`'s external count

#### Scenario: Bare relative base resolves to the anchor package

- **GIVEN** a file in `pkg/b/` whose `--deps` row lists `<inc t=".."/>`
- **WHEN** the edge set is built
- **THEN** it MUST contain the edge `pkg.b → pkg`

#### Scenario: Relative import climbing above the crawl root

- **GIVEN** a file `pkg/a/x.py` whose `--deps` row lists `<inc t="...."/>`
- **WHEN** the edge set is built
- **THEN** no edge MUST be added, the target MUST NOT be counted external,
  and the `ambiguity-rate` warning MUST be present

#### Scenario: Relative name below a missing anchor is not guessed

- **GIVEN** a relative target whose anchor directory and candidate
  directories are not modules, although an ancestor of the anchor is
- **WHEN** the edge set is built
- **THEN** no edge to the ancestor MUST be added and the `ambiguity-rate`
  warning MUST be present

### Requirement: Member edges include relative bases

The member-import light parse SHALL record `from B import n` statements whose
base `B` is relative (one or more leading dots, optionally followed by a
dotted name), keeping `B` verbatim. In the mapping, a relative member SHALL
add an edge from F's module to the module whose directory is exactly the
relative anchor-plus-`R` directory of `B` joined with `n`. The usual
condition still applies: F's `--deps` row MUST list an include target exactly
equal to `B`. A relative base that climbs above the crawl root MUST add no
member edge. Member edges MUST NOT change Ca or Ce.

#### Scenario: Cycle closed only through relative imports is detected

- **GIVEN** the Python fixture's `fixturepy/rel_a/__init__.py` containing
  `from ..rel_b.api import token` and `fixturepy/rel_b/__init__.py`
  containing `from .. import rel_a`
- **WHEN** the project is analyzed
- **THEN** `Cycles` MUST contain `[fixturepy.rel_a, fixturepy.rel_b]`
- **AND** the `cycle-approximation` warning MUST be present

#### Scenario: Without member edges the relative cycle is absent

- **GIVEN** the same recordings mapped with no member imports
- **WHEN** cycles are derived
- **THEN** the edge `fixturepy.rel_a → fixturepy.rel_b` MUST be present
- **AND** `[fixturepy.rel_a, fixturepy.rel_b]` MUST NOT be among the cycles

#### Scenario: Relative bases are scanned verbatim

- **GIVEN** the statements `from . import a`, `from .. import (b, c as cee)`,
  and `from .sub import d`
- **WHEN** the light parse scans the file
- **THEN** it MUST yield members `(., a)`, `(.., b)`, `(.., c)`, and
  `(.sub, d)`
