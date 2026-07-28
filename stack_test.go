package errors

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrame_resolveToStackFrame(t *testing.T) {
	t.Parallel()

	pc := [1]uintptr{}

	runtime.Callers(1, pc[:])

	f := frame(pc[0])

	// The frame stores the raw return PC exactly as runtime.Callers reports
	// it, so symbolization must match resolving that PC unchanged.
	frames := runtime.CallersFrames(pc[:])

	runtimeFrame, _ := frames.Next()

	expectedName := runtimeFrame.Function

	if idx := strings.LastIndex(expectedName, "/"); idx >= 0 {
		expectedName = expectedName[idx+1:]
	}

	result := f.resolveToStackFrame()

	assert.Equal(t, expectedName, result.Name)
	assert.Equal(t, runtimeFrame.File, result.File)
	assert.Equal(t, runtimeFrame.Line, result.Line)
}

func TestStack_resolveToStackFrames(t *testing.T) {
	t.Parallel()

	const depth = 3

	var pcs [depth]uintptr

	n := runtime.Callers(1, pcs[:])

	s := stack(pcs[:n])

	result := s.resolveToStackFrames()

	frames := runtime.CallersFrames(pcs[:n])

	i := 0

	for {
		runtimeFrame, more := frames.Next()

		// resolveToStackFrames drops runtime-internal frames, so only
		// non-runtime frames are compared against its output.
		if !strings.HasPrefix(runtimeFrame.Function, "runtime.") {
			require.Less(t, i, len(result), "More non-runtime frames from runtime than from resolveToStackFrames")

			assert.Equal(t, filepath.Base(runtimeFrame.Function), filepath.Base(result[i].Name))
			assert.Equal(t, runtimeFrame.File, result[i].File)
			assert.Equal(t, runtimeFrame.Line, result[i].Line)

			i++
		}

		if !more {
			break
		}
	}

	assert.Len(t, result, i, "Should have one frame per non-runtime input PC")
}

func TestCaller(t *testing.T) {
	t.Parallel()

	_, file, line, ok := runtime.Caller(0)

	require.True(t, ok, "runtime.Caller failed")

	result := caller(2)

	require.NotNil(t, result, "caller returned nil")

	resolved := result.resolveToStackFrame()

	assert.Equal(t, "TestCaller", strings.Split(resolved.Name, ".")[1])
	assert.Equal(t, file, resolved.File)
	assert.Greater(t, resolved.Line, line)

	nilResult := caller(1000)

	assert.Nil(t, nilResult)
}

func TestCallers(t *testing.T) {
	t.Parallel()

	result := callers(0)

	require.NotNil(t, result)

	assert.NotEmpty(t, *result, "Should get at least one frame")

	frames := result.resolveToStackFrames()

	for _, frame := range frames {
		assert.False(t, strings.HasPrefix(frame.Name, "runtime."), "Resolution should filter out runtime frames")
	}

	innerResult := callers(2)

	require.NotNil(t, innerResult)

	innerFrames := innerResult.resolveToStackFrames()

	if len(frames) > 0 && len(innerFrames) > 0 {
		assert.NotEqual(t, frames[0].Name, innerFrames[0].Name, "First frame should be different when skipping")
	}

	emptyResult := callers(1000)

	assert.Empty(t, *emptyResult)
}

func TestFrame_resolveToStackFrame_EdgeCases(t *testing.T) {
	t.Parallel()

	f := frame(0)

	result := f.resolveToStackFrame()

	assert.Empty(t, result.Name)
	assert.Empty(t, result.File)
	assert.Zero(t, result.Line)
}

func TestStack_resolveToStackFrames_InvalidPCs(t *testing.T) {
	t.Parallel()

	s := stack{0}

	result := s.resolveToStackFrames()

	assert.Len(t, result, 1)
	assert.Empty(t, result[0].Name)
	assert.Empty(t, result[0].File)
	assert.Zero(t, result[0].Line)
}
