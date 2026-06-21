package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// StackFrame holds metadata for a single call site within a backtrace.
// It contains all the information needed to identify and locate the
// source of a function call in the codebase.
//
// Fields:
//   - Name (string): simplified function name (without package path) for concise display
//   - File (string): full path of the source file where the call originated
//   - Line (int): exact line number in the source file where the call occurred
type StackFrame struct {
	Name string
	File string
	Line int
}

// format outputs a single-line representation of the StackFrame using the
// provided separator, ideal for log lines or multi-line error dumps.
// The format is consistent and parsable: "Name<sep>File<sep>Line".
//
// Parameters:
//   - separator (string): delimiter to use between frame components
//
// Returns:
//   - frame (string): formatted frame information as a single string
func (f *StackFrame) format(separator string) (frame string) {
	frame = fmt.Sprintf("%s%s%s%s%d", f.Name, separator, f.File, separator, f.Line)

	return
}

// Stack represents a high-level, resolved backtrace composed of StackFrame entries.
// It enables formatting and presentation of the full call sequence in a human-readable
// format. The Stack type provides methods for formatting the trace in various ways.
//
// The stack is ordered with the most recent call first (index 0) by default,
// but can be formatted in reverse order when needed.
type Stack []StackFrame

// format serializes the Stack into human-readable strings suitable for logging
// or error messages. It allows customization of the separator between frame elements
// and the order of presentation (natural or reversed).
//
// Parameters:
//   - separator (string): delimiter between frame elements (e.g., " " or "\t")
//   - invert (bool): order flag: true for reverse (most recent call last),
//     false for natural (most recent call first)
//
// Returns:
//   - frames ([]string): formatted lines representing each call frame, ordered according
//     to the invert parameter
func (s Stack) format(separator string, invert bool) (frames []string) {
	n := len(s)

	frames = make([]string, n)

	for i, f := range s {
		idx := i

		if !invert {
			idx = n - 1 - i
		}

		frames[idx] = f.format(separator)
	}

	return
}

// frame represents a single raw PC from the call stack.
// It exposes methods to resolve metadata about that call site.
//
// The frame type is used primarily for capturing individual call sites rather than full traces.
type frame uintptr

// pc computes a valid PC for runtime lookups by subtracting one
// (per the Go runtime's call-instruction convention). This adjustment is necessary
// because the PC recorded during function calls is actually the next instruction after the call.
//
// Returns:
//   - PC (uintptr): adjusted PC for retrieving function details from the runtime
func (f frame) pc() (PC uintptr) {
	PC = uintptr(f) - 1

	return
}

// resolveToStackFrame resolves a single frame into a StackFrame, capturing function name,
// file, and line information. It performs the same name simplification as
// stack.resolveToStackFrames() for consistency.
//
// The resolution process:
//  1. Adjusts the PC for runtime lookup.
//  2. Retrieves the runtime.Frame using runtime.CallersFrames.
//  3. Simplifies the function name by removing the package path.
//  4. Constructs and returns the StackFrame.
//
// Returns:
//   - stackFrame (StackFrame): enriched metadata for this call site containing
//     the simplified function name, source file path, and line number.
func (f frame) resolveToStackFrame() (stackFrame StackFrame) {
	PC := f.pc()

	runtimeFrame, _ := runtime.CallersFrames([]uintptr{PC}).Next()

	name := runtimeFrame.Function

	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	stackFrame = StackFrame{
		Name: name,
		File: runtimeFrame.File,
		Line: runtimeFrame.Line,
	}

	return
}

// stack is a slice of raw program counters recorded from the call stack at the
// point an error was created. Storing unresolved PCs keeps capture cheap; they
// are turned into human-readable frames on demand by resolveToStackFrames, only
// when an error is actually formatted with traces enabled.
type stack []uintptr

// resolveToStackFrames resolves the recorded PCs into a slice of detailed StackFrame objects.
// It iterates over each frame in the call stack, extracts the function name (trimming
// any import path for brevity), source file path, and line number. This enables
// presentation of a clear, ordered trace of calls leading up to an error.
//
// The resolution process:
//  1. Converts raw PCs to runtime.Frame objects using runtime.CallersFrames
//  2. Extracts and simplifies function names by removing package paths
//  3. Constructs StackFrame objects with relevant debug information
//
// Returns:
//   - stackFrameObjects ([]StackFrame): the detailed, ordered frames representing the captured backtrace,
//     with the most recent call first in the slice.
func (s *stack) resolveToStackFrames() (stackFrameObjects []StackFrame) {
	PCs := *s

	runtimeFramesObjects := runtime.CallersFrames(PCs)

	stackFrameObjects = make([]StackFrame, 0, len(PCs))

	for {
		runtimeFrame, more := runtimeFramesObjects.Next()

		name := runtimeFrame.Function

		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}

		stackFrameObjects = append(stackFrameObjects, StackFrame{
			Name: name,
			File: runtimeFrame.File,
			Line: runtimeFrame.Line,
		})

		if !more {
			break
		}
	}

	return
}

// caller captures the immediate caller's frame, skipping over internal frames.
// This is useful for annotating errors with the exact call site in application code.
// The skip parameter allows control over how many stack frames to ascend.
//
// It uses runtime.Caller to retrieve the PC and constructs a frame pointer.
// If no valid caller is found, it returns nil.
//
// Parameters:
//   - skip (int): number of additional application frames to skip (0 = direct caller)
//
// Returns:
//   - (f *frame): pointer to the resolved frame metadata, or nil if no frames available
func caller(skip int) (f *frame) {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return
	}

	v := frame(pc)

	f = &v

	return
}

// callers captures the full application call stack, filtering out runtime internals.
// It returns a stack object that can be further resolved or formatted. The skip
// parameter allows the caller to omit wrapper functions from the trace.
//
// The capture process:
//  1. Uses runtime.Callers to gather up to 64 raw PCs.
//  2. Filters out invalid entries and runtime-internal functions (those prefixed with "runtime.").
//  3. Returns a pointer to the filtered stack.
//
// If no valid frames are found after filtering, an empty stack is returned.
//
// Parameters:
//   - skip (int): number of initial frames to omit (e.g., error wrapper functions)
//
// Returns:
//   - s (*stack): stack of filtered program counters ready for resolution,
//     or empty stack if no frames available
func callers(skip int) (s *stack) {
	const depth = 64

	var PCs [depth]uintptr

	c := runtime.Callers(skip, PCs[:])

	valid := PCs[:c]

	v := make(stack, 0, c)

	for _, PC := range valid {
		fn := runtime.FuncForPC(PC - 1)
		if fn == nil {
			continue
		}

		if strings.HasPrefix(fn.Name(), "runtime.") {
			continue
		}

		v = append(v, PC)
	}

	s = &v

	return
}
