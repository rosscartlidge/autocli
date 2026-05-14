package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// shellPrefs is the on-disk shape of per-user shell preferences. Kept
// flat and JSON-tagged so new fields can be added without breaking
// existing files.
type shellPrefs struct {
	EditingMode string `json:"editing_mode,omitempty"`
}

// loadPrefs reads path and returns the parsed EditingMode if present.
// Missing file or parse errors return (0, false) — the caller falls
// back to the Options-level default. Never returns an error: prefs
// are advisory, a corrupt file shouldn't prevent the user from
// connecting.
func loadPrefs(path string) (EditingMode, bool) {
	if path == "" {
		return EditingEmacs, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return EditingEmacs, false
	}
	var p shellPrefs
	if err := json.Unmarshal(data, &p); err != nil {
		return EditingEmacs, false
	}
	switch p.EditingMode {
	case "vi":
		return EditingVi, true
	case "emacs":
		return EditingEmacs, true
	}
	return EditingEmacs, false
}

// savePrefs writes the current EditingMode to path. Creates parent
// directories with 0700. The file itself is 0600. Atomic via
// tmp-file-and-rename so a crash mid-write can't corrupt the prefs.
//
// Returns nil on success or the underlying I/O error — the shell
// loop logs but doesn't surface it to the user (prefs are best-
// effort, the in-memory state still works).
func savePrefs(path string, mode EditingMode) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	p := shellPrefs{EditingMode: editingModeName(mode)}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func editingModeName(m EditingMode) string {
	if m == EditingVi {
		return "vi"
	}
	return "emacs"
}
