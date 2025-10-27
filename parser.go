package completionflags

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Execute parses arguments and runs the handler
func (cmd *Command) Execute(args []string) error {
	// Check for special flags first
	if len(args) > 0 {
		switch args[0] {
		case "-help", "--help", "-h":
			fmt.Println(cmd.GenerateHelp())
			return nil
		case "-man":
			fmt.Println(cmd.GenerateManPage())
			return nil
		case "-complete":
			if len(args) < 2 {
				return fmt.Errorf("-complete requires position argument")
			}
			return cmd.handleCompletion(args[1:])
		case "-completion-script":
			fmt.Print(cmd.GenerateCompletionScript())
			return nil
		}
	}

	// Parse into clauses
	ctx, err := cmd.Parse(args)
	if err != nil {
		return err
	}

	// Validate
	if err := cmd.validate(ctx); err != nil {
		return err
	}

	// Bind values to pointers
	if err := cmd.bindValues(ctx); err != nil {
		return err
	}

	// Execute handler
	return cmd.handler(ctx)
}

// Parse breaks arguments into clauses
func (cmd *Command) Parse(args []string) (*Context, error) {
	ctx := &Context{
		Command:        cmd,
		Clauses:        []Clause{},
		GlobalFlags:    make(map[string]interface{}),
		RawArgs:        args,
		deferredValues: make(map[string]*deferredValue),
	}

	currentClause := Clause{
		Separator:  "",
		Flags:      make(map[string]interface{}),
		Positional: []string{},
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		// Check if this is a clause separator
		if cmd.isSeparator(arg) {
			// Save current clause
			ctx.Clauses = append(ctx.Clauses, currentClause)

			// Start new clause
			currentClause = Clause{
				Separator:  arg,
				Flags:      make(map[string]interface{}),
				Positional: []string{},
			}
			i++
			continue
		}

		// Check if this is a flag (starts with - or +)
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "+") {
			consumed, err := cmd.parseFlag(args, i, &currentClause, ctx)
			if err != nil {
				return nil, err
			}
			i += consumed
			continue
		}

		// Positional argument
		currentClause.Positional = append(currentClause.Positional, arg)
		i++
	}

	// Save final clause
	ctx.Clauses = append(ctx.Clauses, currentClause)

	// Resolve deferred values now that all flags are parsed
	if err := cmd.resolveDeferredValues(ctx); err != nil {
		return nil, err
	}

	// Match positional arguments to positional flag specs
	if err := cmd.matchPositionals(ctx); err != nil {
		return nil, err
	}

	// Apply defaults
	cmd.applyDefaults(ctx)

	return ctx, nil
}

// parseFlag handles both -flag and +flag
func (cmd *Command) parseFlag(args []string, pos int, clause *Clause, ctx *Context) (int, error) {
	flagArg := args[pos]
	hasPlus := strings.HasPrefix(flagArg, "+")

	// Normalize to find flag spec (remove prefix)
	normalized := flagArg
	if hasPlus {
		normalized = "-" + flagArg[1:]
	}

	spec := cmd.findFlagSpec(normalized)
	if spec == nil {
		return 0, ParseError{
			Flag:    flagArg,
			Message: "unknown flag",
		}
	}

	// Determine target storage based on scope
	var target map[string]interface{}
	if spec.Scope == ScopeGlobal {
		target = ctx.GlobalFlags
	} else {
		target = clause.Flags
	}

	// Parse based on argument count
	if spec.ArgCount == 0 {
		// Boolean flag - no arguments
		value := true
		finalValue := cmd.getPrefixHandler()(spec.Names[0], hasPlus, value)
		target[spec.Names[0]] = finalValue
		return 1, nil
	}

	// Check we have enough arguments
	if pos+spec.ArgCount >= len(args) {
		return 0, ParseError{
			Flag:    flagArg,
			Message: fmt.Sprintf("requires %d argument(s)", spec.ArgCount),
		}
	}

	// Parse arguments
	if spec.ArgCount == 1 {
		// Check if we need deferred parsing
		needsDeferred := spec.ArgTypes[0] == ArgTime && spec.TimeZoneFromFlag != ""

		if needsDeferred {
			// Store for deferred parsing
			clauseIdx := len(ctx.Clauses) // Current clause index
			ctx.deferredValues[spec.Names[0]] = &deferredValue{
				rawString:   args[pos+1],
				spec:        spec,
				isGlobal:    spec.Scope == ScopeGlobal,
				clauseIndex: clauseIdx,
			}
			// Don't parse yet, just consume the argument
			return 1 + spec.ArgCount, nil
		}

		// Single argument - parse immediately
		value, err := parseArgValue(args[pos+1], spec.ArgTypes[0], spec, ctx.GlobalFlags)
		if err != nil {
			return 0, ParseError{
				Flag:    flagArg,
				Message: fmt.Sprintf("invalid argument: %v", err),
			}
		}

		// Apply prefix handler
		finalValue := cmd.getPrefixHandler()(spec.Names[0], hasPlus, value)

		// Handle slices (accumulate) vs single values (replace)
		if spec.IsSlice {
			existing, ok := target[spec.Names[0]]
			if !ok {
				target[spec.Names[0]] = []interface{}{finalValue}
			} else {
				slice := existing.([]interface{})
				target[spec.Names[0]] = append(slice, finalValue)
			}
		} else {
			target[spec.Names[0]] = finalValue
		}

		return 1 + spec.ArgCount, nil
	}

	// Multi-argument flag
	argMap := make(map[string]interface{})
	for i := 0; i < spec.ArgCount; i++ {
		value, err := parseArgValue(args[pos+1+i], spec.ArgTypes[i], spec, ctx.GlobalFlags)
		if err != nil {
			return 0, ParseError{
				Flag:    flagArg,
				Message: fmt.Sprintf("invalid argument %d: %v", i, err),
			}
		}
		argMap[spec.ArgNames[i]] = value
	}

	// Apply prefix handler
	finalValue := cmd.getPrefixHandler()(spec.Names[0], hasPlus, argMap)

	// Handle slices (accumulate) vs single values (replace)
	if spec.IsSlice {
		existing, ok := target[spec.Names[0]]
		if !ok {
			target[spec.Names[0]] = []interface{}{finalValue}
		} else {
			slice := existing.([]interface{})
			target[spec.Names[0]] = append(slice, finalValue)
		}
	} else {
		target[spec.Names[0]] = finalValue
	}

	return 1 + spec.ArgCount, nil
}

// parseArgValue converts a string to the appropriate type
func parseArgValue(value string, argType ArgType, spec *FlagSpec, globalFlags map[string]interface{}) (interface{}, error) {
	switch argType {
	case ArgString:
		return value, nil
	case ArgInt:
		return strconv.Atoi(value)
	case ArgFloat:
		return strconv.ParseFloat(value, 64)
	case ArgBool:
		return strconv.ParseBool(value)
	case ArgDuration:
		return time.ParseDuration(value)
	case ArgTime:
		return parseTimeValue(value, spec, globalFlags)
	default:
		return value, nil
	}
}

// parseTimeValue parses a time string using the spec's time configuration
func parseTimeValue(value string, spec *FlagSpec, globalFlags map[string]interface{}) (time.Time, error) {
	formats := spec.TimeFormats
	if len(formats) == 0 {
		formats = []string{time.RFC3339} // Default format
	}

	// Determine timezone
	timezone := spec.TimeZone

	// Try to resolve timezone from TimeZoneFromFlag
	if spec.TimeZoneFromFlag != "" {
		// Look up the timezone from globalFlags
		if tzValue, ok := globalFlags[spec.TimeZoneFromFlag]; ok {
			if tzStr, ok := tzValue.(string); ok {
				timezone = tzStr
			}
		}
	}

	if timezone == "" {
		timezone = "Local"
	}

	// Load location
	var loc *time.Location
	if timezone == "Local" {
		loc = time.Local
	} else {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
		}
	}

	// Try each format
	var lastErr error
	for _, format := range formats {
		t, err := time.ParseInLocation(format, value, loc)
		if err == nil {
			return t, nil // Success!
		}
		lastErr = err
	}

	return time.Time{}, fmt.Errorf("could not parse %q with any format: %w", value, lastErr)
}

// resolveDeferredValues parses values that were deferred because they depend on other flags
func (cmd *Command) resolveDeferredValues(ctx *Context) error {
	for flagName, deferred := range ctx.deferredValues {
		// Now that all flags are parsed, we can resolve dependencies
		value, err := parseArgValue(deferred.rawString, deferred.spec.ArgTypes[0],
			deferred.spec, ctx.GlobalFlags)
		if err != nil {
			return ParseError{
				Flag:    flagName,
				Message: fmt.Sprintf("deferred parsing failed: %v", err),
			}
		}

		// Store in correct location
		if deferred.isGlobal {
			ctx.GlobalFlags[flagName] = value
		} else {
			// For local flags, store in the appropriate clause
			if deferred.clauseIndex < len(ctx.Clauses) {
				ctx.Clauses[deferred.clauseIndex].Flags[flagName] = value
			}
		}
	}
	return nil
}

// applyDefaults applies default values to flags that weren't specified
func (cmd *Command) applyDefaults(ctx *Context) {
	for _, spec := range cmd.flags {
		if spec.Default == nil {
			continue
		}

		if spec.Scope == ScopeGlobal {
			if _, exists := ctx.GlobalFlags[spec.Names[0]]; !exists {
				ctx.GlobalFlags[spec.Names[0]] = spec.Default
			}
		} else {
			// Apply to each clause
			for i := range ctx.Clauses {
				if _, exists := ctx.Clauses[i].Flags[spec.Names[0]]; !exists {
					ctx.Clauses[i].Flags[spec.Names[0]] = spec.Default
				}
			}
		}
	}
}

// validate checks required flags and runs custom validators
func (cmd *Command) validate(ctx *Context) error {
	for _, spec := range cmd.flags {
		if spec.Required {
			if spec.Scope == ScopeGlobal {
				if _, exists := ctx.GlobalFlags[spec.Names[0]]; !exists {
					return ValidationError{
						Flag:    spec.Names[0],
						Message: "required flag not provided",
					}
				}
			} else {
				// For local flags, check at least one clause has it
				found := false
				for _, clause := range ctx.Clauses {
					if _, exists := clause.Flags[spec.Names[0]]; exists {
						found = true
						break
					}
				}
				if !found {
					return ValidationError{
						Flag:    spec.Names[0],
						Message: "required flag not provided in any clause",
					}
				}
			}
		}

		// Run custom validator if provided
		if spec.Validator != nil {
			if spec.Scope == ScopeGlobal {
				if value, exists := ctx.GlobalFlags[spec.Names[0]]; exists {
					if err := spec.Validator(value); err != nil {
						return ValidationError{
							Flag:    spec.Names[0],
							Message: err.Error(),
						}
					}
				}
			} else {
				// Validate in each clause
				for i, clause := range ctx.Clauses {
					if value, exists := clause.Flags[spec.Names[0]]; exists {
						if err := spec.Validator(value); err != nil {
							return ValidationError{
								Flag:    fmt.Sprintf("%s (clause %d)", spec.Names[0], i),
								Message: err.Error(),
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// bindValues binds parsed values to pointers using reflection
func (cmd *Command) bindValues(ctx *Context) error {
	for _, spec := range cmd.flags {
		if spec.Pointer == nil {
			continue
		}

		ptr := reflect.ValueOf(spec.Pointer)
		if ptr.Kind() != reflect.Ptr {
			continue
		}

		elem := ptr.Elem()
		if !elem.CanSet() {
			continue
		}

		var value interface{}
		var exists bool

		if spec.Scope == ScopeGlobal {
			value, exists = ctx.GlobalFlags[spec.Names[0]]
		} else {
			// For local scope, bind from first clause (or could be last, user's choice)
			// TODO: This might need to be configurable
			if len(ctx.Clauses) > 0 {
				value, exists = ctx.Clauses[0].Flags[spec.Names[0]]
			}
		}

		if !exists {
			continue
		}

		// Set the value using reflection
		if err := setValue(elem, value); err != nil {
			return fmt.Errorf("binding %s: %w", spec.Names[0], err)
		}
	}

	return nil
}

// matchPositionals matches positional arguments to positional flag specs
func (cmd *Command) matchPositionals(ctx *Context) error {
	positionalSpecs := cmd.positionalFlags()
	if len(positionalSpecs) == 0 {
		return nil // No positional specs defined
	}

	// Separate global and local positionals
	var globalPositionals []*FlagSpec
	var localPositionals []*FlagSpec
	for _, spec := range positionalSpecs {
		if spec.Scope == ScopeGlobal {
			globalPositionals = append(globalPositionals, spec)
		} else {
			localPositionals = append(localPositionals, spec)
		}
	}

	// Match global positionals once (from first clause's positional args)
	if len(globalPositionals) > 0 && len(ctx.Clauses) > 0 {
		// Collect all positional args from the first clause
		positionalArgs := ctx.Clauses[0].Positional
		consumed, err := cmd.matchPositionalsToSpecs(globalPositionals, positionalArgs, ctx.GlobalFlags)
		if err != nil {
			return err
		}
		// Remove consumed args from first clause
		ctx.Clauses[0].Positional = positionalArgs[consumed:]
	}

	// Match local positionals in each clause
	if len(localPositionals) > 0 {
		for i := range ctx.Clauses {
			positionalArgs := ctx.Clauses[i].Positional
			_, err := cmd.matchPositionalsToSpecs(localPositionals, positionalArgs, ctx.Clauses[i].Flags)
			if err != nil {
				return fmt.Errorf("clause %d: %w", i, err)
			}
			// Note: we keep remaining positional args in clause for backward compatibility
		}
	}

	return nil
}

// matchPositionalsToSpecs matches positional arguments to specs and stores in target map
func (cmd *Command) matchPositionalsToSpecs(specs []*FlagSpec, args []string, target map[string]interface{}) (int, error) {
	argIndex := 0

	for _, spec := range specs {
		if spec.IsVariadic {
			// Variadic consumes all remaining args
			if argIndex < len(args) {
				remaining := args[argIndex:]
				values := make([]interface{}, len(remaining))
				for i, arg := range remaining {
					val, err := parseArgValue(arg, spec.ArgTypes[0], spec, nil)
					if err != nil {
						return argIndex, ParseError{
							Flag:    spec.Names[0],
							Message: fmt.Sprintf("invalid value %q: %v", arg, err),
						}
					}
					values[i] = val
				}
				target[spec.Names[0]] = values
				argIndex = len(args) // Consumed all
			}
			// If no args left, variadic gets empty slice (handled by defaults)
			break
		}

		// Non-variadic: consume one arg
		if argIndex < len(args) {
			val, err := parseArgValue(args[argIndex], spec.ArgTypes[0], spec, nil)
			if err != nil {
				return argIndex, ParseError{
					Flag:    spec.Names[0],
					Message: fmt.Sprintf("invalid value %q: %v", args[argIndex], err),
				}
			}
			target[spec.Names[0]] = val
			argIndex++
		}
		// If no arg available, leave unset (will use default or fail validation if required)
	}

	return argIndex, nil
}

// setValue sets a reflect.Value from an interface{} value
func setValue(target reflect.Value, value interface{}) error {
	if value == nil {
		return nil
	}

	valueReflect := reflect.ValueOf(value)

	// Handle direct assignment if types match
	if valueReflect.Type().AssignableTo(target.Type()) {
		target.Set(valueReflect)
		return nil
	}

	// Handle type conversions
	switch target.Kind() {
	case reflect.String:
		if v, ok := value.(string); ok {
			target.SetString(v)
			return nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Special case: time.Duration is an int64
		if target.Type() == reflect.TypeOf(time.Duration(0)) {
			if v, ok := value.(time.Duration); ok {
				target.Set(reflect.ValueOf(v))
				return nil
			}
		}
		if v, ok := value.(int); ok {
			target.SetInt(int64(v))
			return nil
		}
	case reflect.Float32, reflect.Float64:
		if v, ok := value.(float64); ok {
			target.SetFloat(v)
			return nil
		}
	case reflect.Bool:
		if v, ok := value.(bool); ok {
			target.SetBool(v)
			return nil
		}
	case reflect.Struct:
		// Special case: time.Time
		if target.Type() == reflect.TypeOf(time.Time{}) {
			if v, ok := value.(time.Time); ok {
				target.Set(reflect.ValueOf(v))
				return nil
			}
		}
	case reflect.Slice:
		// Handle slice types
		if slice, ok := value.([]interface{}); ok {
			newSlice := reflect.MakeSlice(target.Type(), len(slice), len(slice))
			for i, item := range slice {
				if err := setValue(newSlice.Index(i), item); err != nil {
					return err
				}
			}
			target.Set(newSlice)
			return nil
		}
	}

	return fmt.Errorf("cannot assign %T to %v", value, target.Type())
}
