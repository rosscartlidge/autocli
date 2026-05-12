package ssh

import (
	"bytes"
	"fmt"
	"os"

	gossh "golang.org/x/crypto/ssh"
)

// buildAuthorizedSet parses an OpenSSH authorized_keys file into a
// set keyed by the marshaled public-key bytes. Empty path returns
// (nil, error) — refuse-to-start safety.
//
// Comments (#-prefix) and blank lines are skipped. Options and
// from= clauses on each line are accepted but ignored (we don't
// enforce them in v0.1).
func buildAuthorizedSet(path string) (map[string]struct{}, error) {
	if path == "" {
		return nil, fmt.Errorf("authorized_keys not configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	set := make(map[string]struct{})
	rest := data
	for len(rest) > 0 {
		// Skip blank / comment lines manually because
		// ParseAuthorizedKey doesn't expose remaining input on those.
		trimmed := bytes.TrimLeft(rest, " \t")
		if len(trimmed) == 0 {
			break
		}
		if trimmed[0] == '\n' || trimmed[0] == '#' {
			// Advance past this line.
			nl := bytes.IndexByte(rest, '\n')
			if nl == -1 {
				break
			}
			rest = rest[nl+1:]
			continue
		}
		key, _, _, remaining, err := gossh.ParseAuthorizedKey(rest)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		set[string(key.Marshal())] = struct{}{}
		rest = remaining
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("%s: no keys found", path)
	}
	return set, nil
}
