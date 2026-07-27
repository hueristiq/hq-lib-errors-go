package errors

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFormatter(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		f := NewFormatter()

		assert.False(t, f.options.IsInnerFirst)
		assert.False(t, f.options.WithTrace)
		assert.False(t, f.options.InvertTrace)
		assert.True(t, f.options.WithExternal)
		assert.Equal(t, " ", f.options.Spacing)
		assert.Equal(t, "  ", f.options.Indentation)
	})

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()

		f := NewFormatter(FormatWithTrace(), FormatWithInnerFirst(), FormatWithoutExternal())

		assert.True(t, f.options.WithTrace)
		assert.True(t, f.options.IsInnerFirst)
		assert.False(t, f.options.WithExternal)
	})
}

func TestFormatterOptionConstructors(t *testing.T) {
	t.Parallel()

	f := NewFormatter(FormatWithTrace(), FormatWithInnerFirst(), FormatWithoutExternal(), FormatWithInvertedTrace())

	assert.True(t, f.options.WithTrace)
	assert.True(t, f.options.IsInnerFirst)
	assert.False(t, f.options.WithExternal)
	assert.True(t, f.options.InvertTrace)
}

func TestFormatWithTrace(t *testing.T) {
	t.Parallel()

	options := &FormatterOptions{}

	FormatWithTrace()(options)

	assert.True(t, options.WithTrace)
}

func TestUnpack(t *testing.T) {
	t.Parallel()

	stdErr := stderrors.New("external")

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(nil)

		assert.Empty(t, u.ErrRoot.Message)
		assert.Empty(t, u.ErrChain)
		require.NoError(t, u.ErrExternal)
		assert.Nil(t, u.ErrJoined)
	})

	t.Run("root error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(New("root", WithType("T"), WithField("k", "v")))

		assert.Equal(t, "root", u.ErrRoot.Message)
		assert.Equal(t, Type("T"), u.ErrRoot.Type)
		assert.Equal(t, map[string]any{"k": "v"}, u.ErrRoot.Fields)
		assert.NotEmpty(t, u.ErrRoot.Stack, "Unpack resolves stacks")
		assert.Empty(t, u.ErrChain)
	})

	t.Run("wrapped chain", func(t *testing.T) {
		t.Parallel()

		u := Unpack(Wrap(New("root"), "wrap"))

		require.Len(t, u.ErrChain, 1)
		assert.Equal(t, "wrap", u.ErrChain[0].Message)
		assert.Len(t, u.ErrChain[0].Stack, 1)
		assert.Equal(t, "root", u.ErrRoot.Message)
	})

	t.Run("external error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(stdErr)

		assert.Equal(t, stdErr, u.ErrExternal)
		assert.Empty(t, u.ErrRoot.Message)
		assert.Empty(t, u.ErrChain)
	})

	t.Run("wrapped external error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(Wrap(stdErr, "wrap"))

		assert.Equal(t, "wrap", u.ErrRoot.Message)
		assert.Equal(t, stdErr, u.ErrExternal)
	})

	t.Run("joined error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(Join(New("a"), New("b")))

		require.Len(t, u.ErrJoined, 2)
		assert.Empty(t, u.ErrChain)
	})
}

func TestUnpackStopsAtFirstExternal(t *testing.T) {
	t.Parallel()

	base := New("root-msg", WithType("ROOT"))
	mid := fmt.Errorf("fmt ctx: %w", base)
	top := Wrap(mid, "top")

	u := Unpack(top)

	assert.Equal(t, "top", u.ErrRoot.Message)
	assert.Empty(t, u.ErrChain)
	assert.Equal(t, mid, u.ErrExternal, "traversal stops at the first non-package error")
}

func TestUnpackResolveStacksGate(t *testing.T) {
	t.Parallel()

	err := Wrap(New("root"), "wrap")

	withStacks := unpack(err, true)
	withoutStacks := unpack(err, false)

	assert.NotEmpty(t, withStacks.ErrRoot.Stack)
	assert.NotEmpty(t, withStacks.ErrChain[0].Stack)

	assert.Empty(t, withoutStacks.ErrRoot.Stack)
	assert.Empty(t, withoutStacks.ErrChain[0].Stack)
}

func TestToString(t *testing.T) {
	t.Parallel()

	stdErr := stderrors.New("ext")

	tests := []struct {
		name     string
		err      error
		opts     []FormatterOptionFunc
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "root without type",
			err:      New("boom"),
			expected: "boom",
		},
		{
			name:     "root with type",
			err:      New("boom", WithType("T")),
			expected: "[T] boom",
		},
		{
			name:     "root with single field",
			err:      New("boom", WithField("key", "value")),
			expected: "boom\n\nFields:\n  key: value",
		},
		{
			name:     "single wrap",
			err:      Wrap(New("base"), "wrap"),
			expected: "wrap\n\nbase",
		},
		{
			name:     "double wrap outer first",
			err:      Wrap(Wrap(New("root"), "w1"), "w2"),
			expected: "w2\n\nw1\n\nroot",
		},
		{
			name:     "double wrap inner first",
			err:      Wrap(Wrap(New("root"), "w1"), "w2"),
			opts:     []FormatterOptionFunc{FormatWithInnerFirst()},
			expected: "root\n\nw1\n\nw2",
		},
		{
			name:     "external only",
			err:      stdErr,
			expected: "ext",
		},
		{
			name:     "root and external",
			err:      Wrap(stdErr, "wrap"),
			expected: "wrap\n\next",
		},
		{
			name:     "root and external without external",
			err:      Wrap(stdErr, "wrap"),
			opts:     []FormatterOptionFunc{FormatWithoutExternal()},
			expected: "wrap",
		},
		{
			name:     "root and external inner first",
			err:      Wrap(stdErr, "wrap"),
			opts:     []FormatterOptionFunc{FormatWithInnerFirst()},
			expected: "ext\n\nwrap",
		},
		{
			name:     "external only is shown even without external",
			err:      stdErr,
			opts:     []FormatterOptionFunc{FormatWithoutExternal()},
			expected: "ext",
		},
		{
			name:     "joined",
			err:      Join(New("a"), New("b")),
			expected: "Multiple errors (2):\n\n1. a\n\n2. b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, ToString(tt.err, tt.opts...))
		})
	}
}

func TestToStringFieldsSorted(t *testing.T) {
	t.Parallel()

	err := New("boom", WithField("z", 1), WithField("a", 2), WithField("m", 3))

	assert.Equal(t, "boom\n\nFields:\n  a: 2\n  m: 3\n  z: 1", ToString(err), "fields must render in sorted key order")
}

func TestToStringWithTrace(t *testing.T) {
	t.Parallel()

	out := ToString(Wrap(New("base"), "wrap"), FormatWithTrace())

	assert.Contains(t, out, "root Trace:")
	assert.Contains(t, out, "wrap Trace:")
	assert.Contains(t, out, "format_test.go")
}

func TestToStringJoinedWithTrace(t *testing.T) {
	t.Parallel()

	out := ToString(Join(New("a"), New("b")), FormatWithTrace())

	assert.Contains(t, out, "Multiple errors (2):")
	assert.Contains(t, out, "Join Location:")
	assert.Contains(t, out, "format_test.go")
}

func TestToStringJoinedSkipsNilEntry(t *testing.T) {
	t.Parallel()

	j := &joined{errs: []error{New("a"), nil, New("b")}}

	out := ToString(j)

	assert.Contains(t, out, "Multiple errors (3):")
	assert.Contains(t, out, "1. a")
	assert.Contains(t, out, "3. b")
}

func TestToJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, ToJSON(nil))
	})

	t.Run("root with type and fields", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(New("root", WithType("T"), WithField("k", "v")))

		root, ok := m["root"].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "root", root["message"])
		assert.Equal(t, "T", root["type"])
		assert.Equal(t, map[string]any{"k": "v"}, root["fields"])
		assert.NotContains(t, root, "stack", "no stack without trace")
	})

	t.Run("chain", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(Wrap(New("root"), "wrap"))

		chain, ok := m["chain"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, chain, 1)

		assert.Equal(t, "wrap", chain[0]["message"])

		root, ok := m["root"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "root", root["message"])
	})

	t.Run("external", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(stderrors.New("ext"))

		external, ok := m["external"].(map[string]any)
		require.True(t, ok)

		assert.Equal(t, "ext", external["message"])
		assert.Contains(t, external, "go_type")
	})

	t.Run("joined", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(Join(New("a"), New("b")))

		assert.Equal(t, "joined", m["type"])
		assert.Equal(t, 2, m["count"])
		assert.Len(t, m["errors"], 2)
	})
}

func TestToJSONWithTrace(t *testing.T) {
	t.Parallel()

	m := ToJSON(New("trace me"), FormatWithTrace())

	root, ok := m["root"].(map[string]any)
	require.True(t, ok)

	stack, ok := root["stack"].([]map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, stack)

	first := stack[0]

	assert.Contains(t, first, "function")
	assert.Contains(t, first, "line")
	assert.Contains(t, first, "file")
	assert.Contains(t, first["file"], "format_test.go")
}

func TestToJSONJoinedWithTrace(t *testing.T) {
	t.Parallel()

	m := ToJSON(Join(New("a"), New("b")), FormatWithTrace())

	joinStack, ok := m["join_stack"].([]map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, joinStack)

	assert.Contains(t, joinStack[0], "function")
	assert.Contains(t, joinStack[0]["file"], "format_test.go")

	errs, ok := m["errors"].([]any)
	require.True(t, ok)
	assert.Len(t, errs, 2)
}

func TestToJSONString(t *testing.T) {
	t.Parallel()

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, ToJSONString(nil))
	})

	t.Run("chain with type and fields", func(t *testing.T) {
		t.Parallel()

		err := Wrap(New("root msg", WithType("RT"), WithField("a", 1), WithField("b", 2)), "w")

		expected := `{
			"chain": [{"message": "w"}],
			"root": {
				"message": "root msg",
				"type": "RT",
				"fields": {"a": 1, "b": 2}
			}
		}`

		assert.JSONEq(t, expected, ToJSONString(err))
	})

	t.Run("joined", func(t *testing.T) {
		t.Parallel()

		err := Join(New("a"), New("b"))

		expected := `{
			"type": "joined",
			"count": 2,
			"errors": [
				{"root": {"message": "a"}},
				{"root": {"message": "b"}}
			]
		}`

		assert.JSONEq(t, expected, ToJSONString(err))
	})

	t.Run("marshal error", func(t *testing.T) {
		t.Parallel()

		err := New("bad", WithField("ch", make(chan int)))

		assert.Contains(t, ToJSONString(err), "JSON formatting error")
	})
}

func TestFormatterInvertTrace(t *testing.T) {
	t.Parallel()

	err := New("trace me")

	normal := ToJSON(err, FormatWithTrace())
	inverted := ToJSON(err, FormatWithTrace(), FormatWithInvertedTrace())

	normalStack := normal["root"].(map[string]any)["stack"].([]map[string]any)
	invertedStack := inverted["root"].(map[string]any)["stack"].([]map[string]any)

	require.NotEmpty(t, normalStack)
	require.Len(t, invertedStack, len(normalStack))

	for i := range normalStack {
		assert.Equal(t, normalStack[i], invertedStack[len(invertedStack)-1-i])
	}
}

func TestFormatterIsInnerFirstJSON(t *testing.T) {
	t.Parallel()

	err := Wrap(Wrap(New("root"), "w1"), "w2")

	normalChain := ToJSON(err)["chain"].([]map[string]any)
	innerChain := ToJSON(err, FormatWithInnerFirst())["chain"].([]map[string]any)

	require.Len(t, normalChain, 2)
	require.Len(t, innerChain, 2)

	assert.Equal(t, "w2", normalChain[0]["message"])
	assert.Equal(t, "w1", normalChain[1]["message"])

	assert.Equal(t, "w1", innerChain[0]["message"])
	assert.Equal(t, "w2", innerChain[1]["message"])
}

func TestFormatterNil(t *testing.T) {
	t.Parallel()

	f := NewFormatter()

	assert.Empty(t, f.String(nil))
	assert.Nil(t, f.JSON(nil))
}

func TestHasRootContent(t *testing.T) {
	t.Parallel()

	f := NewFormatter()

	tests := []struct {
		name     string
		part     ErrPart
		expected bool
	}{
		{"empty", ErrPart{}, false},
		{"message", ErrPart{Message: "m"}, true},
		{"type", ErrPart{Type: "T"}, true},
		{"fields", ErrPart{Fields: map[string]any{"k": "v"}}, true},
		{"stack", ErrPart{Stack: Stack{{Name: "fn"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, f.hasRootContent(&tt.part))
		})
	}
}

func TestIsOnlyExternal(t *testing.T) {
	t.Parallel()

	f := NewFormatter()

	ext := stderrors.New("ext")

	tests := []struct {
		name     string
		unpacked UnpackedError
		expected bool
	}{
		{
			name:     "external only",
			unpacked: UnpackedError{ErrExternal: ext},
			expected: true,
		},
		{
			name:     "external with root",
			unpacked: UnpackedError{ErrExternal: ext, ErrRoot: ErrPart{Message: "r"}},
			expected: false,
		},
		{
			name:     "external with chain",
			unpacked: UnpackedError{ErrExternal: ext, ErrChain: []ErrPart{{Message: "w"}}},
			expected: false,
		},
		{
			name:     "no external",
			unpacked: UnpackedError{ErrRoot: ErrPart{Message: "r"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, f.isOnlyExternal(&tt.unpacked))
		})
	}
}

func TestFormatExternalString(t *testing.T) {
	t.Parallel()

	ext := stderrors.New("boom")

	assert.Equal(t, "boom", NewFormatter().formatExternalString(ext))
	assert.Equal(t, "boom", NewFormatter(FormatWithTrace()).formatExternalString(ext))
}

func TestFormatterSpacingAndIndentation(t *testing.T) {
	t.Parallel()

	err := New("boom", WithType("T"), WithField("key", "value"))

	out := ToString(err, func(o *FormatterOptions) {
		o.Spacing = "_"
		o.Indentation = ">>"
	})

	assert.Equal(t, "[T]_boom\n\nFields:\n>>key:_value", out)
}
