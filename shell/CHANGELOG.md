# Changelog

All notable changes to `github.com/rosscartlidge/autocli/shell` — the embeddable interactive shell (line editing, history, pipe-aware completion) —
are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The version is the `Version` constant in `version.go` (no "v"); its release
is the annotated tag `shell/vX.Y.Z` (a nested module's tags carry its
directory). `TestVersionMatchesChangelog` keeps the constant, the newest
entry below and a tag on HEAD in agreement.

## [Unreleased]

## [0.5.1] - 2026-10-11

### Added

- **A changelog, a `Version` constant and a release gate**, released the
  way autocli and ssql are (see the autocli `CLAUDE.md` "Release
  Process"). No change to the module's behaviour.

## [0.5.0] - 2026-09-04

- Options.Validate pre-execution hook

## [0.4.0] - 2026-06-17

- SchemaWalk hook seeds upstream fields into tab completion

## [0.3.1] - 2026-05-20

- Ctrl-C, pipe-aware completion, safe non-pipeline stdin

## [0.3.0] - 2026-05-20

- Position 2 pipes

## [0.2.2] - 2026-05-19

- ResizeChan for terminal-size updates

## [0.2.1] - 2026-05-18

- Generic Settings registry

## [0.2.0] - 2026-05-15

- Switch from chzyer/readline to x/term

## [0.1.5] - 2026-05-15

- No raw-mode on non-stdin sources

## [0.1.4] - 2026-05-14

- Prefs.json persistence

## [0.1.3] - 2026-05-14

- Trailing-space completion + v4.6.1

## [0.1.2] - 2026-05-14

- Embedded help + friendly unknown-command

## [0.1.1] - 2026-05-12

- Readline driver (initial release; v0.1.0 was retagged due to path correction)
