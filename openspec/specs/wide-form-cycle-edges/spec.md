# wide-form-cycle-edges Specification

## Purpose
Makes the ripwire adapter's Python cycle derivation see the member edges of
wide-form imports (`from pkg import subpkg`), which ripwire's `--arch` table
counts but its `--deps` include text omits.

`openspec/specs/` holds no archived specs, so the refinement of the
`ripwire-adapter` capability is ADDED here and supersedes it:

| Requirement here | Supersedes in `ripwire-adapter` | Was |
|------------------|----------------------------------|-----|
| Cycle derivation includes wide-form member edges | Cycle detection over ripwire dependency edges | edges only from name-resolving `--deps` `<inc t=>` text |

## Requirements

### Requirement: Cycle derivation includes wide-form member edges

For Python, the adapter's directory edge set (the input to cycle derivation
and to the internal edge counters) SHALL contain, in addition to the edges
name-resolved from `--deps` include targets, one member edge for each
statement `from B import n` (B an absolute dotted module name) found by
vibe-check's own light parse of a source file F, when all of the following
hold:

- the directory `B/n` (dots in B replaced by `/`) is a module in the `--arch`
  module universe;
- ripwire's `--deps` row for F lists an include target exactly equal to `B`.

The member edge runs from F's module to the `B/n` module and MUST be
deduplicated with every other edge. Member edges MUST NOT change any module's
Ca or Ce, which remain the `--arch` values. Relative bases (leading `.`) and
the `*` name MUST NOT produce member edges. Output MUST be deterministic, and
TypeScript analysis MUST be unaffected.

#### Scenario: Cycle closed only through a wide-form import is detected

- **GIVEN** the Python fixture's `fixturepy/ring_a/__init__.py` containing
  `from fixturepy import ring_b` and `fixturepy/ring_b/__init__.py` importing
  `from fixturepy.ring_a.api import marker`
- **WHEN** the project is analyzed
- **THEN** `Cycles` MUST contain `[fixturepy.ring_a, fixturepy.ring_b]`
- **AND** the `cycle-approximation` warning MUST be present

#### Scenario: Member edge reconciles the wide fixture's counters with Ce

- **GIVEN** the recorded Python fixture, where `fixturepy/wide/consumer.py`
  imports `from fixturepy import cycle_b` and `--arch` reports `fixturepy/wide`
  with `ce=2`
- **WHEN** the recordings are mapped to modules
- **THEN** the edge set MUST contain `fixturepy.wide → fixturepy` and
  `fixturepy.wide → fixturepy.cycle_b`
- **AND** `fixturepy.wide` MUST have `InternalOut = 2`, equal to its Ce

#### Scenario: Imported symbol or submodule file adds no member edge

- **GIVEN** `from fixturepy.cycle_a.api import helper_a`, where
  `fixturepy/cycle_a/api/helper_a` is not a module directory
- **WHEN** the edge set is built
- **THEN** no edge beyond the name-resolved `→ fixturepy.cycle_a` edge MUST be
  added

#### Scenario: Unanchored statement adds no edge

- **GIVEN** a source file whose light parse finds `from fixturepy import
  ring_b` but whose `--deps` row lists no include target `fixturepy` (for
  example, the row is outside a capped window, or the text sits in a string
  the light parse could not exclude)
- **WHEN** the edge set is built
- **THEN** no member edge from that statement MUST be added

#### Scenario: Multi-line and aliased forms are recognized

- **GIVEN** a statement written as `from pkg import (\n    a,\n    b as bee,\n)`
  or `from pkg import a, \\\n    b`
- **WHEN** the light parse scans the file
- **THEN** it MUST yield members `a` and `b` with base `pkg`

#### Scenario: Ca and Ce are not altered by member edges

- **GIVEN** any Python project
- **WHEN** member edges are added
- **THEN** every module's Ca and Ce MUST equal its `--arch` row values

#### Scenario: Unreadable source degrades, never fails

- **GIVEN** a Python source file the light parse cannot read
- **WHEN** the project is analyzed
- **THEN** analysis MUST complete with a `parse-error` warning naming the path
  and status `partial`, and that file MUST contribute no member edges

### Requirement: Cycle approximation caveat no longer cites wide-form imports

User-facing documentation of the `cycle-approximation` warning SHALL NOT claim
that cycles closed through `from pkg import subpkg` imports go undetected.

#### Scenario: README caveat row

- **GIVEN** the README warning table
- **WHEN** a reader looks up `cycle-approximation`
- **THEN** the row MUST describe the directory-grain SCC derivation and its
  corroboration without the wide-form limitation sentence
