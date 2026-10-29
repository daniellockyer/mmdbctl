package shrink

import (
	"testing"
)

// FuzzCompactDataSection checks that compacting an arbitrary data section
// never panics, and that the resulting offsets all point at parseable values
// inside the compacted section.
func FuzzCompactDataSection(f *testing.F) {
	seeds := [][]byte{
		{0x42, 'h', 'i'},
		{0x42, 'h', 'i', 0x45, 'w', 'o', 'r', 'l', 'd'},
		{},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		// Try a handful of roots spread across the section.
		var roots []uint32
		for off := uint32(0); off < uint32(len(data)) && len(roots) < 8; off += uint32(len(data)) / 8 {
			roots = append(roots, off)
		}
		if len(roots) == 0 {
			roots = []uint32{0}
		}

		res, err := compactDataSection(testLogger, data, roots)
		if err != nil {
			return
		}
		for old, new := range res.offsetMap {
			if new >= uint32(len(res.bytes)) {
				t.Fatalf("offsetMap[%d] = %d is past the %d-byte compacted section",
					old, new, len(res.bytes))
			}
			end, _, err := valueSpan(res.bytes, new)
			if err != nil {
				t.Fatalf("valueSpan at the remapped offset %d (was %d): %v", new, old, err)
			}
			if end > uint32(len(res.bytes)) {
				t.Fatalf("valueSpan at %d ended at %d, past the %d-byte section",
					new, end, len(res.bytes))
			}
		}
	})
}
