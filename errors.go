package errors

import (
	"reflect"
	"strings"
	"sync"
)

// root represents a fundamental error with complete stack trace information.
// It serves as the base error type in the package and implements the Error interface.
//
// Fields:
//   - mu (sync.RWMutex): mutex for thread-safe access to modifiable fields
//   - isGlobal (bool): indicates if error occurred during package initialization
//   - errType (Type): error type for classification (Type)
//   - message (string): human-readable error message
//   - fields (map[string]any): additional structured context (key-value pairs)
//   - cause (error): the underlying error being wrapped (if any)
//   - trace (*stack): captured call stack information
type root struct {
	mu       sync.RWMutex
	isGlobal bool
	errType  Type
	message  string
	fields   map[string]any
	cause    error
	trace    *stack
}

// Type returns the error's classification type if one was set.
// It safely reads the errType field.
//
// Returns:
//   - errType (Type): the error's type, or empty string if untyped or receiver is nil
func (e *root) Type() (errType Type) {
	if e == nil {
		return
	}

	errType = e.errType

	return
}

// Error implements the error interface, returning the error message.
// If the error wraps another error, it combines both messages.
//
// Returns:
//   - msg (string): the error message (or "<nil>" if receiver is nil)
func (e *root) Error() (msg string) {
	msg = "<nil>"

	if e == nil {
		return
	}

	msg = e.message

	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}

	return
}

// Fields returns all structured fields attached to the error.
// The returned map should not be modified directly.
//
// Returns:
//   - fields (map[string]any): all attached fields (may be nil) or nil if receiver is nil
func (e *root) Fields() (fields map[string]any) {
	if e == nil {
		return
	}

	fields = e.fields

	return
}

// StackFrames returns the raw PCs (program counters) from the call stack.
// These can be used to reconstruct the full stack trace.
//
// Returns:
//   - frames ([]uintptr): slice of program counters representing the call stack,
//     or nil if receiver or trace is nil
func (e *root) StackFrames() (frames []uintptr) {
	if e == nil || e.trace == nil {
		return
	}

	frames = *e.trace

	return
}

// Is implements error equality checking. Two errors are considered equal if:
//   - Both are nil, or
//   - They are of the same type (*root), and:
//   - Their types match (or target type is empty), and
//   - Their messages match
//   - Their messages match exactly (fallback)
//
// Parameters:
//   - target (error): the error to compare against
//
// Returns:
//   - matches (bool): true if errors are considered equal
func (e *root) Is(target error) (matches bool) {
	if target == nil {
		matches = e == nil

		return
	}

	if err, ok := target.(*root); ok {
		matches = (err.errType == "" || e.errType == err.errType) && e.message == err.message

		return
	}

	return
}

// As attempts to assign the error to the target interface.
// The target must be a non-nil pointer to either:
//   - An interface type that the error implements, or
//   - A concrete type that matches the error's type
//
// The assignment process:
//  1. Validates that target is a non-nil pointer.
//  2. Checks if the error's type is assignable to the target's element type.
//  3. If assignable, sets the value using reflection.
//
// Parameters:
//   - target (any): pointer to interface or concrete type
//
// Returns:
//   - ok (bool): true if assignment was successful
func (e *root) As(target any) (ok bool) {
	if target == nil {
		return
	}

	val := reflect.ValueOf(target)

	if val.Kind() != reflect.Pointer || val.IsNil() {
		return
	}

	targetType := val.Type().Elem()
	currentType := reflect.TypeOf(e)

	if currentType.AssignableTo(targetType) {
		val.Elem().Set(reflect.ValueOf(e))

		ok = true

		return
	}

	return
}

// Unwrap returns the underlying error if this error wraps another.
// Implements the standard library's error unwrapping interface.
//
// Returns:
//   - cause (error): the wrapped error (may be nil) or nil if receiver or cause is nil
func (e *root) Unwrap() (cause error) {
	if e == nil || e.cause == nil {
		return
	}

	cause = e.cause

	return
}

// SetType associates a type with the error for classification purposes.
// This enables error handling based on error categories/types.
// The operation is thread-safe, protected by the mutex.
//
// Parameters:
//   - errType (Type): the Type to assign to this error
//
// Returns:
//   - err (Error): the modified error (supports method chaining) or nil if receiver is ni
func (e *root) SetType(errType Type) (err Error) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.errType = errType

	err = e

	return
}

// SetField adds a key-value pair to the error's structured context.
// Fields provide additional machine-readable information about the error.
//
// If no fields map exists, it initializes one before adding the key-value pair.
// The operation is thread-safe, protected by the mutex.
//
// Parameters:
//   - key (string): field name (should be descriptive and consistent)
//   - value (any): field value (any serializable type)
//
// Returns:
//   - err (Error): the modified error (supports method chaining) or nil if receiver is nil
func (e *root) SetField(key string, value any) (err Error) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.fields == nil {
		e.fields = map[string]any{}
	}

	e.fields[key] = value

	err = e

	return
}

// wrapped represents an error that wraps another error with additional context.
// Unlike root, it only captures a single stack frame (where it was created).
//
// Fields:
//   - mu (sync.RWMutex): mutex for thread-safe access to modifiable fields
//   - errType (Type): error type for classification (Type)
//   - message (string): human-readable error message
//   - fields (map[string]any): additional structured context (key-value pairs)
//   - cause (error): underlying error being wrapped
//   - frame (*frame): stack frame where the wrap occurred
type wrapped struct {
	mu      sync.RWMutex
	errType Type
	message string
	fields  map[string]any
	cause   error
	frame   *frame
}

// Type returns the error's classification type if one was set.
// It safely reads the errType field.
//
// Returns:
//   - errType (Type): the error's type, or empty string if untyped or receiver is nil
func (e *wrapped) Type() (errType Type) {
	if e == nil {
		return
	}

	errType = e.errType

	return
}

// Error implements the error interface, returning the error message.
// If the error wraps another error, it combines both messages.
//
// Returns:
//   - msg (string): the error message (or "<nil>" if receiver is nil)
func (e *wrapped) Error() (msg string) {
	msg = "<nil>"

	if e == nil {
		return
	}

	msg = e.message

	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}

	return
}

// Fields returns all structured fields attached to the error.
// The returned map should not be modified directly.
//
// Returns:
//   - fields (map[string]any): all attached fields (may be nil) or nil if receiver is nil
func (e *wrapped) Fields() (fields map[string]any) {
	if e == nil {
		return
	}

	fields = e.fields

	return
}

// StackFrames returns the raw program counters from the call stack.
// These can be used to reconstruct the full stack trace.
//
// For wrapped errors, this returns a single-frame stack containing the wrap point.
//
// Returns:
//   - frames ([]uintptr): slice of program counters representing the call stack or nil if receiver is nil
func (e *wrapped) StackFrames() (frames []uintptr) {
	if e == nil {
		return
	}

	frames = []uintptr{e.frame.pc()}

	return
}

// Is implements error equality checking. Two errors are considered equal if:
//   - Both are nil, or
//   - They are of the same type (*wrapped), and:
//   - Their types match (or target type is empty), and
//   - Their messages match
//   - Their messages match exactly (fallback)
//
// Parameters:
//   - target (error): the error to compare against
//
// Returns:
//   - matches (bool): true if errors are considered equal
func (e *wrapped) Is(target error) (matches bool) {
	if target == nil {
		matches = e == nil

		return
	}

	if err, ok := target.(*wrapped); ok {
		matches = (err.errType == "" || e.errType == err.errType) && e.message == err.message

		return
	}

	return
}

// As attempts to assign the error to the target interface.
// The target must be a non-nil pointer to either:
//   - An interface type that the error implements, or
//   - A concrete type that matches the error's type
//
// The assignment process:
//  1. Validates that target is a non-nil pointer.
//  2. Checks if the error's type is assignable to the target's element type.
//  3. If assignable, sets the value using reflection.
//
// Parameters:
//   - target (any): pointer to interface or concrete type
//
// Returns:
//   - ok (bool): true if assignment was successful
func (e *wrapped) As(target any) (ok bool) {
	if target == nil {
		return
	}

	val := reflect.ValueOf(target)

	if val.Kind() != reflect.Pointer || val.IsNil() {
		return
	}

	targetType := val.Type().Elem()
	currentType := reflect.TypeOf(e)

	if currentType.AssignableTo(targetType) {
		val.Elem().Set(reflect.ValueOf(e))

		ok = true

		return
	}

	return
}

// Unwrap returns the underlying error if this error wraps another.
// Implements the standard library's error unwrapping interface.
//
// Returns:
//   - cause (error): the wrapped error (may be nil) or nil if receiver or cause is nil
func (e *wrapped) Unwrap() (cause error) {
	if e == nil || e.cause == nil {
		return
	}

	cause = e.cause

	return
}

// SetType associates a type with the error for classification purposes.
// This enables error handling based on error categories/types.
// The operation is thread-safe, protected by the mutex.
//
// Parameters:
//   - errType (Type): the Type to assign to this error
//
// Returns:
//   - err (Error): the modified error (supports method chaining) or nil if receiver is nil
func (e *wrapped) SetType(errType Type) (err Error) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.errType = errType

	err = e

	return
}

// SetField adds a key-value pair to the error's structured context.
// Fields provide additional machine-readable information about the error.
//
// If no fields map exists, it initializes one before adding the key-value pair.
// The operation is thread-safe, protected by the mutex.
//
// Parameters:
//   - key (string): field name (should be descriptive and consistent)
//   - value (any): field value (any serializable type)
//
// Returns:
//   - err (Error): the modified error (supports method chaining) or nil if receiver is nil
func (e *wrapped) SetField(key string, value any) (err Error) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.fields == nil {
		e.fields = map[string]any{}
	}

	e.fields[key] = value

	err = e

	return
}

// joined represents a collection of multiple errors joined into one.
// It captures a stack trace at the join point and implements multi-error unwrapping.
//
// Fields:
//   - isGlobal (bool): indicates if the join occurred during package initialization
//   - errors ([]error): the list of joined errors
//   - trace (*stack): captured call stack at the join point
type joined struct {
	isGlobal bool
	errors   []error
	trace    *stack
}

// Error implements the error interface by joining all error messages with newlines.
// If there are no errors, it returns an empty string.
//
// Returns:
//   - msg (string): concatenated error messages separated by newlines or empty if receiver is nil or no errors
func (e *joined) Error() (msg string) {
	if e == nil || len(e.errors) == 0 {
		return
	}

	var messages []string

	for _, err := range e.errors {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}

	msg = strings.Join(messages, "\n")

	return
}

// StackFrames returns the raw program counters from the call stack at the join point.
//
// Returns:
//   - frames ([]uintptr): slice of program counters representing the call stack or nil if receiver or trace is nil
func (e *joined) StackFrames() (frames []uintptr) {
	if e == nil || e.trace == nil {
		return
	}

	frames = *e.trace

	return
}

// Is checks if any of the joined errors match the target using the Is function.
//
// Parameters:
//   - target (error): the error to compare against
//
// Returns:
//   - matches (bool): true if any joined error matches the target
func (e *joined) Is(target error) (matches bool) {
	if target == nil {
		matches = e == nil

		return
	}

	for _, err := range e.errors {
		if Is(err, target) {
			matches = true

			return
		}
	}

	return
}

// As attempts to assign any of the joined errors to the target using the As function.
//
// Parameters:
//   - target (any): pointer to interface or concrete type
//
// Returns:
//   - ok (bool): true if any joined error was successfully assigned
func (e *joined) As(target any) (ok bool) {
	for _, err := range e.errors {
		if As(err, target) {
			ok = true

			return
		}
	}

	return
}

// Unwrap returns the list of joined errors for multi-error unwrapping.
//
// Returns:
//   - errs ([]error): the slice of joined errors or nil if receiver is nil
func (e *joined) Unwrap() (errs []error) {
	if e == nil {
		return
	}

	errs = e.errors

	return
}

// Error is the interface that groups all error capabilities in this package.
// It extends the standard error interface with additional functionality:
//   - Type classification
//   - Structured fields
//   - Stack traces
//   - Standard error wrapping
type Error interface {
	error
	Type() (errType Type)
	Fields() (fields map[string]interface{})
	StackFrames() (PCs []uintptr)
	Is(target error) (result bool)
	As(target interface{}) (result bool)
	SetType(errType Type) (err Error)
	SetField(key string, value interface{}) (err Error)
}

// Type represents a classification type for errors.
// Types allow errors to be categorized and handled based on their kind.
type Type string

// OptionFunc represents a function that can configure an Error.
// Used with New and Wrap to set error properties at creation time.
type OptionFunc func(err Error)

var (
	_ Error = (*root)(nil)
	_ Error = (*wrapped)(nil)
	_ error = (*joined)(nil)
)

// New creates a new root error with stack trace information.
// The skip parameter (3) ensures the trace starts at the caller's location.
//
// The creation process:
//  1. Captures the call stack skipping internal frames.
//  2. Checks if the error occurred during global initialization.
//  3. Applies all provided option functions to configure the error.
//
// Parameters:
//   - msg (string): the primary error message
//   - ofs (...OptionFunc): variadic list of OptionFunc functions to configure the error
//
// Returns:
//   - err (error): the newly created error (implements Error interface)
func New(msg string, ofs ...OptionFunc) (err error) {
	trace := callers(3) // callers(3) skips this method (New), callers, and runtime.Callers

	e := &root{
		isGlobal: trace.isGlobal(),
		message:  msg,
		trace:    trace,
	}

	for _, f := range ofs {
		f(e)
	}

	err = e

	return
}

// Wrap creates a new error that wraps an existing error with additional context.
// The new error will have its own stack frame while preserving the original's trace.
//
// It delegates to the internal wrap function and applies options afterward.
//
// Parameters:
//   - cause (error): the error to wrap
//   - msg (string): additional context message
//   - ofs (...OptionFunc): configuration options (same as New)
//
// Returns:
//   - err (error): the new wrapping error
func Wrap(cause error, msg string, ofs ...OptionFunc) (err error) {
	w := wrap(cause, msg)

	for _, f := range ofs {
		f(w)
	}

	err = w

	return
}

// wrap is the internal implementation of error wrapping logic that handles three distinct cases:
//
// 1. Wrapping a root (preserves full stack trace while adding new context)
// 2. Wrapping a wrapped (finds root error to preserve complete trace)
// 3. Wrapping a non-package error (creates new root error with full stack)
//
// The wrapping process:
//  1. Captures the current stack trace and frame.
//  2. Handles root by inserting the new trace or recreating if global.
//  3. For wrapped, inserts into the underlying root's trace.
//  4. For other errors, creates a new root.
//
// Parameters:
//   - cause (error): The error being wrapped. Must be non-nil for the function to have effect.
//     If nil is passed, the function returns nil.
//   - msg (string): Additional contextual information describing the wrapping site.
//     This message will become part of the error chain and appear in the Error() output.
//     Should be descriptive enough to identify where/why the wrap occurred.
//
// Returns:
//   - err (Error): The newly created wrapping error that implements the Error interface.
func wrap(cause error, msg string) (err Error) {
	if cause == nil {
		return
	}

	trace := callers(4) // callers(4) skips runtime.Callers, callers, this method (wrap), and Wrap
	frame := caller(3)  // caller(3) skips caller, this method (wrap), and Wrap

	switch e := cause.(type) {
	case *root:
		if e.isGlobal {
			cause = &root{
				isGlobal: e.isGlobal,
				errType:  e.errType,
				message:  e.message,
				fields:   e.fields,
				cause:    e.cause,
				trace:    trace,
			}
		} else {
			e.trace.insertPC(*trace)
		}
	case *wrapped:
		if r, ok := Cause(cause).(*root); ok {
			r.trace.insertPC(*trace)
		}
	default:
		err = &root{
			message: msg,
			cause:   e,
			trace:   trace,
		}

		return
	}

	err = &wrapped{
		message: msg,
		cause:   cause,
		frame:   frame,
	}

	return
}

// WithType creates an OptionFunc that sets an error's type.
//
// Parameters:
//   - errType (Type): the Type to set
//
// Returns:
//   - f (OptionFunc): configuration function for New/Wrap
func WithType(errType Type) (f OptionFunc) {
	return func(err Error) {
		err.SetType(errType)
	}
}

// WithField creates an OptionFunc that adds a field to an error.
//
// Parameters:
//   - key (string): field key
//   - value (any): field value
//
// Returns:
//   - f (OptionFunc): configuration function for New/Wrap
func WithField(key string, value any) (f OptionFunc) {
	return func(err Error) {
		err.SetField(key, value)
	}
}

// Unwrap returns the result of calling Unwrap() on err if available.
// Matches the behavior of errors.Unwrap in the standard library.
//
// Parameters:
//   - err (error): the error to unwrap.
//
// Returns:
//   - cause (error): The next error in the chain, or nil if none.
func Unwrap(err error) (cause error) {
	u, k := err.(interface{ Unwrap() error })
	if !k {
		return
	}

	cause = u.Unwrap()

	return
}

// Is reports whether err or any error in its chain matches target.
// Implements an enhanced version of errors.Is from the standard library.
//
// It delegates to the internal is function for recursive checking.
//
// Parameters:
//   - err (error): the error to inspect.
//   - target (error): the error to compare against.
//
// Returns:
//   - matches (bool): true if any error in err's chain matches target.
func Is(err, target error) (matches bool) {
	if err == nil || target == nil {
		matches = err == target

		return
	}

	isComparable := reflect.TypeOf(target).Comparable()

	matches = is(err, target, isComparable)

	return
}

// is is the internal recursive helper for Is.
// It checks direct equality, custom Is methods, and unwraps errors (including multi-errors).
//
// Parameters:
//   - err (error): the current error to check
//   - target (error): the target to match
//   - isComparable (bool): whether the target type is comparable
//
// Returns:
//   - matches (bool): true if a match is found
func is(err, target error, isComparable bool) (matches bool) {
	for {
		if isComparable && err == target {
			matches = true

			return
		}

		if x, k := err.(interface{ Is(error) bool }); k && x.Is(target) {
			matches = true

			return
		}

		switch x := err.(type) {
		case interface{ Unwrap() error }:
			if err = x.Unwrap(); err == nil {
				return
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if is(err, target, isComparable) {
					matches = true

					return
				}
			}

			return
		default:
			return
		}
	}
}

// As searches err's chain for an error assignable to target and sets target if found.
// Implements an enhanced version of errors.As from the standard library.
//
// It validates the target and delegates to the internal as function.
//
// Parameters:
//   - err (error): the error to inspect.
//   - target (any): pointer to the destination interface or concrete type.
//
// Returns:
//   - ok (bool): true if a matching error was found and target was set.
func As(err error, target any) (ok bool) {
	if target == nil || err == nil {
		return
	}

	val := reflect.ValueOf(target)
	typ := val.Type()

	if typ.Kind() != reflect.Pointer || val.IsNil() {
		return
	}

	targetType := typ.Elem()

	if targetType.Kind() != reflect.Interface && !targetType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return
	}

	ok = as(err, target, val, targetType)

	return
}

// as is the internal recursive helper for As.
// It checks assignability, custom As methods, and unwraps errors (including multi-errors).
//
// Parameters:
//   - err (error): the current error to check
//   - target (any): the target pointer
//   - targetVal (reflect.Value): reflection value of target
//   - targetType (reflect.Type): type of the target's element
//
// Returns:
//   - ok (bool): true if a match is found and target is set
func as(err error, target any, targetVal reflect.Value, targetType reflect.Type) (ok bool) {
	for {
		if reflect.TypeOf(err).AssignableTo(targetType) {
			targetVal.Elem().Set(reflect.ValueOf(err))

			ok = true

			return
		}

		if x, k := err.(interface{ As(interface{}) bool }); k && x.As(target) {
			ok = true

			return
		}

		switch x := err.(type) {
		case interface{ Unwrap() error }:
			if err = x.Unwrap(); err == nil {
				return
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err == nil {
					continue
				}

				if as(err, target, targetVal, targetType) {
					ok = true

					return
				}
			}

			return
		default:
			return
		}
	}
}

// Cause returns the underlying root cause of the error by recursively unwrapping.
// Unlike Unwrap, it follows the entire chain to the original error.
//
// Parameters:
//   - err (error): the error to inspect.
//
// Returns:
//   - cause (error): The deepest non-wrapped error in the chain.
func Cause(err error) (cause error) {
	for {
		uerr := Unwrap(err)
		if uerr == nil {
			cause = err

			return
		}

		err = uerr
	}
}

// Join combines multiple errors into a single joined error.
// It filters out nil errors and captures a stack trace at the join point.
//
// If no non-nil errors are provided, returns nil.
// If only one non-nil error, returns that error directly.
//
// Parameters:
//   - errs (...error): variadic list of errors to join
//
// Returns:
//   - err (error): the joined error or single error if only one
func Join(errs ...error) (err error) {
	var nonNilErrs []error

	for _, e := range errs {
		if e != nil {
			nonNilErrs = append(nonNilErrs, e)
		}
	}

	if len(nonNilErrs) == 0 {
		return
	}

	if len(nonNilErrs) == 1 {
		err = nonNilErrs[0]

		return
	}

	trace := callers(3)

	err = &joined{
		isGlobal: trace.isGlobal(),
		errors:   nonNilErrs,
		trace:    trace,
	}

	return
}
