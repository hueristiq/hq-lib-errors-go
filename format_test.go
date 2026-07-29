package errors

import (
	stderrors "errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFormatter(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		f := NewFormatter()

		assert.False(t, f.options.InnerFirst)
		assert.False(t, f.options.WithTrace)
		assert.False(t, f.options.InvertTrace)
		assert.True(t, f.options.WithExternal)
		assert.Equal(t, " ", f.options.Spacing)
		assert.Equal(t, "  ", f.options.Indentation)
	})

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()

		f := NewFormatter(FormatterWithTrace(), FormatterWithInnerFirst(), FormatterWithoutExternal())

		assert.True(t, f.options.WithTrace)
		assert.True(t, f.options.InnerFirst)
		assert.False(t, f.options.WithExternal)
	})
}

func TestFormatterOptionConstructors(t *testing.T) {
	t.Parallel()

	f := NewFormatter(FormatterWithTrace(), FormatterWithInnerFirst(), FormatterWithoutExternal(), FormatterWithInvertedTrace())

	assert.True(t, f.options.WithTrace)
	assert.True(t, f.options.InnerFirst)
	assert.False(t, f.options.WithExternal)
	assert.True(t, f.options.InvertTrace)
}

func TestFormatterWithTrace(t *testing.T) {
	t.Parallel()

	options := &FormatterOptions{}

	FormatterWithTrace()(options)

	assert.True(t, options.WithTrace)
}

func TestUnpack(t *testing.T) {
	t.Parallel()

	stdErr := stderrors.New("external")

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(nil)

		assert.Empty(t, u.Root.Message)
		assert.Empty(t, u.Chain)
		require.NoError(t, u.External)
		assert.Nil(t, u.Joined)
	})

	t.Run("root error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(New("root", WithType("T"), WithField("k", "v")))

		assert.Equal(t, "root", u.Root.Message)
		assert.Equal(t, Type("T"), u.Root.Type)
		assert.Equal(t, map[string]any{"k": "v"}, u.Root.Fields)
		assert.NotEmpty(t, u.Root.Stack, "Unpack resolves stacks")
		assert.Empty(t, u.Chain)
	})

	t.Run("wrapped chain", func(t *testing.T) {
		t.Parallel()

		u := Unpack(Wrap(New("root"), "wrap"))

		require.Len(t, u.Chain, 1)
		assert.Equal(t, "wrap", u.Chain[0].Message)
		assert.Len(t, u.Chain[0].Stack, 1)
		assert.Equal(t, "root", u.Root.Message)
	})

	t.Run("external error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(stdErr)

		assert.Equal(t, stdErr, u.External)
		assert.Empty(t, u.Root.Message)
		assert.Empty(t, u.Chain)
	})

	t.Run("wrapped external error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(Wrap(stdErr, "wrap"))

		assert.Equal(t, "wrap", u.Root.Message)
		assert.Equal(t, stdErr, u.External)
	})

	t.Run("joined error", func(t *testing.T) {
		t.Parallel()

		u := Unpack(Join(New("a"), New("b")))

		require.Len(t, u.Joined, 2)
		assert.Empty(t, u.Chain)
	})
}

func TestUnpackStopsAtFirstExternal(t *testing.T) {
	t.Parallel()

	base := New("root-msg", WithType("ROOT"))
	mid := fmt.Errorf("fmt ctx: %w", base)
	top := Wrap(mid, "top")

	u := Unpack(top)

	assert.Equal(t, "top", u.Root.Message)
	assert.Empty(t, u.Chain)
	assert.Equal(t, mid, u.External, "traversal stops at the first non-package error")
}

func TestUnpackResolveStacksGate(t *testing.T) {
	t.Parallel()

	err := Wrap(New("root"), "wrap")

	withStacks := unpack(err, true)
	withoutStacks := unpack(err, false)

	assert.NotEmpty(t, withStacks.Root.Stack)
	assert.NotEmpty(t, withStacks.Chain[0].Stack)

	assert.Empty(t, withoutStacks.Root.Stack)
	assert.Empty(t, withoutStacks.Chain[0].Stack)
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
			opts:     []FormatterOptionFunc{FormatterWithInnerFirst()},
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
			opts:     []FormatterOptionFunc{FormatterWithoutExternal()},
			expected: "wrap",
		},
		{
			name:     "root and external inner first",
			err:      Wrap(stdErr, "wrap"),
			opts:     []FormatterOptionFunc{FormatterWithInnerFirst()},
			expected: "ext\n\nwrap",
		},
		{
			name:     "external only is shown even without external",
			err:      stdErr,
			opts:     []FormatterOptionFunc{FormatterWithoutExternal()},
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

	out := ToString(Wrap(New("base"), "wrap"), FormatterWithTrace())

	assert.Contains(t, out, "root Trace:")
	assert.Contains(t, out, "wrap Trace:")
	assert.Contains(t, out, "format_test.go")
}

func TestToStringJoinedWithTrace(t *testing.T) {
	t.Parallel()

	out := ToString(Join(New("a"), New("b")), FormatterWithTrace())

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

		assert.Equal(t, "joined", m["kind"])
		assert.Equal(t, 2, m["count"])
		assert.Len(t, m["errors"], 2)
	})

	t.Run("root omitted when it has no content", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(Wrap(stderrors.New("ext"), ""))

		assert.NotContains(t, m, "root")
		assert.NotContains(t, m, "chain")
		assert.Contains(t, m, "external")
	})

	t.Run("external excluded via option", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(Wrap(stderrors.New("ext"), "wrap"), FormatterWithoutExternal())

		assert.NotContains(t, m, "external")
		assert.Contains(t, m, "root")
	})
}

func TestToJSONWithTrace(t *testing.T) {
	t.Parallel()

	m := ToJSON(New("trace me"), FormatterWithTrace())

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

	m := ToJSON(Join(New("a"), New("b")), FormatterWithTrace())

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
			"kind": "joined",
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

	normal := ToJSON(err, FormatterWithTrace())
	inverted := ToJSON(err, FormatterWithTrace(), FormatterWithInvertedTrace())

	normalStack := normal["root"].(map[string]any)["stack"].([]map[string]any)
	invertedStack := inverted["root"].(map[string]any)["stack"].([]map[string]any)

	require.NotEmpty(t, normalStack)
	require.Len(t, invertedStack, len(normalStack))

	for i := range normalStack {
		assert.Equal(t, normalStack[i], invertedStack[len(invertedStack)-1-i])
	}
}

func TestFormatterInnerFirstJSON(t *testing.T) {
	t.Parallel()

	err := Wrap(Wrap(New("root"), "w1"), "w2")

	normalChain := ToJSON(err)["chain"].([]map[string]any)
	innerChain := ToJSON(err, FormatterWithInnerFirst())["chain"].([]map[string]any)

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

func TestZeroValueFormatter(t *testing.T) {
	t.Parallel()

	f := &Formatter{}

	t.Run("string matches defaults", func(t *testing.T) {
		t.Parallel()

		err := Wrap(New("base", WithType("T"), WithField("k", "v")), "wrap")

		assert.Equal(t, "wrap\n\n[T] base\n\nFields:\n  k: v", f.String(err))
		assert.Equal(t, "ext", f.String(stderrors.New("ext")))
		assert.Empty(t, f.String(nil))
	})

	t.Run("json matches defaults", func(t *testing.T) {
		t.Parallel()

		m := f.JSON(Wrap(New("root"), "wrap"))

		chain, ok := m["chain"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, chain, 1)
		assert.Equal(t, "wrap", chain[0]["message"])

		root, ok := m["root"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "root", root["message"])
		assert.NotContains(t, root, "stack", "no stack without trace")

		assert.Nil(t, f.JSON(nil))
	})

	t.Run("joined", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Multiple errors (2):\n\n1. a\n\n2. b", f.String(Join(New("a"), New("b"))))
	})
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
			unpacked: UnpackedError{External: ext},
			expected: true,
		},
		{
			name:     "external with root",
			unpacked: UnpackedError{External: ext, Root: ErrPart{Message: "r"}},
			expected: false,
		},
		{
			name:     "external with chain",
			unpacked: UnpackedError{External: ext, Chain: []ErrPart{{Message: "w"}}},
			expected: false,
		},
		{
			name:     "no external",
			unpacked: UnpackedError{Root: ErrPart{Message: "r"}},
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
	assert.Equal(t, "boom", NewFormatter(FormatterWithTrace()).formatExternalString(ext))
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

func TestFormatVerbs(t *testing.T) {
	t.Parallel()

	err := Wrap(New("base", WithType("T")), "wrap")

	t.Run("percent v prints message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "wrap: base", formatf("%v", err))
	})

	t.Run("percent s prints message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "wrap: base", formatf("%s", err))
	})

	t.Run("percent q quotes message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `"wrap: base"`, formatf("%q", err))
	})

	t.Run("percent plus v includes traces", func(t *testing.T) {
		t.Parallel()

		out := formatf("%+v", err)

		assert.Contains(t, out, "wrap Trace:")
		assert.Contains(t, out, "root Trace:")
		assert.Contains(t, out, "format_test.go")
	})

	t.Run("joined percent plus v", func(t *testing.T) {
		t.Parallel()

		out := formatf("%+v", Join(New("a"), New("b")))

		assert.Contains(t, out, "Multiple errors (2):")
		assert.Contains(t, out, "Join Location:")
	})

	t.Run("joined percent v prints messages", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "a\nb", formatf("%v", Join(New("a"), New("b"))))
	})

	t.Run("joined percent q quotes messages", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, strconv.Quote("a\nb"), formatf("%q", Join(New("a"), New("b"))))
	})
}

func formatf(verb string, err error) (out string) {
	var buf strings.Builder

	fmt.Fprintf(&buf, verb, err)

	return buf.String()
}

func TestFormatNilReceivers(t *testing.T) {
	t.Parallel()

	var (
		rootErr    *root
		wrappedErr *wrapped
		joinedErr  *joined
	)

	assert.NotPanics(t, func() {
		assert.Equal(t, "<nil>", formatf("%v", rootErr))
		assert.Equal(t, "<nil>", formatf("%+v", wrappedErr))
		assert.Equal(t, "<nil>", formatf("%s", joinedErr))
	})
}

func TestToStringNestedJoined(t *testing.T) {
	t.Parallel()

	out := ToString(Join(Join(New("a"), New("b")), New("c")))

	assert.Equal(t, "Multiple errors (2):\n\n1. Multiple errors (2):\n\n1. a\n\n2. b\n\n2. c", out)
}

func TestToJSONNestedJoined(t *testing.T) {
	t.Parallel()

	m := ToJSON(Join(Join(New("a"), New("b")), New("c")))

	assert.Equal(t, "joined", m["kind"])
	assert.Equal(t, 2, m["count"])

	errs, ok := m["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errs, 2)

	nested, ok := errs[0].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "joined", nested["kind"])
	assert.Equal(t, 2, nested["count"])
}

func TestJoinedFormattingWithoutCapturedTrace(t *testing.T) {
	t.Parallel()

	j := &joined{errs: []error{New("a"), New("b")}}

	t.Run("string omits join location", func(t *testing.T) {
		t.Parallel()

		out := ToString(j, FormatterWithTrace())

		assert.Contains(t, out, "Multiple errors (2):")
		assert.NotContains(t, out, "Join Location:")
	})

	t.Run("json omits join stack", func(t *testing.T) {
		t.Parallel()

		m := ToJSON(j, FormatterWithTrace())

		assert.NotContains(t, m, "join_stack")
	})
}
