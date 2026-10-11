# Changelog

All notable changes to `github.com/rosscartlidge/autocli/ssh` — the SSH-accessible service console over autocli/shell —
are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The version is the `Version` constant in `version.go` (no "v"); its release
is the annotated tag `ssh/vX.Y.Z` (a nested module's tags carry its
directory). `TestVersionMatchesChangelog` keeps the constant, the newest
entry below and a tag on HEAD in agreement.

## [Unreleased]

## [0.1.15] - 2026-10-11

### Added

- **A changelog, a `Version` constant and a release gate**, released the
  way autocli and ssql are (see the autocli `CLAUDE.md` "Release
  Process"). No change to the module's behaviour.

## [0.1.14] - 2026-09-04

- Options.Validate pass-through (shell v0.5.0)

## [0.1.13] - 2026-06-17

- SchemaWalk passthrough

## [0.1.12] - 2026-05-20

- shell v0.3.0 (pipes)

## [0.1.11] - 2026-05-19

- SSH window-size propagation

## [0.1.10] - 2026-05-18

- Settings pass-through

## [0.1.9] - 2026-05-18

- Pull in shell v0.2.1

## [0.1.8] - 2026-05-15

- Pull in shell v0.2.0

## [0.1.7] - 2026-05-15

- Pull in shell v0.1.5

## [0.1.6] - 2026-05-14

- Per-user prefs.json

## [0.1.5] - 2026-05-14

- Pull in v4.6.1 + shell v0.1.3

## [0.1.4] - 2026-05-14

- Force-close active sessions on ctx-cancel

## [0.1.3] - 2026-05-14

- Pull in v4.6.0 + shell v0.1.2

## [0.1.2] - 2026-05-14

- CRLF translation for SSH-PTY sessions

## [0.1.0] - 2026-05-12

- SSH-accessible service console
