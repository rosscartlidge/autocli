package completionflags

import (
	"slices"
	"testing"
)

// declutterFlags models a subcommand whose own options are mixed with an
// inherited (demoted) root global and a positional — the shape that used to
// drown the real options on a broad `-<TAB>`.
func declutterFlags() []*FlagSpec {
	return []*FlagSpec{
		{Names: []string{"-default-type", "-dt"}, Scope: ScopeGlobal},
		{Names: []string{"-source"}, Scope: ScopeGlobal},
		{Names: []string{"-generate", "-g"}, Scope: ScopeGlobal},
		{Names: []string{"-secret"}, Scope: ScopeGlobal, Hidden: true},
		{Names: []string{"FILE"}, Scope: ScopeGlobal}, // positional
		{Names: []string{"-verbose", "-v"}, Scope: ScopeGlobal, demoted: true},
	}
}

func has(s []string, v string) bool { return slices.Contains(s, v) }

func TestCompleteFlagSet_BroadShowsPrimariesOnly(t *testing.T) {
	got := completeFlagSet(declutterFlags(), "-")

	// Foreground: each own flag's primary name, plus a single --help.
	for _, want := range []string{"-default-type", "-source", "-generate", "--help"} {
		if !has(got, want) {
			t.Errorf("broad list missing %q: %v", want, got)
		}
	}
	// Background must NOT appear on a broad prefix.
	for _, no := range []string{
		"-dt", "-g", // aliases
		"-verbose", "-v", // demoted root global
		"-help", "-h", "-man", "-completion-script", // meta built-ins
		"FILE",    // positional
		"-secret", // hidden
	} {
		if has(got, no) {
			t.Errorf("broad list should not contain %q: %v", no, got)
		}
	}
}

func TestCompleteFlagSet_AliasOnPrefix(t *testing.T) {
	// Typing the alias prefix surfaces both primary and alias.
	got := completeFlagSet(declutterFlags(), "-d")
	for _, want := range []string{"-default-type", "-dt"} {
		if !has(got, want) {
			t.Errorf("-d completion missing %q: %v", want, got)
		}
	}
	// The exact alias still completes.
	if got := completeFlagSet(declutterFlags(), "-dt"); !has(got, "-dt") {
		t.Errorf("-dt should still complete: %v", got)
	}
}

func TestCompleteFlagSet_DemotedGlobalOnPrefix(t *testing.T) {
	got := completeFlagSet(declutterFlags(), "-v")
	for _, want := range []string{"-verbose", "-v"} {
		if !has(got, want) {
			t.Errorf("-v completion missing demoted global %q: %v", want, got)
		}
	}
}

func TestCompleteFlagSet_HelpBuiltinsOnPrefix(t *testing.T) {
	got := completeFlagSet(declutterFlags(), "-h")
	for _, want := range []string{"-help", "-h"} {
		if !has(got, want) {
			t.Errorf("-h completion missing %q: %v", want, got)
		}
	}
	// -man / -completion-script only on their own prefix.
	if got := completeFlagSet(declutterFlags(), "-m"); !has(got, "-man") {
		t.Errorf("-m should complete -man: %v", got)
	}
	if got := completeFlagSet(declutterFlags(), "-"); has(got, "-man") {
		t.Errorf("-man should not appear on broad prefix: %v", got)
	}
}

func TestCompleteFlagSet_HiddenNeverShown(t *testing.T) {
	if got := completeFlagSet(declutterFlags(), "-s"); has(got, "-secret") {
		t.Errorf("hidden flag must never complete: %v", got)
	}
}
