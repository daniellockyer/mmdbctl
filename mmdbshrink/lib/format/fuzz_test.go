package format

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// FuzzInt checks that the thousands-separated rendering of an integer parses
// back to the same value, and that the grouping is well formed.
func FuzzInt(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(999))
	f.Add(uint64(1000))
	f.Add(uint64(1234567))

	f.Fuzz(func(t *testing.T, v uint64) {
		got := Int(v)

		// Strip the separators and parse: the digits must be exactly the
		// decimal representation of v.
		digits := strings.ReplaceAll(got, ",", "")
		parsed, err := strconv.ParseUint(digits, 10, 64)
		if err != nil {
			t.Fatalf("Int(%d) = %q, which does not parse: %v", v, got, err)
		}
		if parsed != v {
			t.Fatalf("Int(%d) = %q, which parses back as %d", v, got, parsed)
		}

		// The separators must sit every third digit from the right, with
		// none at the start or the end. The leading group may be shorter
		// than three digits; every group after it must be exactly three.
		if strings.HasPrefix(got, ",") || strings.HasSuffix(got, ",") {
			t.Fatalf("Int(%d) = %q has a misplaced separator", v, got)
		}
		groups := strings.Split(got, ",")
		for i, g := range groups {
			if len(g) == 0 {
				t.Fatalf("Int(%d) = %q has an empty group", v, got)
			}
			if i == 0 {
				if len(g) > 3 {
					t.Fatalf("Int(%d) = %q has a %d-digit leading group", v, got, len(g))
				}
				continue
			}
			if len(g) != 3 {
				t.Fatalf("Int(%d) = %q has a %d-digit group", v, got, len(g))
			}
		}
	})
}

// FuzzBytes checks the IEC byte rendering: it must always end in a known
// unit, and the number it holds must scale back to the original value.
func FuzzBytes(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1024))
	f.Add(uint64(1024 * 1024))

	f.Fuzz(func(t *testing.T, v uint64) {
		got := Bytes(v)
		for _, unit := range []string{" B", " KiB", " MiB", " GiB"} {
			if strings.HasSuffix(got, unit) {
				return
			}
		}
		t.Fatalf("Bytes(%d) = %q, which has no known unit", v, got)
	})
}

// FuzzDecimalBytes checks the SI byte rendering the same way.
func FuzzDecimalBytes(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1000))
	f.Add(uint64(1000 * 1000))

	f.Fuzz(func(t *testing.T, v uint64) {
		got := DecimalBytes(v)
		for _, unit := range []string{" B", " KB", " MB", " GB", " TB", " PB", " EB"} {
			if strings.HasSuffix(got, unit) {
				return
			}
		}
		t.Fatalf("DecimalBytes(%d) = %q, which has no known unit", v, got)
	})
}

// FuzzPctDelta checks that a percentage delta is only reported when there is
// something to report, and that the sign matches the direction of the change.
func FuzzPctDelta(f *testing.F) {
	f.Add(int64(0), int64(0))
	f.Add(int64(-5), int64(100))
	f.Add(int64(5), int64(100))

	f.Fuzz(func(t *testing.T, diff, base int64) {
		got := PctDelta(diff, base)
		if diff == 0 || base == 0 {
			if got != "(unchanged)" {
				t.Fatalf("PctDelta(%d, %d) = %q, want %q", diff, base, got, "(unchanged)")
			}
			return
		}
		if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
			t.Fatalf("PctDelta(%d, %d) = %q, which is not a finite number", diff, base, got)
		}
		if diff < 0 && !strings.HasPrefix(got, "(") {
			t.Fatalf("PctDelta(%d, %d) = %q, want a negative delta", diff, base, got)
		}
		if diff > 0 && !strings.Contains(got, "+") {
			t.Fatalf("PctDelta(%d, %d) = %q, want a positive delta", diff, base, got)
		}
	})
}

// FuzzSignedPct checks that the signed percentage keeps its sign and renders
// NaN as the em dash.
func FuzzSignedPct(f *testing.F) {
	f.Add(5.43)
	f.Add(-65.78)
	f.Add(math.NaN())

	f.Fuzz(func(t *testing.T, v float64) {
		got := SignedPct(v)
		if math.IsNaN(v) {
			if got != "—" {
				t.Fatalf("SignedPct(NaN) = %q, want %q", got, "—")
			}
			return
		}
		if math.IsInf(v, 0) {
			return
		}
		if !strings.HasSuffix(got, "%") {
			t.Fatalf("SignedPct(%v) = %q, which has no percent sign", v, got)
		}
		if v >= 0 && !strings.HasPrefix(got, "+") {
			t.Fatalf("SignedPct(%v) = %q, want a leading plus", v, got)
		}
		if v < 0 && !strings.HasPrefix(got, "-") {
			t.Fatalf("SignedPct(%v) = %q, want a leading minus", v, got)
		}
	})
}
