package shell

// Setting describes a runtime-configurable knob on a service.
// Operators inspect and change settings via the `:set` built-in:
//
//	> :set                    list all settings + current values
//	> :set log-level          show the current log-level
//	> :set log-level debug    change it (Set is called with "debug")
//
// Implementations supply Get and Set. The shell layer handles the
// UX (formatting, listing, error reporting from Set). Get and Set
// may be called from multiple sessions concurrently — the service
// is responsible for any locking around the underlying value.
type Setting struct {
	// Name is the operator-facing identifier. Conventionally
	// lowercase, hyphen-separated ("log-level", "row-cap").
	Name string

	// Description is shown in the `:set` listing. One short line.
	Description string

	// Get returns the current value formatted for display. Called
	// from a session goroutine; must be safe to call concurrently
	// with Set.
	Get func() string

	// Set applies a new value. Called with the raw string the
	// operator typed after the setting name. Return an error to
	// surface "invalid value" feedback to the operator without
	// taking down the session.
	Set func(value string) error
}
