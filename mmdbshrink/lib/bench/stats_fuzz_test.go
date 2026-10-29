package bench

import (
	"encoding/binary"
	"math"
	"sort"
	"testing"
)

// decodeFloats reads a float64 sample list from a byte slice, eight bytes per
// sample. The native fuzzer only supports byte slices, so the samples are
// encoded rather than passed as []float64.
func decodeFloats(data []byte) []float64 {
	n := len(data) / 8
	out := make([]float64, n)
	for i := range n {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return out
}

// hasNonFinite reports whether any sample is NaN or infinite. Such samples
// make every derived statistic NaN or infinite too, so there is nothing
// meaningful to assert about them.
func hasNonFinite(xs []float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return true
		}
	}
	return false
}

// withinRange reports whether v lies in [min, max] up to a small relative
// tolerance. Accumulating a sum and dividing it rounds, so a mean can land a
// couple of ulps outside the range of its samples.
func withinRange(v, min, max float64) bool {
	if v >= min && v <= max {
		return true
	}
	tol := 1e-9 * math.Max(math.Abs(min), math.Abs(max))
	if tol == 0 {
		tol = math.SmallestNonzeroFloat64
	}
	return v >= min-tol && v <= max+tol
}

// FuzzPercentile checks the quantile helper: it must stay within the range
// of the samples, and it must be monotonic in p.
func FuzzPercentile(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{1, 1, 1, 1, 1, 1, 1, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		xs := decodeFloats(data)
		if len(xs) == 0 {
			if got := percentile(nil, 0.5); !math.IsNaN(got) {
				t.Fatalf("percentile(empty) = %v, want NaN", got)
			}
			return
		}
		if hasNonFinite(xs) {
			return
		}

		sorted := append([]float64(nil), xs...)
		sort.Float64s(sorted)

		min, max := sorted[0], sorted[len(sorted)-1]
		prev := math.Inf(-1)
		for _, p := range []float64{-1, 0, 0.25, 0.5, 0.75, 1, 2} {
			got := percentile(sorted, p)
			if math.IsNaN(got) {
				t.Fatalf("percentile(p=%v) = NaN", p)
			}
			if got < min || got > max {
				t.Fatalf("percentile(p=%v) = %v, outside [%v, %v]", p, got, min, max)
			}
			if got < prev {
				t.Fatalf("percentile(p=%v) = %v, less than the previous %v", p, got, prev)
			}
			prev = got
		}
	})
}

// FuzzSummarize checks the descriptive statistics: the order statistics must
// agree with each other, and the mean must lie within the range.
func FuzzSummarize(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 1, 1, 1, 1, 1, 1, 1})
	f.Add([]byte{2, 2, 2, 2, 2, 2, 2, 2})

	f.Fuzz(func(t *testing.T, data []byte) {
		xs := decodeFloats(data)
		if hasNonFinite(xs) {
			return
		}
		for _, trim := range []float64{0, 0.1, 0.5, 0.9} {
			s := summarize(xs, trim)
			if len(xs) == 0 {
				if s != (Stats{}) {
					t.Fatalf("summarize(empty) = %+v, want zero Stats", s)
				}
				continue
			}

			if s.N != len(xs) {
				t.Fatalf("N = %d, want %d", s.N, len(xs))
			}
			if s.Min > s.Max {
				t.Fatalf("min %v is above max %v", s.Min, s.Max)
			}
			// The percentiles are interpolations, so they can land an ulp
			// outside the range of the samples.
			if !withinRange(s.P05, s.Min, s.Max) {
				t.Fatalf("p05 %v is outside [%v, %v]", s.P05, s.Min, s.Max)
			}
			if !withinRange(s.P95, s.P05, s.Max) {
				t.Fatalf("p95 %v is outside [%v, %v]", s.P95, s.P05, s.Max)
			}
			if !withinRange(s.Median, s.Min, s.Max) {
				t.Fatalf("median %v is outside [%v, %v]", s.Median, s.Min, s.Max)
			}
			// The mean is accumulated as a sum and divided, so it can
			// differ from the exact value by an ulp or two. Compare it
			// with a relative tolerance.
			if !withinRange(s.Mean, s.Min, s.Max) {
				t.Fatalf("mean %v is outside [%v, %v]", s.Mean, s.Min, s.Max)
			}
			if !withinRange(s.Trimmed, s.Min, s.Max) {
				t.Fatalf("trimmed mean %v is outside [%v, %v]", s.Trimmed, s.Min, s.Max)
			}
			if s.Std < 0 {
				t.Fatalf("std %v is negative", s.Std)
			}
		}
	})
}
