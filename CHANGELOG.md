# Changelog

All notable changes to autocli are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The version is the `Version` constant in `flag.go` (no "v"); its release
is the annotated tag `vX.Y.Z`. `TestVersionMatchesChangelog` keeps the
constant, the newest entry below and a tag on HEAD in agreement.

## [Unreleased]

## [4.21.2] - 2026-10-11

### Added

- **A changelog and a release gate.** This file, and
  `TestVersionMatchesChangelog`: `Version` must equal the newest released
  entry here, and any `v*` tag on HEAD must name that version. The release
  steps are in `CLAUDE.md` ("Release Process"), the same shape as ssql's.

### Fixed

- **`Version` said 4.0.0** in every release from v4.0.0 to v4.21.1; it now
  carries the release's own version.

## [4.21.1] - 2026-10-11

- FieldsFromFlag over a variadic source flag: the completer took the source flag only as a string, so a Variadic FILE (a list) fell back to the <FIELD> hint; the first file now names the fields

## [4.21.0] - 2026-10-09

- StrictPositionals

## [4.20.0] - 2026-09-22

- Unknown word on a root with a handler is ErrUnknownCommand (Check and Execute)

## [4.19.0] - 2026-09-22

- Command.Check: validate an argv without running it

## [4.18.0] - 2026-09-22

- -arg VALUE: flag form of a positional argument (injection-safe positionals)

## [4.17.0] - 2026-08-30

- DisplaySink metadata in -spec-json

## [4.16.0] - 2026-08-28

- -spec-json command tree dump

## [4.15.0] - 2026-08-26

- Expression() arg metadata; CompletionContext.FlagSpec; AnalyzeCursor

## [4.14.1] - 2026-08-26

- Completion context keeps longest parseable prefix (same-command field completion restored)

## [4.14.0] - 2026-08-26

- FieldSource/FieldValueSource host hooks; binary formats take the hint path

## [4.13.0] - 2026-08-12

- Remove persistent value cache; FieldValueHint

## [4.12.1] - 2026-08-12

- TSV delimiter auto-detection in field/value completion

## [4.12.0] - 2026-07-01

- Declutter flag completion (primary names + demoted root globals; aliases/meta on prefix only)

## [4.11.0] - 2026-06-29

- Subcommand help lists nested subcommands; full nested path

## [4.10.0] - 2026-06-23

- Remove stale cross-pipe field-name cache; FieldNameHint

## [4.9.0] - 2026-06-22

- HelpAt + -help-at protocol (help at the cursor)

## [4.8.0] - 2026-06-17

- FieldsFromFlag chains UpstreamFieldsCompleter (pipeline-aware field completion)

## [4.7.0] - 2026-06-16

- CompleteWithContext: seed UpstreamFields/State into completion

## [4.6.1] - 2026-05-14

- CompleteRootGlobalFlags includes built-in flags

## [4.6.0] - 2026-05-14

- ErrUnknownCommand + GenerateHelpEmbedded for embedded shells

## [4.5.0] - 2026-05-12

- Public Complete + ExecuteWith API for embedding

## [4.4.0] - 2026-03-30

- PrefixHandler propagation to subcommands, CLAUSES section in help

## [4.3.7] - 2026-03-31

- Positional completions alongside subcommand names

## [4.3.6] - 2026-03-30

- Always show the CLAUSES section in help ("not supported" when a command has no local flags)

## [4.3.5] - 2026-03-30

- Route single-argument `+flag` flags through the map path, so negation works

## [4.3.4] - 2026-03-29

- Propagate PrefixHandler to subcommands

## [4.3.3] - 2025-12-14

- Save/restore outer field cache across process substitutions

## [4.3.2] - 2025-12-14

- Simplify process substitution cache handling for piped commands

## [4.3.1] - 2025-12-14

- Fix process substitution field cache isolation

## [4.3.0] - 2025-12-13

- Automatic field caching on single data file completion

## [4.2.1] - 2025-12-12

- Add GetSubcommand() public method for testing

## [4.2.0] - 2025-11-30

- Clean shell-safe completions with printf %q

## [4.1.2] - 2025-11-26

- Fix bash quoting with single quotes

## [4.1.1] - 2025-11-26

- Auto-escape bash special characters

## [4.1.0] - 2025-11-26

- Filepath caching and special character support

## [4.0.2] - 2025-11-26

- Field value caching for pipelines

## [4.0.1] - 2025-11-25

- Fix completion function guard

## [4.0.0] - 2025-11-25

- Field Value Completion and JSON Directives

## Earlier versions

- **v3.x** — see the git tags (`git tag -l 'v3*'`).
- **v2.1.1** — fluent API return types: Handler() and Example() return
  concrete types, so Subcommand().Example().Flag() chains.
- **v2.1.0** — nested subcommands (git remote add style); breaking:
  Context.Subcommand → Context.SubcommandPath; IsSubcommand(),
  IsSubcommandPath(), SubcommandName(); docs/MIGRATION_v2.0_to_v2.1.md.
- **v2.0.0** — breaking: Bind() and reflection removed; type-safe helpers
  (GetBool, GetString, RequireString, …).
- **v1.0.0** — first stable release; the package-level `Version` constant.
- **v0.2.1** — Unix `--`: Context.RemainingArgs.
- **v0.2.0** — subcommands, three-level flag scoping, clause support,
  completion and help for subcommands.
- **v0.1.0** — initial release: clauses, positional arguments.
