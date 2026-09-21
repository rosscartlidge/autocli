package completionflags

// -spec-json: a machine-readable dump of the registered command tree
// (DFC118 Phase 1 in ssql's numbering). Host tools that want to RENDER
// the grammar — form builders, documentation generators, editors — read
// this instead of keeping their own tables of what commands and flags
// exist (the drift class the FieldSource hooks killed for completion).
// The dump carries declarations only: static enums are inlined (they
// ARE the declaration), dynamic vocabularies (fields, values, files)
// are marked by kind so consumers ask the live completion protocols.

import (
	"encoding/json"
	"io"
	"sort"
)

// SpecArg describes one argument of a flag.
type SpecArg struct {
	Name       string   `json:"name"`
	Type       string   `json:"type,omitempty"`
	Expression bool     `json:"expression,omitempty"`
	Completer  string   `json:"completer"`            // static|file|fields|field-values|none|custom
	Options    []string `json:"options,omitempty"`    // static: the enum, inline
	Pattern    string   `json:"pattern,omitempty"`    // file: glob
	Hint       string   `json:"hint,omitempty"`       // none/file: placeholder hint
	SourceFlag string   `json:"sourceFlag,omitempty"` // fields/field-values: FILE-bearing flag
	FieldArg   string   `json:"fieldArg,omitempty"`   // field-values: arg naming the field
}

// SpecFlag describes a flag (or positional — Names[0] without '-').
type SpecFlag struct {
	Names      []string  `json:"names"`
	Help       string    `json:"help,omitempty"`
	Scope      string    `json:"scope"` // global|local
	Required   bool      `json:"required,omitempty"`
	Bool       bool      `json:"bool,omitempty"`
	Accumulate bool      `json:"accumulate,omitempty"`
	Variadic   bool      `json:"variadic,omitempty"`
	Args       []SpecArg `json:"args,omitempty"`
}

// SpecCommand describes a command level (root or subcommand).
type SpecCommand struct {
	Name              string                  `json:"name"`
	Help              string                  `json:"help,omitempty"`
	ClauseDescription string                  `json:"clauseDescription,omitempty"`
	Separators        []string                `json:"separators,omitempty"`
	Examples          []Example               `json:"examples,omitempty"`
	DisplaySink       string                  `json:"displaySink,omitempty"`
	Flags             []SpecFlag              `json:"flags,omitempty"`
	Subcommands       map[string]*SpecCommand `json:"subcommands,omitempty"`

	// PositionalFlag (root only) is the reserved flag that carries one
	// positional argument by arity on EVERY command: `-arg VALUE`.
	// Builders emitting command lines from data should use it for every
	// positional so no value can be read as a flag or a separator.
	PositionalFlag string `json:"positionalFlag,omitempty"`
}

func specArgFromCompleter(c Completer) SpecArg {
	switch cc := c.(type) {
	case *StaticCompleter:
		return SpecArg{Completer: "static", Options: cc.Options}
	case *FileCompleter:
		return SpecArg{Completer: "file", Pattern: cc.Pattern, Hint: cc.Hint}
	case *NoCompleter:
		return SpecArg{Completer: "none", Hint: cc.Hint}
	case NoCompleter:
		return SpecArg{Completer: "none", Hint: cc.Hint}
	case *FieldValueCompleter:
		return SpecArg{Completer: "field-values", SourceFlag: cc.SourceFlag, FieldArg: cc.FieldArg}
	case *ChainCompleter:
		// The fieldsCompleter shape: upstream + FieldCompleter.
		for _, m := range cc.Completers {
			if fc, ok := m.(*FieldCompleter); ok {
				return SpecArg{Completer: "fields", SourceFlag: fc.SourceFlag}
			}
		}
		return SpecArg{Completer: "custom"}
	case *FieldCompleter:
		return SpecArg{Completer: "fields", SourceFlag: cc.SourceFlag}
	default:
		return SpecArg{Completer: "custom"}
	}
}

func specFlag(fs *FlagSpec) SpecFlag {
	sf := SpecFlag{
		Names:      fs.Names,
		Help:       fs.Description,
		Scope:      map[Scope]string{ScopeGlobal: "global", ScopeLocal: "local"}[fs.Scope],
		Required:   fs.Required,
		Bool:       fs.ArgCount == 0,
		Accumulate: fs.IsSlice,
		Variadic:   fs.IsVariadic,
	}
	for i := 0; i < len(fs.ArgNames); i++ {
		var a SpecArg
		if i < len(fs.ArgCompleters) {
			a = specArgFromCompleter(fs.ArgCompleters[i])
		}
		a.Name = fs.ArgNames[i]
		if i < len(fs.ArgTypes) {
			a.Type = argTypeName(fs.ArgTypes[i])
		}
		if i < len(fs.ArgExpr) && fs.ArgExpr[i] {
			a.Expression = true
		}
		sf.Args = append(sf.Args, a)
	}
	return sf
}

func specSubcommand(name string, sc *Subcommand) *SpecCommand {
	out := &SpecCommand{
		Name:              name,
		Help:              sc.Description,
		ClauseDescription: sc.ClauseDescription,
		Separators:        sc.Separators,
		Examples:          sc.Examples,
		DisplaySink:       sc.DisplaySink,
	}
	for _, fs := range sc.Flags {
		out.Flags = append(out.Flags, specFlag(fs))
	}
	if len(sc.Subcommands) > 0 {
		out.Subcommands = map[string]*SpecCommand{}
		for n, nested := range sc.Subcommands {
			out.Subcommands[n] = specSubcommand(n, nested)
		}
	}
	return out
}

// Spec builds the SpecCommand tree for this command.
func (cmd *Command) Spec() *SpecCommand {
	out := &SpecCommand{
		Name:           cmd.name,
		Help:           cmd.description,
		Separators:     cmd.separators,
		Examples:       cmd.examples,
		PositionalFlag: ArgFlag,
	}
	for _, fs := range cmd.flags {
		out.Flags = append(out.Flags, specFlag(fs))
	}
	if len(cmd.subcommands) > 0 {
		out.Subcommands = map[string]*SpecCommand{}
		names := make([]string, 0, len(cmd.subcommands))
		for n := range cmd.subcommands {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			out.Subcommands[n] = specSubcommand(n, cmd.subcommands[n])
		}
	}
	return out
}

// WriteSpecJSON writes the tree as indented JSON.
func (cmd *Command) WriteSpecJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(cmd.Spec())
}
