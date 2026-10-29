package format

import (
	"strings"
	"testing"
)

// FuzzSizeDelta checks the size delta rendering: the direction of the change
// must match the sign of the difference, and the sizes must not overflow.
func FuzzSizeDelta(f *testing.F) {
	f.Add(int64(100), int64(100))
	f.Add(int64(100), int64(50))
	f.Add(int64(50), int64(100))

	f.Fuzz(func(t *testing.T, in, out int64) {
		got := SizeDelta(in, out)
		switch {
		case out == in:
			if got != "(unchanged)" {
				t.Fatalf("SizeDelta(%d, %d) = %q, want %q", in, out, got, "(unchanged)")
			}
		case out < in:
			if !strings.Contains(got, "saved") {
				t.Fatalf("SizeDelta(%d, %d) = %q, want a saving", in, out, got)
			}
		case out > in:
			if !strings.Contains(got, "grew") {
				t.Fatalf("SizeDelta(%d, %d) = %q, want growth", in, out, got)
			}
		}
		if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
			t.Fatalf("SizeDelta(%d, %d) = %q, which is not finite", in, out, got)
		}
	})
}
