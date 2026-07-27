package errors

import (
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

// Stack represents a high-level, resolved backtrace composed of StackFrame entries.
// It enables formatting and presentation of the full call sequence in a human-readable
// format.
//
// The stack is ordered with the most recent call first (index 0).
type Stack []StackFrame

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

	return StackFrame{
		Name: name,
		File: runtimeFrame.File,
		Line: runtimeFrame.Line,
	}
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
// Frames belonging to the Go runtime itself (those prefixed with "runtime.") carry
// no diagnostic value and are dropped here, at resolution time, so that capture in
// callers stays free of symbolization work (see BenchmarkNew).
//
// The resolution process:
//  1. Converts raw PCs to runtime.Frame objects using runtime.CallersFrames
//  2. Drops runtime-internal frames
//  3. Extracts and simplifies function names by removing package paths
//  4. Constructs StackFrame objects with relevant debug information
//
// Returns:
//   - stackFrameObjects ([]StackFrame): the detailed, ordered frames representing the captured backtrace,
//     with the most recent call first in the slice.
func (s stack) resolveToStackFrames() (stackFrameObjects []StackFrame) {
	runtimeFramesObjects := runtime.CallersFrames(s)

	stackFrameObjects = make([]StackFrame, 0, len(s))

	for {
		runtimeFrame, more := runtimeFramesObjects.Next()

		if name := runtimeFrame.Function; !strings.HasPrefix(name, "runtime.") {
			if idx := strings.LastIndex(name, "/"); idx >= 0 {
				name = name[idx+1:]
			}

			stackFrameObjects = append(stackFrameObjects, StackFrame{
				Name: name,
				File: runtimeFrame.File,
				Line: runtimeFrame.Line,
			})
		}

		if !more {
			break
		}
	}

	return stackFrameObjects
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
		return nil
	}

	v := frame(pc)

	return &v
}

// callers captures the application call stack as raw program counters without
// any symbolization: resolving PCs into function/file/line metadata (and
// dropping runtime-internal frames) is deferred to resolveToStackFrames, which
// runs only when a trace is actually rendered. Keeping capture this cheap is
// what allows New and Wrap to stay viable on hot paths (see BenchmarkNew and
// BenchmarkWrap).
//
// At most 64 frames are captured; deeper stacks are silently truncated.
//
// Parameters:
//   - skip (int): number of initial frames to omit (e.g., error wrapper functions)
//
// Returns:
//   - s (*stack): stack of captured program counters ready for lazy resolution,
//     or an empty stack if no frames are available
func callers(skip int) (s *stack) {
	const depth = 64

	var PCs [depth]uintptr

	c := runtime.Callers(skip, PCs[:])

	v := make(stack, 0, c)
	v = append(v, PCs[:c]...)

	return &v
}
