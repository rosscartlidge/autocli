package shell

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// TestVersionMatchesChangelog is the release gate (the same as autocli's):
// Version equals the newest released CHANGELOG.md entry, the changelog
// keeps an [Unreleased] section, and on a clean checkout any shell/v* tag on
// HEAD names Version. Run after tagging and before pushing.
func TestVersionMatchesChangelog(t *testing.T) {
	b, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	if !strings.Contains(log, "\n## [Unreleased]\n") {
		t.Error("CHANGELOG.md has no ## [Unreleased] section")
	}
	m := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`).FindStringSubmatch(log)
	if m == nil {
		t.Fatal("CHANGELOG.md has no released entry (## [X.Y.Z] - YYYY-MM-DD)")
	}
	if m[1] != Version {
		t.Errorf("Version = %q but the newest CHANGELOG.md entry is %s", Version, m[1])
	}
	// A tag describes a clean checkout of its commit; with edits on top
	// (the next release under way) HEAD's tag is the previous release.
	dirty, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		t.Skipf("no git checkout to compare tags: %v", err)
	}
	if len(strings.TrimSpace(string(dirty))) > 0 {
		return
	}
	out, err := exec.Command("git", "tag", "--points-at", "HEAD").Output()
	if err != nil {
		t.Skipf("git tag: %v", err)
	}
	for _, tag := range strings.Fields(string(out)) {
		if strings.HasPrefix(tag, "shell/v") && tag != "shell/v"+Version {
			t.Errorf("HEAD is tagged %s but Version = %q", tag, Version)
		}
	}
}
