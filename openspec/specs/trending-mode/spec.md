# trending-mode Specification

## Purpose
TBD - created by archiving change vibe-check-command-and-reporter. Update Purpose after archive.

## Requirements

### Requirement: Trending mode compares current against historical snapshots

The agent SHALL compare the current analysis results against previously stored snapshots when invoked in trending mode. When 2 or more snapshots exist, the agent SHALL display multi-snapshot time-series with 7-day, 30-day, and 90-day baselines, per-package sparklines, and trend classifications alongside single-baseline comparison. When only 1 snapshot exists, behavior matches the current single-baseline comparison spec.

#### Scenario: Trending with available history

- **GIVEN** the `vibe-check` binary is installed and in PATH
- **WHEN** user invokes `/vibe-check trending`
- **AND** Dewey contains previous snapshots for this project's module path
- **THEN** the agent retrieves the most recent snapshot and shows per-package metric direction (improving/degrading/stable) with delta values
- **AND** if 2 or more snapshots exist, the agent adds a time-series section with sparklines and trend classifications over 7/30/90-day windows

#### Scenario: Trending with no history

- **GIVEN** the `vibe-check` binary is installed and in PATH
- **WHEN** user invokes `/vibe-check trending`
- **AND** Dewey contains no previous snapshots for this project's module path
- **THEN** the agent reports that no historical data is available, stores the current analysis as the first baseline, and suggests running trending mode again after future changes

#### Scenario: Trending with package pattern

- **GIVEN** the `vibe-check` binary is installed and in PATH
- **WHEN** user invokes `/vibe-check trending ./internal/...`
- **THEN** the agent validates the package pattern and compares only the specified packages against their historical values

### Requirement: Dewey unavailability degrades gracefully

The trending mode SHALL report a clear limitation message when Dewey
MCP tools are not available, rather than failing.

#### Scenario: Dewey unavailable

- **GIVEN** the `vibe-check` binary is installed and in PATH
- **WHEN** user invokes `/vibe-check trending`
- **AND** Dewey MCP tools are not available
- **THEN** the agent reports that trending mode requires Dewey and
  suggests using summary or detailed mode instead

### Requirement: Trending output shows metric direction

The trending mode SHALL classify each package's metric trajectory as
improving, degrading, or stable.

#### Scenario: Improving metrics

- **GIVEN** a previous snapshot exists for comparison
- **WHEN** a package's instability or distance has decreased by more
  than 0.01, or LCOM4 has decreased by 1 or more since the last
  snapshot
- **THEN** the trending output marks that metric as "improving" with
  the delta value

#### Scenario: Degrading metrics

- **GIVEN** a previous snapshot exists for comparison
- **WHEN** a package's instability, distance has increased by more
  than 0.01, or LCOM4 has increased by 1 or more since the last
  snapshot
- **THEN** the trending output marks that metric as "degrading" with
  the delta value and flags it for attention

#### Scenario: Stable metrics

- **GIVEN** a previous snapshot exists for comparison
- **WHEN** a package's metric deltas are within tolerance (|delta| ≤
  0.01 for instability/abstractness/distance, |delta| < 1 for LCOM4)
- **THEN** the trending output marks that package as "stable"

Note: Abstractness direction is zone-dependent — a package in the
Zone of Pain improves by increasing abstractness, while other packages
may not. Trending mode tracks instability, distance, and LCOM4
directions only. Abstractness deltas are shown as raw values without
improving/degrading classification.

### Requirement: Trending output includes time-series visualization

When 2 or more snapshots exist, the trending output SHALL include per-package metric sparklines using Unicode block characters (▁▂▃▄▅▆▇█), normalized to the metric's observed range, max width 30 characters.

#### Scenario: Sparkline rendered for multi-snapshot history

- **GIVEN** 5 snapshots exist for package `pkg/foo` with Instability values [0.30, 0.35, 0.38, 0.42, 0.45]
- **WHEN** user invokes `/vibe-check trending`
- **THEN** the output includes a 5-character sparkline for `pkg/foo` Instability with trend classification
- **AND** the sparkline represents increasing values with progressively taller block characters

#### Scenario: No sparkline for single snapshot

- **GIVEN** only 1 snapshot exists for the module
- **WHEN** user invokes `/vibe-check trending`
- **THEN** no sparkline or time-series section is rendered (single-baseline comparison only)

#### Scenario: Sparkline with flat (single-value) range
- **GIVEN** 5 snapshots exist for package `pkg/foo` with Instability values [0.50, 0.50, 0.50, 0.50, 0.50]
- **WHEN** user invokes `/vibe-check trending`
- **THEN** the sparkline renders all characters at the mid-block (▄) to indicate a flat trend
- **AND** the trend is classified as "stable"

### Requirement: Trending output shows multi-window trend classification

The trending output SHALL classify per-package per-metric trends as improving, degrading, or stable for each of the 7-day, 30-day, and 90-day windows.

#### Scenario: Multi-window classification

- **GIVEN** 90 daily snapshots exist for package `pkg/bar`
- **WHEN** user invokes `/vibe-check trending`
- **THEN** the output shows trend classifications for 7-day, 30-day, and 90-day windows independently
