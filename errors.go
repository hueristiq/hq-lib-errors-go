package errors

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// root represents a fundamental error with complete stack trace information.
// It serves as the base error type in the package and implements the Error interface.
//
// A root is immutable once returned by New except through SetType and SetField,
// which are guarded by mu. In particular, wrapping a root never mutates it.
//
// Fields:
//   - mu (sync.RWMutex): mutex for thread-safe access to modifiable fields
//   - errType (Type): error type for classification (Type)
//   - message (string): human-readable error message
//   - fields (map[string]any): additional structured context (key-value pairs)
//   - cause (error): the underlying error being wrapped (if any)
//   - trace (*stack): captured call stack information
type root struct {
	mu      sync.RWMutex
	errType Type
	message string
	fields  map[string]any
	cause   error
	trace   *stack
}

// Type returns the error's classification type, or the empty Type if none was
// set. The read is taken under the read lock, so it is safe to call concurrently
// with [root.SetType].
//
// Returns:
//   - errType (Type): the error's type, or empty string if untyped or the receiver is nil.
func (e *root) Type() (errType Type) {
	if e == nil {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	errType = e.errType

	return
}

// Error implements the error interface, returning the error message.
// If the error wraps another error, it combines both messages, joining them
// with ": " only when the message itself is non-empty.
//
// Returns:
//   - msg (string): the error message (or "<nil>" if receiver is nil)
func (e *root) Error() (msg string) {
	if e == nil {
		return "<nil>"
	}

	msg = e.message

	if e.cause != nil {
		if msg != "" {
			msg += ": "
		}

		msg += e.cause.Error()
	}

	return msg
}

// Fields returns a snapshot copy of all structured fields attached to the error.
// The copy is taken under the read lock, so it is safe to read concurrently with
// SetField; mutating the returned map does not affect the error.
//
// Returns:
//   - fields (map[string]any): a copy of the attached fields (nil if none or if receiver is nil)
func (e *root) Fields() (fields map[string]any) {
	if e == nil {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	fields = maps.Clone(e.fields)

	return
}

// StackFrames returns the raw PCs (program counters) from the call stack.
// These can be used to reconstruct the full stack trace.
//
// The returned slice is a copy; mutating it does not affect the error.
//
// Returns:
//   - frames ([]uintptr): slice of program counters representing the call stack,
//     or nil if receiver or trace is nil
func (e *root) StackFrames() (frames []uintptr) {
	if e == nil || e.trace == nil {
		return nil
	}

	return slices.Clone(*e.trace)
}

// Stack returns the captured call stack as resolved frames, most recent call
// first. The raw PCs are symbolized into function, file, and line at call
// time; use [root.StackFrames] when raw PCs suffice.
//
// Returns:
//   - frames (Stack): the resolved stack frames, or nil if the receiver or its
//     trace is nil
func (e *root) Stack() (frames Stack) {
	if e == nil || e.trace == nil {
		return
	}

	frames = e.trace.resolveToStackFrames()

	return
}

// Is reports whether target is another *root with the same message and a
// matching type. The type matches when both types are equal or target's type
// is empty. Errors of any other concrete type never match here; cross-type
// matching is handled by the package-level Is via Unwrap traversal.
//
// The target's type is read through [root.Type] so the read stays synchronized
// with concurrent [root.SetType] calls on the target.
//
// Parameters:
//   - target (error): the error to compare against
//
// Returns:
//   - matches (bool): true if errors are considered equal
func (e *root) Is(target error) (matches bool) {
	if e == nil || target == nil {
		return e == nil && target == nil
	}

	t, ok := target.(*root)
	if !ok || t == nil {
		return false
	}

	e.mu.RLock()
	errType := e.errType
	e.mu.RUnlock()

	targetType := t.Type()

	return (targetType == "" || errType == targetType) && e.message == t.message
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
		return false
	}

	val := reflect.ValueOf(target)

	if val.Kind() != reflect.Pointer || val.IsNil() {
		return false
	}

	targetType := val.Type().Elem()
	currentType := reflect.TypeOf(e)

	if !currentType.AssignableTo(targetType) {
		return false
	}

	val.Elem().Set(reflect.ValueOf(e))

	return true
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
//   - err (MutableError): the modified error (supports method chaining) or nil if receiver is nil
func (e *root) SetType(errType Type) (err MutableError) {
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
//   - err (MutableError): the modified error (supports method chaining) or nil if receiver is nil
func (e *root) SetField(key string, value any) (err MutableError) {
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

// Type returns the error's classification type, or the empty Type if none was
// set. The read is taken under the read lock, so it is safe to call concurrently
// with [wrapped.SetType].
//
// Returns:
//   - errType (Type): the error's type, or empty string if untyped or the receiver is nil.
func (e *wrapped) Type() (errType Type) {
	if e == nil {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	errType = e.errType

	return
}

// Error implements the error interface, returning the error message.
// If the error wraps another error, it combines both messages, joining them
// with ": " only when the message itself is non-empty.
//
// Returns:
//   - msg (string): the error message (or "<nil>" if receiver is nil)
func (e *wrapped) Error() (msg string) {
	if e == nil {
		return "<nil>"
	}

	msg = e.message

	if e.cause != nil {
		if msg != "" {
			msg += ": "
		}

		msg += e.cause.Error()
	}

	return msg
}

// Fields returns a snapshot copy of all structured fields attached to the error.
// The copy is taken under the read lock, so it is safe to read concurrently with
// SetField; mutating the returned map does not affect the error.
//
// Returns:
//   - fields (map[string]any): a copy of the attached fields (nil if none or if receiver is nil)
func (e *wrapped) Fields() (fields map[string]any) {
	if e == nil {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	fields = maps.Clone(e.fields)

	return
}

// StackFrames returns the raw program counters from the call stack.
// These can be used to reconstruct the full stack trace.
//
// For wrapped errors, this returns a single-frame stack containing the wrap point.
//
// Returns:
//   - frames ([]uintptr): slice of program counters representing the call stack,
//     or nil if the receiver or its frame is nil
func (e *wrapped) StackFrames() (frames []uintptr) {
	if e == nil || e.frame == nil {
		return
	}

	frames = []uintptr{e.frame.pc()}

	return
}

// Stack returns the wrap-site location as a single resolved frame. The raw PC
// is symbolized into function, file, and line at call time; use
// [wrapped.StackFrames] when the raw PC suffices.
//
// Returns:
//   - frames (Stack): the resolved wrap-site frame, or nil if the receiver or
//     its frame is nil
func (e *wrapped) Stack() (frames Stack) {
	if e == nil || e.frame == nil {
		return
	}

	frames = Stack{e.frame.resolveToStackFrame()}

	return
}

// Is reports whether target is another *wrapped with the same message and a
// matching type. The type matches when both types are equal or target's type
// is empty. Errors of any other concrete type never match here; cross-type
// matching is handled by the package-level Is via Unwrap traversal.
//
// The target's type is read through [wrapped.Type] so the read stays synchronized
// with concurrent [wrapped.SetType] calls on the target.
//
// Parameters:
//   - target (error): the error to compare against
//
// Returns:
//   - matches (bool): true if errors are considered equal
func (e *wrapped) Is(target error) (matches bool) {
	if e == nil || target == nil {
		return e == nil && target == nil
	}

	t, ok := target.(*wrapped)
	if !ok || t == nil {
		return false
	}

	e.mu.RLock()
	errType := e.errType
	e.mu.RUnlock()

	targetType := t.Type()

	return (targetType == "" || errType == targetType) && e.message == t.message
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
		return false
	}

	val := reflect.ValueOf(target)

	if val.Kind() != reflect.Pointer || val.IsNil() {
		return false
	}

	targetType := val.Type().Elem()
	currentType := reflect.TypeOf(e)

	if !currentType.AssignableTo(targetType) {
		return false
	}

	val.Elem().Set(reflect.ValueOf(e))

	return true
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
//   - err (MutableError): the modified error (supports method chaining) or nil if receiver is nil
func (e *wrapped) SetType(errType Type) (err MutableError) {
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
//   - err (MutableError): the modified error (supports method chaining) or nil if receiver is nil
func (e *wrapped) SetField(key string, value any) (err MutableError) {
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
//   - errs ([]error): the list of joined errors
//   - trace (*stack): captured call stack at the join point
type joined struct {
	errs  []error
	trace *stack
}

// Error implements the error interface by joining all error messages with newlines.
// If there are no errors, it returns an empty string.
//
// Returns:
//   - msg (string): concatenated error messages separated by newlines or empty if receiver is nil or no errors
func (e *joined) Error() (msg string) {
	if e == nil || len(e.errs) == 0 {
		return ""
	}

	messages := make([]string, 0, len(e.errs))

	for _, err := range e.errs {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}

	return strings.Join(messages, "\n")
}

// StackFrames returns the raw program counters from the call stack at the join point.
//
// The returned slice is a copy; mutating it does not affect the error.
//
// Returns:
//   - frames ([]uintptr): slice of program counters representing the call stack or nil if receiver or trace is nil
func (e *joined) StackFrames() (frames []uintptr) {
	if e == nil || e.trace == nil {
		return nil
	}

	return slices.Clone(*e.trace)
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
		return e == nil
	}

	for _, err := range e.errs {
		if Is(err, target) {
			return true
		}
	}

	return false
}

// As attempts to assign any of the joined errors to the target using the As function.
//
// Parameters:
//   - target (any): pointer to interface or concrete type
//
// Returns:
//   - ok (bool): true if any joined error was successfully assigned
func (e *joined) As(target any) (ok bool) {
	for _, err := range e.errs {
		if As(err, target) {
			return true
		}
	}

	return false
}

// Unwrap returns the list of joined errors for multi-error unwrapping.
//
// Returns:
//   - errs ([]error): the slice of joined errors or nil if receiver is nil
func (e *joined) Unwrap() (errs []error) {
	if e == nil {
		return nil
	}

	return e.errs
}

// Error is the read-only inspection interface implemented by every error this
// package creates (those returned by [New] and [Wrap]). It extends the standard
// error interface with type classification, structured fields, and stack traces.
//
// Use [As] (or a type assertion) to obtain an Error from an opaque error value.
// Implementations are safe for concurrent use. For post-construction mutation,
// see [MutableError].
type Error interface {
	error

	// Type returns the error's classification type, or the empty Type if unset.
	Type() (errType Type)

	// Fields returns a snapshot copy of the error's structured fields.
	Fields() (fields map[string]interface{})

	// Stack returns the captured call stack as resolved frames, most recent
	// call first. Resolution from raw PCs happens at call time.
	Stack() (frames Stack)

	// StackFrames returns the raw program counters of the captured call stack.
	// The returned slice is a copy; mutating it does not affect the error.
	StackFrames() (PCs []uintptr)
}

// MutableError extends [Error] with post-construction, thread-safe mutation of
// the classification type and structured fields. It is the interface option
// functions receive (see [OptionFunc]); obtain it from an opaque error with
// [As] or a type assertion.
//
// Prefer configuring errors at creation time with [WithType] and [WithField]:
// mutating a shared error changes what every holder of it observes.
type MutableError interface {
	Error

	// SetType sets the classification type and returns the error for chaining.
	SetType(errType Type) (err MutableError)

	// SetField adds a key-value field and returns the error for chaining.
	SetField(key string, value interface{}) (err MutableError)
}

// Type is a classification label for an error, letting callers categorize and
// branch on errors by kind (for example "PaymentError" or "NotFound") rather
// than by message. Set it with [WithType] or [Error.SetType] and read it with
// [Error.Type]. The empty Type means unclassified.
type Type string

// OptionFunc configures a [MutableError] at construction time. Pass option
// functions such as [WithType] and [WithField] to [New] and [Wrap]. A nil
// OptionFunc is ignored.
type OptionFunc func(err MutableError)

// Compile-time assertions that the concrete error types satisfy their intended
// interfaces. root and wrapped implement the full MutableError interface;
// joined implements only the standard error interface (it carries no type or
// fields).
var (
	_ Error        = (*root)(nil)
	_ MutableError = (*root)(nil)
	_ Error        = (*wrapped)(nil)
	_ MutableError = (*wrapped)(nil)
	_ error        = (*joined)(nil)
)

// New creates a new root error, capturing a stack trace starting at the caller.
//
// The captured program counters are stored unresolved; symbolization into
// file, line, and function happens lazily during formatting, so New stays cheap
// even on hot paths (see BenchmarkNew). At most 64 stack frames are captured;
// deeper stacks are truncated. Pass [WithType] and [WithField] options to
// classify the error and attach structured context at creation time.
//
// The returned error always implements the [Error] and [MutableError]
// interfaces; type-assert to them (or use [As]) to reach [Error.Type],
// [Error.Fields], [Error.Stack], and [Error.StackFrames].
//
// Parameters:
//   - msg (string): the primary, human-readable error message.
//   - ofs (...OptionFunc): options applied to the new error, e.g. [WithType] and [WithField].
//
// Returns:
//   - err (error): the newly created error; never nil.
func New(msg string, ofs ...OptionFunc) (err error) {
	e := &root{
		message: msg,
		trace:   callers(3), // callers(3) skips this method (New), callers, and runtime.Callers
	}

	for _, f := range ofs {
		if f != nil {
			f(e)
		}
	}

	return e
}

// Wrap creates a new error that wraps an existing error with additional context.
// The new error captures its own wrap-site frame; the wrapped error is referenced
// as-is and is never mutated, so wrapping a shared error is safe for concurrent use.
//
// It delegates to the internal wrap function and applies options afterward.
// Wrapping a nil cause returns nil and any options are ignored.
//
// Parameters:
//   - cause (error): the error to wrap
//   - msg (string): additional context message
//   - ofs (...OptionFunc): configuration options (same as New)
//
// Returns:
//   - err (error): the new wrapping error, or nil if cause is nil
func Wrap(cause error, msg string, ofs ...OptionFunc) (err error) {
	w := wrap(cause, msg)
	if w == nil {
		return nil
	}

	for _, f := range ofs {
		if f != nil {
			f(w)
		}
	}

	return w
}

// wrap is the internal implementation of error wrapping. It never mutates the
// error it wraps; the cause is referenced as-is and the new error owns only the
// context captured at the wrap site. This keeps wrapping safe for concurrent use
// and free of surprising side effects on the caller's error value.
//
// Two cases are handled:
//
//  1. Wrapping a package error (*root or *wrapped): a *wrapped is returned that
//     captures the single wrap-site frame and points at cause. The underlying
//     root retains its original creation trace.
//  2. Wrapping any other (external) error: a *root is returned that captures a
//     full stack at the wrap site and points at cause.
//
// Parameters:
//   - cause (error): The error being wrapped. If nil, the function returns nil.
//   - msg (string): Additional contextual information describing the wrapping site.
//     This message becomes part of the error chain and appears in Error() output.
//
// Returns:
//   - err (MutableError): The newly created wrapping error.
func wrap(cause error, msg string) (err MutableError) {
	if cause == nil {
		return nil
	}

	switch cause.(type) {
	case *root, *wrapped:
		err = &wrapped{
			message: msg,
			cause:   cause,
			frame:   caller(3), // caller(3) skips caller, this method (wrap), and Wrap
		}
	default:
		err = &root{
			message: msg,
			cause:   cause,
			trace:   callers(4), // callers(4) skips runtime.Callers, callers, this method (wrap), and Wrap
		}
	}

	return err
}

// WithType creates an OptionFunc that sets an error's type.
//
// Parameters:
//   - errType (Type): the Type to set
//
// Returns:
//   - f (OptionFunc): configuration function for New/Wrap
func WithType(errType Type) (f OptionFunc) {
	return func(err MutableError) {
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
	return func(err MutableError) {
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
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}

	return u.Unwrap()
}

// Is reports whether err or any error in its chain matches target.
// Implements an enhanced version of errors.Is from the standard library.
//
// It delegates to the internal is function for recursive checking.
//
// Note: errors created by this package compare by value, not identity — a root
// or wrapped error matches any target of the same concrete type with an equal
// message and a compatible [Type] (an empty target Type matches any Type).
// This differs from the standard library's errors.New, whose values only match
// themselves: two distinct [New] calls with the same arguments produce errors
// that match each other here. Create sentinel errors once and reuse them when
// identity matters.
//
// Parameters:
//   - err (error): the error to inspect.
//   - target (error): the error to compare against.
//
// Returns:
//   - matches (bool): true if any error in err's chain matches target.
func Is(err, target error) (matches bool) {
	if err == nil || target == nil {
		return err == target
	}

	return is(err, target, reflect.TypeOf(target).Comparable())
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
			return true
		}

		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}

		switch x := err.(type) {
		case interface{ Unwrap() error }:
			if err = x.Unwrap(); err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err != nil && is(err, target, isComparable) {
					return true
				}
			}

			return false
		default:
			return false
		}
	}
}

// As searches err's chain for an error assignable to target and sets target if found.
// Matches the behavior of errors.As in the standard library, including
// panicking when target is invalid: target must be a non-nil pointer to a type
// that implements error, or to any interface type.
//
// Parameters:
//   - err (error): the error to inspect.
//   - target (any): pointer to the destination interface or concrete type.
//
// Returns:
//   - ok (bool): true if a matching error was found and target was set.
func As(err error, target any) (ok bool) {
	if err == nil {
		return false
	}

	if target == nil {
		panic("errors: target cannot be nil")
	}

	val := reflect.ValueOf(target)
	typ := val.Type()

	if typ.Kind() != reflect.Pointer || val.IsNil() {
		panic("errors: target must be a non-nil pointer")
	}

	targetType := typ.Elem()

	if targetType.Kind() != reflect.Interface && !targetType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic("errors: *target must be interface or implement error")
	}

	return as(err, target, val, targetType)
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

			return true
		}

		if x, ok := err.(interface{ As(interface{}) bool }); ok && x.As(target) {
			return true
		}

		switch x := err.(type) {
		case interface{ Unwrap() error }:
			if err = x.Unwrap(); err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err == nil {
					continue
				}

				if as(err, target, targetVal, targetType) {
					return true
				}
			}

			return false
		default:
			return false
		}
	}
}

// Cause returns the underlying root cause of the error by recursively unwrapping.
// Unlike Unwrap, it follows the entire chain to the original error.
//
// Cause follows single-error unwrapping only: it does not descend into the
// branches of a joined error (one created by [Join], or any error with an
// Unwrap() []error method) and instead returns the joined error itself.
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
			return err
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
	nonNilErrs := make([]error, 0, len(errs))

	for _, e := range errs {
		if e != nil {
			nonNilErrs = append(nonNilErrs, e)
		}
	}

	if len(nonNilErrs) == 0 {
		return nil
	}

	if len(nonNilErrs) == 1 {
		return nonNilErrs[0]
	}

	return &joined{
		errs:  nonNilErrs,
		trace: callers(3),
	}
}
