package errors

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// UnpackedError represents the decomposed structure of an error.
// It breaks down complex errors into their constituent parts for easier formatting and analysis.
// This struct is used internally by formatting functions to organize error information.
//
// Fields:
//   - ErrExternal (error): the first external (non-package) error found in the chain;
//     traversal stops there, even if that error itself wraps further errors
//   - ErrRoot (ErrPart): the root error part, if present
//   - ErrChain ([]ErrPart): the chain of wrapped error parts
//   - ErrJoined ([]error): list of joined errors, if the error is a joined type
type UnpackedError struct {
	ErrExternal error
	ErrRoot     ErrPart
	ErrChain    []ErrPart
	ErrJoined   []error
}

// ErrPart represents a single component of an error, either root or wrapped.
// It encapsulates the key details of that error segment for formatting.
//
// Fields:
//   - Message (string): the error message for this part
//   - Type (Type): the classification type of this error part
//   - Fields (map[string]any): structured key-value fields associated with this part
//   - Stack (Stack): the stack trace frames for this error part
type ErrPart struct {
	Message string
	Type    Type
	Fields  map[string]any
	Stack   Stack
}

// Formatter is responsible for converting errors into human-readable string or JSON formats.
// It uses configurable options to control the output structure and content.
//
// Fields:
//   - options (*FormatterOptions): the configuration options for formatting
type Formatter struct {
	options *FormatterOptions
}

// String formats the error as a multi-line string.
// It handles both chain and joined errors differently.
//
// Parameters:
//   - err (error): the error to format
//
// Returns:
//   - formatted (string): the formatted string representation, or empty if err is nil
func (f *Formatter) String(err error) (formatted string) {
	if err == nil {
		return ""
	}

	if e, ok := err.(*joined); ok {
		return f.formatJoinedString(e)
	}

	return f.formatChainString(err)
}

// JSON formats the error as a map suitable for JSON encoding.
// It handles both chain and joined errors differently.
//
// Parameters:
//   - err (error): the error to format
//
// Returns:
//   - formatted (map[string]any): the formatted map, or nil if err is nil
func (f *Formatter) JSON(err error) (formatted map[string]any) {
	if err == nil {
		return nil
	}

	if e, ok := err.(*joined); ok {
		return f.formatJoinedJSON(e)
	}

	return f.formatChainJSON(err)
}

// formatChainString formats a chain error (root + wraps) into a string.
// It unpacks the error and assembles parts based on options (e.g., order, external inclusion).
//
// Parameters:
//   - err (error): the chain error to format
//
// Returns:
//   - (string): the formatted string
func (f *Formatter) formatChainString(err error) string {
	unpacked := unpack(err, f.options.WithTrace)

	parts := make([]string, 0, len(unpacked.ErrChain)+2)

	if f.options.IsInnerFirst {
		if unpacked.ErrExternal != nil && (f.options.WithExternal || f.isOnlyExternal(&unpacked)) {
			parts = append(parts, f.formatExternalString(unpacked.ErrExternal))
		}

		if f.hasRootContent(&unpacked.ErrRoot) {
			parts = append(parts, f.formatPartString(&unpacked.ErrRoot, "root"))
		}

		for i := len(unpacked.ErrChain) - 1; i >= 0; i-- {
			parts = append(parts, f.formatPartString(&unpacked.ErrChain[i], "wrap"))
		}
	} else {
		for i := range len(unpacked.ErrChain) {
			parts = append(parts, f.formatPartString(&unpacked.ErrChain[i], "wrap"))
		}

		if f.hasRootContent(&unpacked.ErrRoot) {
			parts = append(parts, f.formatPartString(&unpacked.ErrRoot, "root"))
		}

		if unpacked.ErrExternal != nil && (f.options.WithExternal || f.isOnlyExternal(&unpacked)) {
			parts = append(parts, f.formatExternalString(unpacked.ErrExternal))
		}
	}

	separator := "\n\n"

	return strings.Join(parts, separator)
}

// formatPartString formats a single ErrPart into a string.
// It includes type, message, fields (sorted by key for stable output), and
// optional trace.
//
// Parameters:
//   - part (*ErrPart): the error part to format
//   - kind (string): the kind of part ("root" or "wrap") for trace labeling
//
// Returns:
//   - (string): the formatted string for this part
func (f *Formatter) formatPartString(part *ErrPart, kind string) string {
	var buf strings.Builder

	if part.Type != "" {
		buf.WriteString("[")
		buf.WriteString(string(part.Type))
		buf.WriteString("]")
		buf.WriteString(f.options.Spacing)
	}

	buf.WriteString(part.Message)

	if len(part.Fields) > 0 {
		buf.WriteString("\n\nFields:")

		for _, k := range slices.Sorted(maps.Keys(part.Fields)) {
			fmt.Fprintf(&buf, "\n%s%s:%s%v", f.options.Indentation, k, f.options.Spacing, part.Fields[k])
		}
	}

	if f.options.WithTrace && len(part.Stack) > 0 {
		frames := part.Stack

		fmt.Fprintf(&buf, "\n\n%s Trace:", kind)

		for _, frame := range frames {
			fmt.Fprintf(&buf, "\n%s%s%s(%s:%d)", f.options.Indentation, frame.Name, f.options.Spacing, frame.File, frame.Line)
		}
	}

	return buf.String()
}

// formatExternalString formats an external error into a string.
// It includes trace if configured, otherwise just the error message.
//
// Parameters:
//   - err (error): the external error to format
//
// Returns:
//   - (string): the formatted string
func (f *Formatter) formatExternalString(err error) string {
	if f.options.WithTrace {
		return fmt.Sprintf("%+v", err)
	}

	return err.Error()
}

// formatJoinedString formats a joined error into a string.
// It includes the count, optional join location, and formats each sub-error recursively.
//
// Parameters:
//   - joinErr (*joined): the joined error to format
//
// Returns:
//   - (string): the formatted string
func (f *Formatter) formatJoinedString(joinErr *joined) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Multiple errors (%d):", len(joinErr.errs))

	if f.options.WithTrace && joinErr.trace != nil {
		frames := joinErr.trace.resolveToStackFrames()

		if len(frames) > 0 {
			buf.WriteString("\n\nJoin Location:")

			frame := frames[0]

			fmt.Fprintf(&buf, "\n%s%s%s(%s:%d)", f.options.Indentation, frame.Name, f.options.Spacing, frame.File, frame.Line)
		}
	}

	for i, err := range joinErr.errs {
		if err == nil {
			continue
		}

		fmt.Fprintf(&buf, "\n\n%d. %s", i+1, f.String(err))
	}

	return buf.String()
}

// formatChainJSON formats a chain error into a JSON-compatible map.
// It unpacks the error and structures it with optional reversal based on options.
//
// Parameters:
//   - err (error): the chain error to format
//
// Returns:
//   - (map[string]any): the formatted map
func (f *Formatter) formatChainJSON(err error) map[string]any {
	unpacked := unpack(err, f.options.WithTrace)
	result := make(map[string]any)

	if unpacked.ErrExternal != nil && (f.options.WithExternal || f.isOnlyExternal(&unpacked)) {
		result["external"] = map[string]any{
			"message": unpacked.ErrExternal.Error(),
			"go_type": fmt.Sprintf("%T", unpacked.ErrExternal),
		}
	}

	if f.hasRootContent(&unpacked.ErrRoot) {
		result["root"] = f.formatPartJSON(&unpacked.ErrRoot)
	}

	if len(unpacked.ErrChain) > 0 {
		chain := make([]map[string]any, 0, len(unpacked.ErrChain))

		for _, part := range unpacked.ErrChain {
			chain = append(chain, f.formatPartJSON(&part))
		}

		if f.options.IsInnerFirst {
			slices.Reverse(chain)
		}

		result["chain"] = chain
	}

	return result
}

// formatPartJSON formats a single ErrPart into a JSON-compatible map.
// It includes message, type, fields, and optional stack with possible inversion.
//
// Parameters:
//   - part (*ErrPart): the error part to format
//
// Returns:
//   - (map[string]any): the formatted map
func (f *Formatter) formatPartJSON(part *ErrPart) map[string]any {
	result := map[string]any{
		"message": part.Message,
	}

	if part.Type != "" {
		result["type"] = string(part.Type)
	}

	if len(part.Fields) > 0 {
		result["fields"] = part.Fields
	}

	if f.options.WithTrace && len(part.Stack) > 0 {
		stack := part.Stack

		frames := make([]map[string]any, 0, len(stack))

		for _, frame := range stack {
			frameMap := map[string]any{
				"function": frame.Name,
				"file":     frame.File,
				"line":     frame.Line,
			}

			frames = append(frames, frameMap)
		}

		if f.options.InvertTrace {
			slices.Reverse(frames)
		}

		result["stack"] = frames
	}

	return result
}

// formatJoinedJSON formats a joined error into a JSON-compatible map.
// It includes type, count, optional join stack, and recursively formats sub-errors.
//
// Parameters:
//   - joinErr (*joined): the joined error to format
//
// Returns:
//   - (map[string]any): the formatted map
func (f *Formatter) formatJoinedJSON(joinErr *joined) map[string]any {
	result := map[string]any{
		"type":  "joined",
		"count": len(joinErr.errs),
	}

	if f.options.WithTrace && joinErr.trace != nil {
		frames := joinErr.trace.resolveToStackFrames()

		if len(frames) > 0 {
			joinFrames := make([]map[string]any, 0, len(frames))

			for _, frame := range frames {
				joinFrames = append(joinFrames, map[string]any{
					"function": frame.Name,
					"file":     frame.File,
					"line":     frame.Line,
				})
			}

			result["join_stack"] = joinFrames
		}
	}

	errs := make([]any, 0, len(joinErr.errs))

	for _, err := range joinErr.errs {
		if err != nil {
			errs = append(errs, f.JSON(err))
		}
	}

	result["errors"] = errs

	return result
}

// hasRootContent checks if the root ErrPart has any meaningful content.
// Used to decide whether to include the root in formatting. It does not depend
// on the stack being resolved, so the decision is stable whether or not traces
// are enabled.
//
// Parameters:
//   - root (*ErrPart): the root part to check
//
// Returns:
//   - (bool): true if it has a message, type, fields, or stack frames
func (f *Formatter) hasRootContent(root *ErrPart) bool {
	return root.Message != "" || root.Type != "" || len(root.Fields) > 0 || len(root.Stack) > 0
}

// isOnlyExternal checks if the unpacked error consists only of an external error.
// Used to decide inclusion when WithExternal is false.
//
// Parameters:
//   - unpacked (*UnpackedError): the unpacked error to check
//
// Returns:
//   - (bool): true if only external error is present
func (f *Formatter) isOnlyExternal(unpacked *UnpackedError) bool {
	return unpacked.ErrExternal != nil && unpacked.ErrRoot.Message == "" && len(unpacked.ErrChain) == 0
}

// FormatterOptions holds configuration for the Formatter.
// It controls aspects like order, trace inclusion, and formatting style.
//
// Fields:
//   - IsInnerFirst (bool): if true, format from inner to outer (default: false)
//   - WithTrace (bool): include stack traces (default: false)
//   - InvertTrace (bool): invert stack trace order (default: false)
//   - WithExternal (bool): include external errors (default: true)
//   - Spacing (string): spacing between elements (default: " ")
//   - Indentation (string): indentation for nested elements (default: "  ")
type FormatterOptions struct {
	IsInnerFirst bool
	WithTrace    bool
	InvertTrace  bool
	WithExternal bool
	Spacing      string
	Indentation  string
}

// FormatterOptionFunc is a function type for configuring FormatterOptions.
// Used with NewFormatter to set custom options. A nil FormatterOptionFunc is
// ignored.
type FormatterOptionFunc func(options *FormatterOptions)

// NewFormatter creates a new Formatter with default or custom options.
// Defaults: outer-first, no trace, no invert, include external, space " ", indent "  ".
//
// Parameters:
//   - ofs (...FormatterOptionFunc): variadic option functions
//
// Returns:
//   - formatter (*Formatter): the new formatter instance
func NewFormatter(ofs ...FormatterOptionFunc) (formatter *Formatter) {
	options := &FormatterOptions{
		IsInnerFirst: false,
		WithTrace:    false,
		InvertTrace:  false,
		WithExternal: true,
		Spacing:      " ",
		Indentation:  "  ",
	}

	for _, f := range ofs {
		if f != nil {
			f(options)
		}
	}

	return &Formatter{
		options: options,
	}
}

// FormatWithTrace returns an option function to enable stack traces.
//
// Returns:
//   - f (FormatterOptionFunc): configuration function for NewFormatter
func FormatWithTrace() (f FormatterOptionFunc) {
	return func(options *FormatterOptions) {
		options.WithTrace = true
	}
}

// FormatWithInnerFirst returns an option function that formats the error chain
// from the innermost cause outward (the default is outermost first).
//
// Returns:
//   - f (FormatterOptionFunc): configuration function for NewFormatter
func FormatWithInnerFirst() (f FormatterOptionFunc) {
	return func(options *FormatterOptions) {
		options.IsInnerFirst = true
	}
}

// FormatWithInvertedTrace returns an option function that renders stack traces
// with the most recent call last (the default is most recent call first).
//
// Returns:
//   - f (FormatterOptionFunc): configuration function for NewFormatter
func FormatWithInvertedTrace() (f FormatterOptionFunc) {
	return func(options *FormatterOptions) {
		options.InvertTrace = true
	}
}

// FormatWithoutExternal returns an option function that omits external
// (non-package) errors from the output, unless the error consists solely of an
// external error.
//
// Returns:
//   - f (FormatterOptionFunc): configuration function for NewFormatter
func FormatWithoutExternal() (f FormatterOptionFunc) {
	return func(options *FormatterOptions) {
		options.WithExternal = false
	}
}

// Unpack decomposes an error into its parts, resolving stack traces.
// It handles joined, root, wrapped, and external errors.
//
// The unpacking process:
//  1. If joined, sets ErrJoined and returns.
//  2. Traverses the chain using Unwrap.
//  3. For root/wrapped, extracts to ErrRoot/ErrChain.
//  4. For external, sets ErrExternal and stops — even if that error wraps
//     further errors (for example a fmt.Errorf %w chain), which are not
//     decomposed.
//
// Parameters:
//   - err (error): the error to unpack
//
// Returns:
//   - uerr (UnpackedError): the unpacked structure
func Unpack(err error) (uerr UnpackedError) {
	return unpack(err, true)
}

// unpack is the internal implementation of Unpack. Symbolizing program counters
// into file/line/function is the expensive part of unpacking, so resolveStacks
// gates it: callers that will not render traces (Formatter with WithTrace false)
// pass false and skip the work entirely.
//
// Parameters:
//   - err (error): the error to unpack
//   - resolveStacks (bool): whether to resolve stack frames into ErrPart.Stack
//
// Returns:
//   - uerr (UnpackedError): the unpacked structure
func unpack(err error, resolveStacks bool) (uerr UnpackedError) {
	if joinErr, ok := err.(*joined); ok {
		uerr.ErrJoined = joinErr.errs

		return uerr
	}

	for err != nil {
		switch e := err.(type) {
		case *root:
			uerr.ErrRoot = ErrPart{
				Type:    e.Type(),
				Message: e.message,
				Fields:  e.Fields(),
			}

			if resolveStacks && e.trace != nil {
				uerr.ErrRoot.Stack = e.trace.resolveToStackFrames()
			}
		case *wrapped:
			part := ErrPart{
				Type:    e.Type(),
				Message: e.message,
				Fields:  e.Fields(),
			}

			if resolveStacks && e.frame != nil {
				part.Stack = Stack{e.frame.resolveToStackFrame()}
			}

			uerr.ErrChain = append(uerr.ErrChain, part)
		default:
			uerr.ErrExternal = err

			return uerr
		}

		err = Unwrap(err)
	}

	return uerr
}

// ToString is a convenience function to format an error as a string.
// It creates a formatter with options and calls String.
//
// Parameters:
//   - err (error): the error to format
//   - ofs (...FormatterOptionFunc): optional configuration
//
// Returns:
//   - formatted (string): the formatted string
func ToString(err error, ofs ...FormatterOptionFunc) (formatted string) {
	formatter := NewFormatter(ofs...)

	formatted = formatter.String(err)

	return
}

// ToJSON is a convenience function to format an error as a JSON map.
// It creates a formatter with options and calls JSON.
//
// Parameters:
//   - err (error): the error to format
//   - ofs (...FormatterOptionFunc): optional configuration
//
// Returns:
//   - formatted (map[string]any): the formatted map
func ToJSON(err error, ofs ...FormatterOptionFunc) (formatted map[string]any) {
	formatter := NewFormatter(ofs...)

	formatted = formatter.JSON(err)

	return
}

// ToJSONString is a convenience function to format an error as a JSON string.
// It uses ToJSON and marshals with indentation.
//
// Parameters:
//   - err (error): the error to format
//   - ofs (...FormatterOptionFunc): optional configuration
//
// Returns:
//   - formatted (string): the JSON string, or error message if marshaling fails
func ToJSONString(err error, ofs ...FormatterOptionFunc) (formatted string) {
	data := ToJSON(err, ofs...)
	if data == nil {
		return
	}

	bytes, jsonErr := json.MarshalIndent(data, "", "  ")
	if jsonErr != nil {
		formatted = fmt.Sprintf("JSON formatting error: %v", jsonErr)

		return
	}

	formatted = string(bytes)

	return
}
