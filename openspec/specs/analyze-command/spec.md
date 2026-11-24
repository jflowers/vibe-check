# analyze-command Specification

## Purpose
TBD - created by archiving change go-analyze. Update Purpose after archive.

## Requirements

### Requirement: CLI entry point

The `vibe-check` binary MUST be buildable from `cmd/vibe-check/main.go` using
`go build ./cmd/vibe-check`. The binary MUST use cobra for command-line parsing
with a root command and an `analyze` subcommand. The root command MUST support a
`--version` flag that prints the tool version, build commit, and build date
(embedded at build time via ldflags).

#### Scenario: Binary builds successfully

- **GIVEN** the project source code
- **WHEN** `go build ./cmd/vibe-check` is executed
- **THEN** a `vibe-check` binary MUST be produced without errors

#### Scenario: Help output

- **GIVEN** a built `vibe-check` binary
- **WHEN** `vibe-check --help` is executed
- **THEN** the output MUST list the `analyze` subcommand with a brief description

#### Scenario: Version output

- **GIVEN** a built `vibe-check` binary with version info embedded
- **WHEN** `vibe-check --version` is executed
- **THEN** the output MUST match the format `vibe-check version <version> (commit <hash>, built <date>)`

### Requirement: Analyze command invokes Go adapter

The `analyze` subcommand MUST create a Go adapter instance, invoke `Analyze` with
the target project path, and output the resulting `ModuleGraph` as JSON to stdout.
The command MUST follow the testable CLI pattern (AP-002/AP-003): a `RunAnalyze`
function accepts a params struct with `io.Writer` fields for stdout/stderr.

#### Scenario: Default analysis target

- **GIVEN** a Go module directory as the current working directory
- **WHEN** `vibe-check analyze` is invoked with no positional arguments
- **THEN** the adapter MUST analyze the current directory (`.`)

#### Scenario: Explicit path argument

- **GIVEN** a Go module at `/path/to/project`
- **WHEN** `vibe-check analyze /path/to/project` is invoked
- **THEN** the adapter MUST analyze the specified path

#### Scenario: Adapter error propagation

- **GIVEN** a target path that causes the Go adapter to return an error
- **WHEN** `vibe-check analyze` is invoked
- **THEN** the command MUST print an actionable error to stderr (including the operation that failed, the cause, and suggested remediation) and exit with status 2

### Requirement: JSON output to stdout

The analyze command MUST output the `ModuleGraph` as JSON to stdout. The output
MUST be valid JSON that passes `metrics.Validate`. The JSON MUST be
pretty-printed (indented) by default.

#### Scenario: Valid JSON on stdout

- **GIVEN** a valid Go module
- **WHEN** analysis completes successfully
- **THEN** stdout MUST contain a JSON object conforming to the `ModuleGraph` schema

#### Scenario: Stderr separation

- **GIVEN** analysis that completes with threshold violations or warnings
- **WHEN** the command exits
- **THEN** violation messages MUST be written to stderr, and valid JSON MUST still be written to stdout

### Requirement: Max instability threshold flag

The analyze command MUST accept a `--max-instability` flag with a float64 value in
the range [0.0, 1.0]. When set, if any module's Instability exceeds the threshold,
the command MUST report the violation to stderr and exit with status 1.

#### Scenario: Instability within threshold

- **GIVEN** all modules have Instability <= 0.8
- **WHEN** `--max-instability 0.8` is set
- **THEN** the command MUST exit with status 0

#### Scenario: Instability exceeds threshold

- **GIVEN** a module with Instability 0.7
- **WHEN** `--max-instability 0.5` is set
- **THEN** the command MUST print the violating module and its instability to stderr and exit with status 1

### Requirement: Max distance threshold flag

The analyze command MUST accept a `--max-distance` flag with a float64 value in
the range [0.0, 1.0]. When set, if any module's Distance exceeds the threshold,
the command MUST report the violation to stderr and exit with status 1.

#### Scenario: Distance within threshold

- **GIVEN** all modules have Distance <= 0.5
- **WHEN** `--max-distance 0.5` is set
- **THEN** the command MUST exit with status 0

#### Scenario: Distance exceeds threshold

- **GIVEN** a module with Distance 0.6
- **WHEN** `--max-distance 0.3` is set
- **THEN** the command MUST print the violating module and its distance to stderr and exit with status 1

### Requirement: No circular deps flag

The analyze command MUST accept a `--no-circular-deps` boolean flag. When set,
if any cycles are detected in the `ModuleGraph`, the command MUST report each
cycle to stderr and exit with status 1.

#### Scenario: No cycles detected

- **GIVEN** the `Cycles` slice is empty
- **WHEN** `--no-circular-deps` is set
- **THEN** the command MUST exit with status 0

#### Scenario: Cycles detected

- **GIVEN** cycles exist in the analysis result
- **WHEN** `--no-circular-deps` is set
- **THEN** the command MUST print each cycle to stderr and exit with status 1

### Requirement: Max LCOM threshold flag

The analyze command MUST accept a `--max-lcom` flag with an integer value >= 1.
When set, if any module's LCOM exceeds the threshold (higher LCOM = worse cohesion),
the command MUST report the violation to stderr and exit with status 1.

#### Scenario: LCOM within threshold

- **GIVEN** all modules have LCOM <= 3
- **WHEN** `--max-lcom 3` is set
- **THEN** the command MUST exit with status 0

#### Scenario: LCOM exceeds threshold

- **GIVEN** a module with LCOM 4
- **WHEN** `--max-lcom 2` is set
- **THEN** the command MUST print the violating module and its LCOM to stderr and exit with status 1

### Requirement: Flag value validation

The command MUST validate all flag values before running analysis. Invalid flag
values MUST prevent analysis from running and exit with status 2.

#### Scenario: Max instability out of range

- **GIVEN** `--max-instability 1.5` (value > 1.0)
- **WHEN** the command is invoked
- **THEN** the command MUST print a validation error to stderr and exit with status 2 without running analysis

#### Scenario: Max distance negative

- **GIVEN** `--max-distance -0.5` (negative value)
- **WHEN** the command is invoked
- **THEN** the command MUST print a validation error to stderr and exit with status 2 without running analysis

#### Scenario: Max LCOM less than 1

- **GIVEN** `--max-lcom 0` (value < 1)
- **WHEN** the command is invoked
- **THEN** the command MUST print a validation error to stderr and exit with status 2 without running analysis

### Requirement: Multiple threshold violations

When multiple threshold flags are set and multiple violations occur, the command
MUST report ALL violations before exiting. The command MUST NOT exit on the
first violation.

#### Scenario: Multiple flags with multiple violations

- **GIVEN** violations exist for both instability and distance thresholds
- **WHEN** `--max-instability 0.5 --max-distance 0.3` is set
- **THEN** all violations MUST be reported to stderr and the command MUST exit with status 1

### Requirement: JSON output produced regardless of violations

The analyze command MUST always produce JSON output to stdout, even when threshold
violations cause a non-zero exit code. This enables CI pipelines to capture the full
analysis results while still failing the gate.

#### Scenario: JSON output with violations

- **GIVEN** threshold violations are detected
- **WHEN** the command exits with status 1
- **THEN** the complete `ModuleGraph` JSON MUST still be written to stdout before the command exits

### Requirement: Exit code semantics

The command MUST use distinct exit codes to differentiate failure types:
- Exit 0: analysis succeeded, no threshold violations
- Exit 1: analysis succeeded but threshold violations detected (policy failure)
- Exit 2: analysis itself failed or invalid arguments (tool failure)

#### Scenario: Tool failure exit code

- **GIVEN** the target path is not a Go module
- **WHEN** `vibe-check analyze /not/a/module` is invoked
- **THEN** the command MUST exit with status 2

#### Scenario: Policy failure exit code

- **GIVEN** a threshold violation exists
- **WHEN** analysis completes successfully but a threshold is exceeded
- **THEN** the command MUST exit with status 1

#### Scenario: Success exit code

- **GIVEN** no threshold flags are set or all thresholds pass
- **WHEN** analysis completes successfully
- **THEN** the command MUST exit with status 0

### Requirement: Timeout flag

The analyze command MUST accept a `--timeout <duration>` flag (e.g., `5m`, `30s`).
When set, the command MUST create a `context.WithTimeout` wrapping the signal-handling
context. When the timeout is exceeded, the command MUST exit with status 2 and an
error message indicating timeout. The default is no timeout.

#### Scenario: Timeout exceeded

- **GIVEN** `--timeout 1ms` is set and analysis takes longer than 1ms
- **WHEN** the analysis exceeds the timeout
- **THEN** the command MUST exit with status 2 and print a timeout error to stderr

#### Scenario: No timeout by default

- **GIVEN** no `--timeout` flag is set
- **WHEN** analysis is invoked
- **THEN** no timeout MUST be applied (analysis runs until completion or signal)

### Requirement: Signal handling and graceful shutdown

The CLI MUST create a context with signal handling for SIGINT and SIGTERM. When a
signal is received during analysis, no partial JSON MUST be written to stdout. The
command MUST exit with a non-zero status and an error message to stderr.

#### Scenario: Signal suppresses partial output

- **GIVEN** analysis is in progress
- **WHEN** SIGINT is received
- **THEN** no partial JSON MUST be written to stdout, and the command MUST exit with status 2

#### Scenario: Context timeout propagated to adapter

- **GIVEN** `--timeout 10s` is set
- **WHEN** the deadline is exceeded during analysis
- **THEN** the command MUST exit with status 2 and print a timeout error to stderr

### Requirement: Threshold boundary semantics

Threshold comparison MUST use strict greater-than (`>`) for all threshold flags.
A module whose metric value exactly equals the threshold MUST pass (not violate).

#### Scenario: Instability at exact boundary

- **GIVEN** a module with Instability exactly 0.5
- **WHEN** `--max-instability 0.5` is set
- **THEN** the command MUST exit with status 0 (boundary value passes)

#### Scenario: Distance at exact boundary

- **GIVEN** a module with Distance exactly 0.3
- **WHEN** `--max-distance 0.3` is set
- **THEN** the command MUST exit with status 0 (boundary value passes)

#### Scenario: LCOM at exact boundary

- **GIVEN** a module with LCOM exactly 3
- **WHEN** `--max-lcom 3` is set
- **THEN** the command MUST exit with status 0 (boundary value passes)

### Requirement: Hermetic Toolchain

The `vibe-check analyze` command SHALL force `GOTOOLCHAIN=local` in the analysis subprocess environment so that an untrusted target module's `go.mod` `toolchain` directive cannot trigger a Go toolchain download and execution during metric collection.

The forced value MUST be set by the binary itself (added to the sanitized subprocess environment with a fixed value, not passed through from the host environment). This ensures the guarantee holds regardless of how `analyze` is invoked and does not depend on the caller — including the `divisor-entropy` agent, whose permission allowlist only permits the bare `vibe-check analyze *` form and cannot prefix an inline environment variable.

This requirement hardens `analyze` because the `divisor-entropy` agent runs it against a PR ref whose `go.mod` may be attacker-influenced. It complements — and does not replace — the trusted-refs-only operating constraint documented for the agent. Privileged CI environments SHOULD additionally export `GOTOOLCHAIN=local` ambiently as defense-in-depth.

Forcing `GOTOOLCHAIN=local` is a deliberate compatibility trade-off: a standalone `vibe-check analyze` run against a (trusted) module whose `go.mod` `toolchain` directive requires a newer-than-local Go toolchain will now FAIL rather than silently download and run that toolchain. Within the `divisor-entropy` agent this failure degrades safely to a COMMENT verdict, but it bites operators who run `analyze` directly on such a module — they must install a compatible local toolchain. This trade-off is accepted and the gate MUST NOT be weakened to restore the download behavior.

#### Scenario: Untrusted go.mod toolchain directive cannot download a toolchain

- **WHEN** `vibe-check analyze` is run against a target module whose `go.mod` declares a `toolchain` directive naming a Go version other than the running toolchain
- **THEN** the analysis subprocess environment contains `GOTOOLCHAIN=local` and the analysis proceeds with the local toolchain without downloading or executing the directive-named toolchain

#### Scenario: Forced value is not overridable by the host environment

- **WHEN** the process that invokes `vibe-check analyze` has `GOTOOLCHAIN=auto` (or any other value) set in its own environment
- **THEN** the analysis subprocess still runs with `GOTOOLCHAIN=local`, because the binary sets the forced value in the sanitized subprocess environment rather than inheriting it from the host

### Requirement: Output File Selection

The `vibe-check analyze` command SHALL support an `--output <file>` flag (short form `-o`) that writes the ModuleGraph JSON to the named file. When the flag is absent, the command SHALL write the ModuleGraph JSON to stdout as before. This lets the `divisor-entropy` agent materialize base and PR graphs to files — which `vibe-check diff` consumes as positional arguments — without relying on shell redirection, which the agent's permission allowlist forbids.

#### Scenario: Output flag writes the graph to a file

- **WHEN** `vibe-check analyze --output graph.json <target>` is run against a valid target
- **THEN** the ModuleGraph JSON is written to `graph.json` and NOT to stdout, and the command exits 0

#### Scenario: Absent output flag preserves stdout behavior

- **WHEN** `vibe-check analyze <target>` is run without the `--output` flag
- **THEN** the ModuleGraph JSON is written to stdout exactly as before

#### Scenario: Unwritable output path fails cleanly

- **WHEN** `vibe-check analyze --output <path> <target>` is given an output path that cannot be created or written
- **THEN** the command exits 2 with a diagnostic on stderr and does NOT emit a partial ModuleGraph on stdout
