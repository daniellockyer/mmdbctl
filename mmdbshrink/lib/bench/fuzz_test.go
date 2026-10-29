package bench

import (
	"testing"
)

// FuzzVerdict checks that the verdict classifier agrees with the sign of the
// confidence interval.
func FuzzVerdict(f *testing.F) {
	f.Add(-1.0, 1.0)
	f.Add(1.0, -1.0)
	f.Add(0.0, 0.0)

	f.Fuzz(func(t *testing.T, lo, hi float64) {
		got := verdict(lo, hi)
		switch got {
		case VerdictFaster, VerdictSlower, VerdictNoise:
		default:
			t.Fatalf("verdict(%v, %v) = %q, which is not a known verdict", lo, hi, got)
		}

		// A strictly negative interval is an improvement, a strictly
		// positive one a regression, and anything straddling zero is
		// noise.
		switch {
		case hi < 0:
			if got != VerdictFaster {
				t.Fatalf("verdict(%v, %v) = %q, want %q", lo, hi, got, VerdictFaster)
			}
		case lo > 0:
			if got != VerdictSlower {
				t.Fatalf("verdict(%v, %v) = %q, want %q", lo, hi, got, VerdictSlower)
			}
		default:
			if got != VerdictNoise {
				t.Fatalf("verdict(%v, %v) = %q, want %q", lo, hi, got, VerdictNoise)
			}
		}
	})
}
