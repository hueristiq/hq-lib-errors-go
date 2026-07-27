package errors

import "testing"

// BenchmarkNew measures the cost of creating a root error, including stack
// capture. Capture must stay cheap (no symbolization) so New remains viable on
// hot paths; see callers and stack.resolveToStackFrames for the lazy design.
func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = New("benchmark error")
	}
}

// BenchmarkWrap measures the cost of wrapping a package error (single-frame
// capture).
func BenchmarkWrap(b *testing.B) {
	base := New("base")

	b.ReportAllocs()

	for b.Loop() {
		_ = Wrap(base, "context")
	}
}

// BenchmarkJoin measures the cost of joining two errors, including stack
// capture at the join point.
func BenchmarkJoin(b *testing.B) {
	err1 := New("one")
	err2 := New("two")

	b.ReportAllocs()

	for b.Loop() {
		_ = Join(err1, err2)
	}
}

// BenchmarkToStringWithTrace measures the lazy stack-resolution path: PCs
// captured by New/Wrap are symbolized only when a trace is rendered.
func BenchmarkToStringWithTrace(b *testing.B) {
	err := Wrap(New("base"), "context")

	b.ReportAllocs()

	for b.Loop() {
		_ = ToString(err, FormatWithTrace())
	}
}
