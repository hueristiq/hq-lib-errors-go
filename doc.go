// Package errors provides rich, structured error handling for Go: stack traces,
// error wrapping, type classification, structured key-value fields, and
// multi-error aggregation.
//
// It is a drop-in superset of the standard library's errors package. [Is], [As],
// [Unwrap], and [Join] mirror their standard counterparts and interoperate with
// errors from any source; [Cause] is added for root-cause analysis. Two
// deliberate deviations:
//
//   - [Join] returns a single non-nil error unchanged instead of wrapping it,
//     so the returned value keeps its original identity.
//   - [Is] compares errors created by this package by value, not identity: two
//     distinct errors with the same message and a compatible [Type] match each
//     other. Unlike the standard library's errors.New, calling [New] twice with
//     the same arguments produces matching errors. Create sentinel errors once
//     and reuse them when identity matters.
//
// # Creating errors
//
// [New] creates a root error and captures a stack trace at the call site:
//
//	err := errors.New("connection refused")
//
// Attach a classification type and structured fields at creation with the
// [WithType] and [WithField] options:
//
//	err := errors.New("payment declined",
//	    errors.WithType("PaymentError"),
//	    errors.WithField("order_id", 1234),
//	)
//
// # Wrapping
//
// [Wrap] adds context to an existing error while preserving the original error
// and its stack trace. Wrapping never mutates the wrapped error, so wrapping a
// shared or sentinel error is safe for concurrent use:
//
//	if err := load(); err != nil {
//	    return errors.Wrap(err, "loading configuration")
//	}
//
// # Joining
//
// [Join] aggregates several errors into one that unwraps to all of them. nil
// errors are dropped, and a single non-nil error is returned unchanged:
//
//	err := errors.Join(validateName(), validateAge())
//
// # Inspecting
//
// [Is], [As], and [Unwrap] traverse the error chain, including the branches of
// a joined error. [Cause] follows single-error unwrapping to the deepest error;
// it does not descend into a joined error's branches and returns the joined
// error itself. Errors created by this package also satisfy the [Error]
// interface, which exposes [Error.Type], [Error.Fields], [Error.Stack], and
// [Error.StackFrames] for programmatic handling:
//
//	if errors.Is(err, ErrNotFound) { ... }
//
//	var e errors.Error
//	if errors.As(err, &e) {
//	    log.Println(e.Type(), e.Fields())
//	}
//
// Post-construction mutation is available through the [MutableError] interface,
// though configuring errors at creation time is preferred: mutating a shared
// error changes what every holder of it observes.
//
// # Formatting
//
// [FormatToString], [FormatToJSON], and [FormatToJSONString] render an error chain for logs or
// structured output. Stack traces are omitted by default; enable them with the
// [FormatWithTrace] option:
//
//	fmt.Println(errors.FormatToString(err, errors.FormatWithTrace()))
//
// Errors also implement [fmt.Formatter]: %v and %s print the error message, %q
// prints it quoted, and %+v prints the full chain with stack traces, so
// log.Printf("%+v", err) and similar work as expected.
//
// For finer control over ordering, indentation, and trace inclusion, build a
// [Formatter] with [NewFormatter].
//
// # Stack traces
//
// [New] and [Wrap] capture raw program counters cheaply at the call site and
// resolve them to file, line, and function names lazily — only when an error is
// formatted with traces enabled. A root error records where it was created; each
// [Wrap] records its own call site, and a joined error records where it was
// joined.
//
// # Concurrency
//
// Errors produced by this package are safe for concurrent use.
// [MutableError.SetType] and [MutableError.SetField] mutate under a write lock;
// the matching readers [Error.Type] and [Error.Fields] take a read lock, and
// [Error.Fields] returns a defensive copy so the caller can never observe a
// partially written map. A value's message, cause, and stack trace are
// immutable after construction.
package errors
