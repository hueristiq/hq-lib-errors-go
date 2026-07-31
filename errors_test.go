package errors

import (
	stderrors "errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootError(t *testing.T) {
	t.Parallel()

	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()

		err := New("test error")

		require.Error(t, err)
		assert.Equal(t, "test error", err.Error())
		assert.NotEmpty(t, err.(*root).trace)
	})

	t.Run("with type", func(t *testing.T) {
		t.Parallel()

		err := New("typed error", WithType("TEST_TYPE"))

		require.Error(t, err)
		assert.Equal(t, Type("TEST_TYPE"), err.(*root).errType)
	})

	t.Run("with field", func(t *testing.T) {
		t.Parallel()

		err := New("field error", WithField("key", "value"))

		require.Error(t, err)
		assert.Equal(t, map[string]interface{}{"key": "value"}, err.(*root).fields)
	})

	t.Run("error message", func(t *testing.T) {
		t.Parallel()

		err := New("base error")

		wrappedErr := Wrap(err, "wrapper")

		assert.Equal(t, "wrapper: base error", wrappedErr.Error())
	})

	t.Run("is comparison", func(t *testing.T) {
		t.Parallel()

		err1 := New("error", WithType("TYPE"))
		err2 := New("error", WithType("TYPE"))
		err3 := New("different error")

		assert.True(t, err1.(*root).Is(err2))
		assert.False(t, err1.(*root).Is(err3))
	})

	t.Run("as type assertion", func(t *testing.T) {
		t.Parallel()

		err := New("error")

		var target *root

		assert.True(t, err.(*root).As(&target))
		assert.Equal(t, err, target)
	})

	t.Run("unwrap", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")
		wrappedErr := Wrap(baseErr, "wrapper")

		assert.Equal(t, baseErr, wrappedErr.(*wrapped).Unwrap())
	})

	t.Run("stack frames", func(t *testing.T) {
		t.Parallel()

		err := New("error")

		pcs := err.(*root).StackFrames()

		assert.NotEmpty(t, pcs)
	})

	t.Run("stack", func(t *testing.T) {
		t.Parallel()

		err := New("error")

		frames := err.(*root).Stack()

		require.NotEmpty(t, frames)
		assert.Contains(t, frames[0].Name, "TestRootError")
	})

	t.Run("nil receiver", func(t *testing.T) {
		t.Parallel()

		var nilErr *root

		assert.Equal(t, "<nil>", nilErr.Error())
		assert.Nil(t, nilErr.Fields())
		assert.Empty(t, nilErr.StackFrames())
		assert.False(t, nilErr.Is(stderrors.New("test")))
	})
}

func TestWrapError(t *testing.T) {
	t.Parallel()

	t.Run("basic wrapping", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")
		wrappedErr := Wrap(baseErr, "wrapper")

		require.Error(t, wrappedErr)
		assert.Equal(t, "wrapper: base", wrappedErr.Error())
	})

	t.Run("wrapping non-package error", func(t *testing.T) {
		t.Parallel()

		stdErr := stderrors.New("standard error")
		wrappedErr := Wrap(stdErr, "wrapper")

		require.Error(t, wrappedErr)
		assert.Equal(t, "wrapper: standard error", wrappedErr.Error())
	})

	t.Run("stack frames", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")
		wrappedErr := Wrap(baseErr, "wrapper")

		pcs := wrappedErr.(*wrapped).StackFrames()

		assert.Len(t, pcs, 1)
	})

	t.Run("preserves root stack", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")
		wrappedErr1 := Wrap(baseErr, "wrapper1")
		wrappedErr2 := Wrap(wrappedErr1, "wrapper2")

		rootErr := Cause(wrappedErr2).(*root)

		assert.NotEmpty(t, rootErr.trace)
	})

	t.Run("double wrapping with fields", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base", WithField("base_key", "base_value"))
		wrappedErr := Wrap(baseErr, "wrapper", WithField("wrap_key", "wrap_value"))

		assert.Equal(t, map[string]interface{}{"base_key": "base_value"}, baseErr.(*root).fields)
		assert.Equal(t, map[string]interface{}{"wrap_key": "wrap_value"}, wrappedErr.(*wrapped).fields)
	})
}

func TestWrapDoesNotMutateCause(t *testing.T) {
	t.Parallel()

	t.Run("root trace is unchanged by wrapping", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")

		before := len(*baseErr.(*root).trace)

		_ = Wrap(baseErr, "wrapper1")
		_ = Wrap(baseErr, "wrapper2")

		after := len(*baseErr.(*root).trace)

		assert.Equal(t, before, after, "wrapping must not mutate the cause's trace")
	})

	t.Run("concurrent wrapping of a shared error is race-free", func(t *testing.T) {
		t.Parallel()

		shared := New("shared")

		var wg sync.WaitGroup

		for range 50 {
			wg.Go(func() {
				err := Wrap(shared, "ctx")

				_ = err.Error()
				_ = Cause(err).(*root).StackFrames()
			})
		}

		wg.Wait()
	})

	t.Run("wrapping with options does not mutate the cause", func(t *testing.T) {
		t.Parallel()

		base := New("base")

		_ = Wrap(base, "ctx", WithType("WRAP"), WithField("wrap_key", "v"))

		assert.Empty(t, base.(*root).Type())
		assert.Nil(t, base.(*root).Fields())
	})
}

func TestConcurrentFieldAccess(t *testing.T) {
	t.Parallel()

	err := New("base")

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Go(func() {
			_ = err.(*root).SetField(fmt.Sprintf("key-%d", i), i)
			_ = err.(*root).SetType(Type(fmt.Sprintf("TYPE_%d", i)))
		})

		wg.Go(func() {
			_ = err.(*root).Fields()
			_ = err.(*root).Type()
			_ = ToJSON(err, FormatterWithTrace())
			_ = ToString(err)
		})
	}

	wg.Wait()
}

func TestErrorOptions(t *testing.T) {
	t.Parallel()

	t.Run("with type", func(t *testing.T) {
		t.Parallel()

		opt := WithType("TEST")
		err := New("error", opt)

		assert.Equal(t, Type("TEST"), err.(*root).errType)
	})

	t.Run("with field", func(t *testing.T) {
		t.Parallel()

		opt := WithField("key", "value")
		err := New("error", opt)

		assert.Equal(t, map[string]interface{}{"key": "value"}, err.(*root).fields)
	})

	t.Run("multiple options", func(t *testing.T) {
		t.Parallel()

		err := New("error",
			WithType("TYPE"),
			WithField("key1", "value1"),
			WithField("key2", "value2"),
		)

		assert.Equal(t, Type("TYPE"), err.(*root).errType)
		assert.Equal(t, map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}, err.(*root).fields)
	})
}

func TestNilHandling(t *testing.T) {
	t.Parallel()

	t.Run("new with empty message", func(t *testing.T) {
		t.Parallel()

		err := New("")

		require.Error(t, err)
	})

	t.Run("wrap nil error", func(t *testing.T) {
		t.Parallel()

		err := Wrap(nil, "wrapper")

		assert.NoError(t, err)
	})

	t.Run("is with nil", func(t *testing.T) {
		t.Parallel()

		assert.True(t, Is(nil, nil))
		assert.False(t, Is(New("error"), nil))
	})

	t.Run("as with nil", func(t *testing.T) {
		t.Parallel()

		var target *root

		assert.False(t, As(nil, &target))
		assert.Nil(t, target)
	})

	t.Run("unwrap nil", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, Unwrap(nil))
	})

	t.Run("cause nil", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, Cause(nil))
	})

	t.Run("join nil errors", func(t *testing.T) {
		t.Parallel()

		err := Join(nil, nil)

		assert.NoError(t, err)
	})
}

func TestStackPreservation(t *testing.T) {
	t.Parallel()

	t.Run("wrapping preserves original stack", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")
		wrappedErr := Wrap(baseErr, "wrapper")

		rootErr := Cause(wrappedErr).(*root)

		assert.NotEmpty(t, rootErr.trace)
		assert.Greater(t, len(*rootErr.trace), 1)
	})

	t.Run("double wrapping", func(t *testing.T) {
		t.Parallel()

		baseErr := New("base")
		wrappedErr1 := Wrap(baseErr, "wrapper1")
		wrappedErr2 := Wrap(wrappedErr1, "wrapper2")

		rootErr := Cause(wrappedErr2).(*root)

		assert.NotEmpty(t, rootErr.trace)
	})
}

func TestIs(t *testing.T) {
	t.Parallel()

	err1 := New("1")
	err1a := Wrap(err1, "wrap 2")
	err1b := Wrap(err1a, "wrap 3")

	err2 := stderrors.New("2")
	err2a := fmt.Errorf("wrap 2: %w", err1)

	joinedErr := Join(err1, err2)

	tests := []struct {
		err    error
		target error
		match  bool
	}{
		{nil, nil, true},
		{nil, err1, false},
		{err1, nil, false},
		{err1, err1, true},
		{err1a, err1, true},
		{err1b, err1, true},
		{nil, err2, false},
		{err2, nil, false},
		{err2, err2, true},
		{err2a, err2, false},
		{joinedErr, err1, true},
		{joinedErr, err2, true},
		{joinedErr, New("3"), false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			if tt.match {
				assert.True(t, Is(tt.err, tt.target))
			} else {
				assert.False(t, Is(tt.err, tt.target))
			}
		})
	}
}

func TestAs(t *testing.T) {
	t.Parallel()

	err1 := New("1")
	err1a := Wrap(err1, "wrap 2")

	joinedErr := Join(err1, err1a)

	tests := []struct {
		name   string
		err    error
		target func() any
		match  bool
	}{
		{"nil error and target", nil, func() any { return nil }, false},
		{"root to Error", err1, func() any { return new(Error) }, true},
		{"root to *root", err1, func() any { return new(*root) }, true},
		{"wrapped to *wrapped", err1a, func() any { return new(*wrapped) }, true},
		{"joined to Error", joinedErr, func() any { return new(Error) }, true},
		{"joined to *root", joinedErr, func() any { return new(*root) }, true},
		{"joined to *wrapped", joinedErr, func() any { return new(*wrapped) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.match {
				assert.True(t, As(tt.err, tt.target()))
			} else {
				assert.False(t, As(tt.err, tt.target()))
			}
		})
	}
}

func TestJoin(t *testing.T) {
	t.Parallel()

	t.Run("basic join", func(t *testing.T) {
		t.Parallel()

		err1 := New("error1")
		err2 := New("error2")

		joined := Join(err1, err2)

		require.Error(t, joined)
		assert.Equal(t, "error1\nerror2", joined.Error())
	})

	t.Run("join single error", func(t *testing.T) {
		t.Parallel()

		err := New("error")
		joined := Join(err)

		assert.Equal(t, err, joined)
	})

	t.Run("join no errors", func(t *testing.T) {
		t.Parallel()

		joined := Join()

		assert.NoError(t, joined)
	})

	t.Run("join with nil", func(t *testing.T) {
		t.Parallel()

		err := New("error")

		joined := Join(nil, err, nil)

		assert.Equal(t, err, joined)
	})

	t.Run("joined error is", func(t *testing.T) {
		t.Parallel()

		err1 := New("error1")
		err2 := New("error2")

		joined := Join(err1, err2).(*joined)

		assert.True(t, joined.Is(err1))
		assert.True(t, joined.Is(err2))
		assert.False(t, joined.Is(New("error3")))
	})

	t.Run("joined error as", func(t *testing.T) {
		t.Parallel()

		err1 := New("error1")
		err2 := New("error2")

		joined := Join(err1, err2).(*joined)

		var target *root

		assert.True(t, joined.As(&target))
	})

	t.Run("joined stack frames", func(t *testing.T) {
		t.Parallel()

		err1 := New("error1")
		err2 := New("error2")

		joined := Join(err1, err2).(*joined)

		frames := joined.StackFrames()

		assert.NotEmpty(t, frames)
	})

	t.Run("joined unwrap", func(t *testing.T) {
		t.Parallel()

		err1 := New("error1")
		err2 := New("error2")

		joined := Join(err1, err2).(*joined)

		unwrapped := joined.Unwrap()

		assert.Equal(t, []error{err1, err2}, unwrapped)
	})
}

func TestCause(t *testing.T) {
	t.Parallel()

	t.Run("cause of root", func(t *testing.T) {
		t.Parallel()

		err := New("error")
		cause := Cause(err)

		assert.Equal(t, err, cause)
	})

	t.Run("cause of wrapped", func(t *testing.T) {
		t.Parallel()

		rootErr := New("root")
		wrappedErr := Wrap(rootErr, "wrapped")

		cause := Cause(wrappedErr)

		assert.Equal(t, rootErr, cause)
	})

	t.Run("cause of double wrapped", func(t *testing.T) {
		t.Parallel()

		rootErr := New("root")
		wrappedErr1 := Wrap(rootErr, "wrapped1")
		wrappedErr2 := Wrap(wrappedErr1, "wrapped2")

		cause := Cause(wrappedErr2)

		assert.Equal(t, rootErr, cause)
	})

	t.Run("cause of joined", func(t *testing.T) {
		t.Parallel()

		err1 := New("error1")
		err2 := New("error2")

		joined := Join(err1, err2)

		cause := Cause(joined)

		assert.Equal(t, joined, cause)
	})

	t.Run("cause through a foreign fmt wrap", func(t *testing.T) {
		t.Parallel()

		base := New("base")
		foreign := fmt.Errorf("foreign ctx: %w", base)
		top := Wrap(foreign, "top")

		assert.Equal(t, base, Cause(top))
	})

	t.Run("cyclic unwrap chain terminates", func(t *testing.T) {
		t.Parallel()

		a := &cyclicError{}
		b := &cyclicError{}
		a.next = b
		b.next = a

		assert.Same(t, a, Cause(a), "a cycle back to a visited error must terminate")
	})

	t.Run("self-referential unwrap terminates", func(t *testing.T) {
		t.Parallel()

		a := &cyclicError{}
		a.next = a

		assert.Same(t, a, Cause(a))
	})
}

func TestWrappedMethods(t *testing.T) {
	t.Parallel()

	newWrapped := func() *wrapped {
		return Wrap(New("base"), "ctx").(*wrapped)
	}

	t.Run("type", func(t *testing.T) {
		t.Parallel()

		typed := Wrap(New("base"), "ctx", WithType("WT")).(*wrapped)

		assert.Equal(t, Type("WT"), typed.Type())
		assert.Empty(t, newWrapped().Type())
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "ctx: base", newWrapped().Error())
	})

	t.Run("fields", func(t *testing.T) {
		t.Parallel()

		withField := Wrap(New("base"), "ctx", WithField("k", "v")).(*wrapped)

		assert.Equal(t, map[string]any{"k": "v"}, withField.Fields())
		assert.Nil(t, newWrapped().Fields())
	})

	t.Run("stack frames", func(t *testing.T) {
		t.Parallel()

		assert.Len(t, newWrapped().StackFrames(), 1)
	})

	t.Run("stack", func(t *testing.T) {
		t.Parallel()

		frames := newWrapped().Stack()

		require.Len(t, frames, 1)
		assert.Contains(t, frames[0].Name, "TestWrappedMethods")
	})

	t.Run("is matching and non-matching", func(t *testing.T) {
		t.Parallel()

		w := Wrap(New("base"), "ctx", WithType("WT")).(*wrapped)
		same := Wrap(New("base"), "ctx", WithType("WT")).(*wrapped)
		diffType := Wrap(New("base"), "ctx", WithType("OTHER")).(*wrapped)
		diffMsg := Wrap(New("base"), "other", WithType("WT")).(*wrapped)

		assert.True(t, w.Is(same))
		assert.False(t, w.Is(diffType))
		assert.False(t, w.Is(diffMsg))
		assert.False(t, w.Is(New("base")), "different concrete type never matches")
		assert.False(t, w.Is(nil), "nil target with non-nil receiver")
	})

	t.Run("is empty target type matches any type", func(t *testing.T) {
		t.Parallel()

		w := Wrap(New("base"), "ctx", WithType("WT")).(*wrapped)
		untypedTarget := Wrap(New("base"), "ctx").(*wrapped)

		assert.True(t, w.Is(untypedTarget))
	})

	t.Run("as to concrete type", func(t *testing.T) {
		t.Parallel()

		w := newWrapped()

		var target *wrapped

		assert.True(t, w.As(&target))
		assert.Equal(t, w, target)
	})

	t.Run("as to interface", func(t *testing.T) {
		t.Parallel()

		var target Error

		assert.True(t, newWrapped().As(&target))
		assert.NotNil(t, target)
	})

	t.Run("as incompatible type", func(t *testing.T) {
		t.Parallel()

		var target *root

		assert.False(t, newWrapped().As(&target))
	})

	t.Run("as invalid target", func(t *testing.T) {
		t.Parallel()

		w := newWrapped()

		assert.False(t, w.As(nil))
		assert.False(t, w.As(42), "non-pointer target")
	})

	t.Run("unwrap", func(t *testing.T) {
		t.Parallel()

		base := New("base")
		w := Wrap(base, "ctx").(*wrapped)

		assert.Equal(t, base, w.Unwrap())
	})

	t.Run("set type chains", func(t *testing.T) {
		t.Parallel()

		w := newWrapped()
		got := w.SetType("NEW")

		assert.Equal(t, Type("NEW"), w.Type())
		assert.Equal(t, w, got)
	})

	t.Run("set field chains", func(t *testing.T) {
		t.Parallel()

		w := newWrapped()
		got := w.SetField("k", 1)

		assert.Equal(t, 1, w.Fields()["k"])
		assert.Equal(t, w, got)
	})
}

func TestRootNilReceiver(t *testing.T) {
	t.Parallel()

	var e *root

	assert.Empty(t, e.Type())
	assert.Equal(t, "<nil>", e.Error())
	assert.Nil(t, e.Fields())
	assert.Empty(t, e.Stack())
	assert.Empty(t, e.StackFrames())
	require.NoError(t, e.Unwrap())
	assert.Nil(t, e.SetType("X"))
	assert.Nil(t, e.SetField("k", "v"))
	assert.False(t, e.As(nil))
}

func TestWrappedNilReceiver(t *testing.T) {
	t.Parallel()

	var e *wrapped

	assert.Empty(t, e.Type())
	assert.Equal(t, "<nil>", e.Error())
	assert.Nil(t, e.Fields())
	assert.Empty(t, e.Stack())
	assert.Empty(t, e.StackFrames())
	require.NoError(t, e.Unwrap())
	assert.Nil(t, e.SetType("X"))
	assert.Nil(t, e.SetField("k", "v"))
	assert.True(t, e.Is(nil), "nil target with nil receiver matches")
	assert.False(t, e.As(nil))
}

func TestWrappedStackFramesNilFrame(t *testing.T) {
	t.Parallel()

	assert.Empty(t, (&wrapped{}).StackFrames(), "nil frame yields no frames")
}

func TestWrappedStackFramesSymbolizeLikeStack(t *testing.T) {
	t.Parallel()

	err := Wrap(New("base"), "ctx").(*wrapped)

	pcs := err.StackFrames()
	require.Len(t, pcs, 1)

	runtimeFrame, _ := runtime.CallersFrames(pcs).Next()

	expectedName := runtimeFrame.Function

	if idx := strings.LastIndex(expectedName, "/"); idx >= 0 {
		expectedName = expectedName[idx+1:]
	}

	frames := err.Stack()
	require.Len(t, frames, 1)

	assert.Equal(t, expectedName, frames[0].Name)
	assert.Equal(t, runtimeFrame.File, frames[0].File)
	assert.Equal(t, runtimeFrame.Line, frames[0].Line)
	assert.Contains(t, frames[0].Name, "TestWrappedStackFramesSymbolizeLikeStack")
}

func TestFieldsReturnsCopy(t *testing.T) {
	t.Parallel()

	t.Run("root", func(t *testing.T) {
		t.Parallel()

		err := New("x", WithField("k", "v")).(*root)

		snapshot := err.Fields()
		snapshot["k"] = "mutated"
		snapshot["new"] = "added"

		assert.Equal(t, "v", err.Fields()["k"])
		assert.NotContains(t, err.Fields(), "new")
	})

	t.Run("wrapped", func(t *testing.T) {
		t.Parallel()

		err := Wrap(New("base"), "ctx", WithField("k", "v")).(*wrapped)

		snapshot := err.Fields()
		snapshot["k"] = "mutated"

		assert.Equal(t, "v", err.Fields()["k"])
	})
}

func TestJoinedMethods(t *testing.T) {
	t.Parallel()

	t.Run("error nil receiver", func(t *testing.T) {
		t.Parallel()

		var e *joined

		assert.Empty(t, e.Error())
	})

	t.Run("error with no entries", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, (&joined{}).Error())
	})

	t.Run("error skips nil entries", func(t *testing.T) {
		t.Parallel()

		e := &joined{errs: []error{New("a"), nil, New("b")}}

		assert.Equal(t, "a\nb", e.Error())
	})

	t.Run("stack frames nil receiver", func(t *testing.T) {
		t.Parallel()

		var e *joined

		assert.Empty(t, e.StackFrames())
	})

	t.Run("stack frames nil trace", func(t *testing.T) {
		t.Parallel()

		e := &joined{errs: []error{New("a")}}

		assert.Empty(t, e.StackFrames())
	})

	t.Run("is nil target", func(t *testing.T) {
		t.Parallel()

		e := Join(New("a"), New("b")).(*joined)

		assert.False(t, e.Is(nil))
	})

	t.Run("is no match", func(t *testing.T) {
		t.Parallel()

		e := Join(New("a"), New("b")).(*joined)

		assert.False(t, e.Is(New("c")))
	})

	t.Run("as no match", func(t *testing.T) {
		t.Parallel()

		e := Join(New("a"), New("b")).(*joined)

		var target *customAsError

		assert.False(t, e.As(&target))
	})

	t.Run("unwrap nil receiver", func(t *testing.T) {
		t.Parallel()

		var e *joined

		assert.Nil(t, e.Unwrap())
	})
}

func TestRootIsAndAsEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("is nil target", func(t *testing.T) {
		t.Parallel()

		assert.False(t, New("x").(*root).Is(nil))
	})

	t.Run("as edge cases", func(t *testing.T) {
		t.Parallel()

		e := New("x").(*root)

		var nilPtr *root

		assert.False(t, e.As(nilPtr), "typed nil pointer target")
		assert.False(t, e.As(42), "non-pointer target")

		var incompatible *wrapped

		assert.False(t, e.As(&incompatible), "incompatible concrete type")
	})
}

func TestPackageAsPanicsOnInvalidTarget(t *testing.T) {
	t.Parallel()

	err := New("x")

	assert.PanicsWithValue(t, "errors: target cannot be nil", func() {
		As(err, nil)
	}, "nil target")

	assert.PanicsWithValue(t, "errors: target must be a non-nil pointer", func() {
		As(err, 42)
	}, "non-pointer target")

	assert.PanicsWithValue(t, "errors: *target must be interface or implement error", func() {
		var notError int

		As(err, &notError)
	}, "target type does not implement error")
}

func TestAsResolverBranches(t *testing.T) {
	t.Parallel()

	t.Run("custom As method", func(t *testing.T) {
		t.Parallel()

		var target *asTargetError

		ok := As(&customAsError{}, &target)

		require.True(t, ok)
		require.NotNil(t, target)
		assert.True(t, target.filled)
	})

	t.Run("foreign multi-error with nil entry", func(t *testing.T) {
		t.Parallel()

		want := New("x", WithType("WANT")).(*root)
		me := &multiError{errs: []error{nil, stderrors.New("other"), want}}

		var target *root

		require.True(t, As(me, &target))
		assert.Equal(t, want, target)
	})

	t.Run("foreign multi-error no match", func(t *testing.T) {
		t.Parallel()

		me := &multiError{errs: []error{stderrors.New("a"), stderrors.New("b")}}

		var target *root

		assert.False(t, As(me, &target))
	})
}

func TestIsResolverBranches(t *testing.T) {
	t.Parallel()

	t.Run("custom Is method", func(t *testing.T) {
		t.Parallel()

		err := &customIsError{token: "abc"}

		assert.True(t, Is(err, &customIsError{token: "abc"}))
		assert.False(t, Is(err, &customIsError{token: "xyz"}))
	})

	t.Run("foreign multi-error", func(t *testing.T) {
		t.Parallel()

		sentinel := stderrors.New("sentinel")
		me := &multiError{errs: []error{stderrors.New("other"), sentinel}}

		assert.True(t, Is(me, sentinel))
		assert.False(t, Is(me, stderrors.New("missing")))
	})
}

type asTargetError struct {
	filled bool
}

func (e *asTargetError) Error() string {
	return "as-target"
}

type customAsError struct{}

func (e *customAsError) Error() string {
	return "custom-as"
}

func (e *customAsError) As(target any) bool {
	p, ok := target.(**asTargetError)
	if !ok {
		return false
	}

	*p = &asTargetError{filled: true}

	return true
}

type customIsError struct {
	token string
}

func (e *customIsError) Error() string {
	return "custom-is"
}

func (e *customIsError) Is(target error) bool {
	t, ok := target.(*customIsError)

	return ok && t.token == e.token
}

type multiError struct {
	errs []error
}

func (e *multiError) Error() string {
	return "multi"
}

func (e *multiError) Unwrap() []error {
	return e.errs
}

type cyclicError struct {
	next *cyclicError
}

func (e *cyclicError) Error() string {
	return "cyclic"
}

func (e *cyclicError) Unwrap() error {
	return e.next
}

func TestWrapNilCauseWithOptions(t *testing.T) {
	t.Parallel()

	assert.NoError(t, Wrap(nil, "ctx", WithType("T"), WithField("k", "v")), "wrapping nil must return nil even with options")
}

func TestIsConcurrentWithTargetSetType(t *testing.T) {
	t.Parallel()

	t.Run("root", func(t *testing.T) {
		t.Parallel()

		target := New("x", WithType("A"))
		err := New("x", WithType("A"))

		var wg sync.WaitGroup

		for range 50 {
			wg.Go(func() {
				_ = target.(MutableError).SetType("B")
			})

			wg.Go(func() {
				_ = Is(err, target)
			})
		}

		wg.Wait()
	})

	t.Run("wrapped", func(t *testing.T) {
		t.Parallel()

		target := Wrap(New("base"), "ctx", WithType("A"))
		err := Wrap(New("base"), "ctx", WithType("A"))

		var wg sync.WaitGroup

		for range 50 {
			wg.Go(func() {
				_ = target.(MutableError).SetType("B")
			})

			wg.Go(func() {
				_ = Is(err, target)
			})
		}

		wg.Wait()
	})
}

func TestIsTypedNilTarget(t *testing.T) {
	t.Parallel()

	err := New("x")

	var nilRoot *root

	var nilWrapped *wrapped

	assert.False(t, Is(err, nilRoot), "typed-nil *root target must not match or panic")
	assert.False(t, Is(err, nilWrapped), "typed-nil *wrapped target must not match or panic")
	assert.False(t, err.(*root).Is(nilRoot), "typed-nil *root target must not match or panic")
	assert.False(t, nilRoot.Is(err.(*root)), "nil receiver with non-nil target must not match or panic")
	assert.False(t, nilWrapped.Is(Wrap(New("y"), "ctx").(*wrapped)), "nil receiver with non-nil target must not match or panic")
}

func TestErrorEmptyMessageWithCause(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "inner", Wrap(New("inner"), "").Error(), "no leading colon for empty message")
	assert.Equal(t, "ext", Wrap(stderrors.New("ext"), "").Error(), "no leading colon for empty message wrapping an external error")
	assert.Equal(t, "outer: inner", Wrap(New("inner"), "outer").Error())
}

func TestStackFramesReturnsCopy(t *testing.T) {
	t.Parallel()

	t.Run("root", func(t *testing.T) {
		t.Parallel()

		err := New("x").(*root)

		frames := err.StackFrames()
		require.NotEmpty(t, frames)

		original := frames[0]
		frames[0] = 0

		assert.Equal(t, original, err.StackFrames()[0], "mutating the returned slice must not affect the error")
	})

	t.Run("joined", func(t *testing.T) {
		t.Parallel()

		err := Join(New("a"), New("b")).(*joined)

		frames := err.StackFrames()
		require.NotEmpty(t, frames)

		original := frames[0]
		frames[0] = 0

		assert.Equal(t, original, err.StackFrames()[0], "mutating the returned slice must not affect the error")
	})

	t.Run("wrapped", func(t *testing.T) {
		t.Parallel()

		err := Wrap(New("base"), "ctx").(*wrapped)

		frames := err.StackFrames()
		require.Len(t, frames, 1)

		frames[0] = 0

		assert.NotZero(t, err.StackFrames()[0], "mutating the returned slice must not affect the error")
	})
}

func TestNilOptionFuncsIgnored(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		_ = New("x", nil)
		_ = Wrap(New("y"), "ctx", nil)
		_ = NewFormatter(nil)
	})
}

func TestStdlibInterop(t *testing.T) {
	t.Parallel()

	t.Run("stdlib Is traverses package chain", func(t *testing.T) {
		t.Parallel()

		base := New("base")
		wrapped := Wrap(base, "ctx")

		assert.ErrorIs(t, wrapped, base)
	})

	t.Run("stdlib Is uses package value comparison", func(t *testing.T) {
		t.Parallel()

		err := New("x", WithType("T"))
		target := New("x", WithType("T"))

		assert.ErrorIs(t, err, target)
	})

	t.Run("stdlib As recovers the Error interface", func(t *testing.T) {
		t.Parallel()

		var e Error

		require.ErrorAs(t, Wrap(New("base"), "ctx"), &e)
		assert.Equal(t, "ctx: base", e.Error())
	})

	t.Run("package Is traverses a foreign fmt wrap", func(t *testing.T) {
		t.Parallel()

		base := New("base")
		foreign := fmt.Errorf("foreign ctx: %w", base)

		assert.True(t, Is(foreign, base))
	})

	t.Run("package As traverses a foreign fmt wrap", func(t *testing.T) {
		t.Parallel()

		base := New("base")
		foreign := fmt.Errorf("foreign ctx: %w", base)

		var target *root

		require.True(t, As(foreign, &target))
		assert.Equal(t, base, target)
	})
}

func TestRootIsTypeMatching(t *testing.T) {
	t.Parallel()

	typed := New("m", WithType("A")).(*root)
	untyped := New("m").(*root)

	t.Run("untyped target matches typed", func(t *testing.T) {
		t.Parallel()

		assert.True(t, typed.Is(untyped))
	})

	t.Run("typed target does not match untyped", func(t *testing.T) {
		t.Parallel()

		assert.False(t, untyped.Is(typed))
	})

	t.Run("different concrete type never matches", func(t *testing.T) {
		t.Parallel()

		assert.False(t, typed.Is(Wrap(New("m"), "ctx").(*wrapped)))
	})
}

func TestAsMutableError(t *testing.T) {
	t.Parallel()

	err := Wrap(New("base"), "ctx")

	var me MutableError

	require.True(t, As(err, &me))

	_ = me.SetType("LATE")
	_ = me.SetField("late", true)

	assert.Equal(t, Type("LATE"), me.Type())
	assert.Equal(t, map[string]any{"late": true}, me.Fields())
}

func TestNestedJoin(t *testing.T) {
	t.Parallel()

	inner := Join(New("a"), New("b"))
	c := New("c")
	outer := Join(inner, c)

	t.Run("error message flattens", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "a\nb\nc", outer.Error())
	})

	t.Run("is traverses nesting", func(t *testing.T) {
		t.Parallel()

		assert.True(t, Is(outer, New("a")))
	})

	t.Run("as traverses nesting", func(t *testing.T) {
		t.Parallel()

		var target *root

		assert.True(t, As(outer, &target))
	})

	t.Run("unwrap returns immediate members only", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []error{inner, c}, outer.(*joined).Unwrap())
	})
}

func TestNewCapturesAtMost64Frames(t *testing.T) {
	t.Parallel()

	var deep func(n int) error

	deep = func(n int) error {
		if n == 0 {
			return New("deep")
		}

		return deep(n - 1)
	}

	pcs := deep(128).(*root).StackFrames()

	assert.Len(t, pcs, 64, "capture is capped at 64 frames")
}

func TestSettersOverwrite(t *testing.T) {
	t.Parallel()

	err := New("x", WithType("A"), WithField("k", 1)).(*root)

	_ = err.SetType("B")
	_ = err.SetField("k", 2)

	assert.Equal(t, Type("B"), err.Type())
	assert.Equal(t, 2, err.Fields()["k"])
}

var (
	benchmarkSink   any
	benchmarkString string
)

// benchmarkWrapChain builds a wrap chain of the given total depth on top of a
// root error: depth 1 is a bare root, depth 2 is one Wrap around a root, etc.
func benchmarkWrapChain(depth int) (err error) {
	err = New("root error")

	for i := 1; i < depth; i++ {
		err = Wrap(err, fmt.Sprintf("wrap %d", i))
	}

	return err
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		benchmarkSink = New("benchmark error")
	}
}

func BenchmarkErrorDeepChain(b *testing.B) {
	err := benchmarkWrapChain(16)

	b.ReportAllocs()

	for b.Loop() {
		benchmarkString = err.Error()
	}
}

func BenchmarkCause(b *testing.B) {
	err := benchmarkWrapChain(16)

	b.ReportAllocs()

	for b.Loop() {
		benchmarkSink = Cause(err)
	}
}
