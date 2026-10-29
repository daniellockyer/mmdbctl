package shrink

import (
	"reflect"
	"testing"
)

// FuzzFindMetadata checks that the metadata marker scan never panics and
// always reports the last marker occurrence.
func FuzzFindMetadata(f *testing.F) {
	seeds := [][]byte{
		append([]byte("data"), metadataStartMarker...),
		append(append([]byte("a"), metadataStartMarker...), metadataStartMarker...),
		{},
		[]byte("no marker here"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		start, metaBytes, err := findMetadata(data)
		if err != nil {
			return
		}
		if start > uint64(len(data)) {
			t.Fatalf("start %d is past the end of the %d-byte buffer", start, len(data))
		}
		if got := data[start+uint64(len(metadataStartMarker)):]; !reflect.DeepEqual(got, metaBytes) {
			t.Fatalf("metaBytes does not match the buffer contents after the marker")
		}
		// The scan takes the LAST occurrence, so the reported offset must be
		// the last one in the buffer.
		if last := lastIndexOfMarker(data); last >= 0 && uint64(last) != start {
			t.Fatalf("found marker at %d, want the last occurrence at %d", start, last)
		}
	})
}

// lastIndexOfMarker returns the index of the last metadata marker in data, or
// -1 if there is none.
func lastIndexOfMarker(data []byte) int {
	for i := len(data) - len(metadataStartMarker); i >= 0; i-- {
		if string(data[i:i+len(metadataStartMarker)]) == string(metadataStartMarker) {
			return i
		}
	}
	return -1
}

// FuzzValueSpan checks that walking an MMDB value at an arbitrary offset in
// an arbitrary buffer never panics, and that a successful walk stays inside
// the buffer.
func FuzzValueSpan(f *testing.F) {
	seeds := [][]byte{
		{0x42, 'h', 'i'},
		{0x20, 0x03},
		{0xE1, 0x41, 'a', 0x42, 'b'},
		{0x02, 0x04, 0x41, 'z'},
		{},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, off := range []uint32{0, 1, 2} {
			end, ptrs, err := valueSpan(data, off)
			if err != nil {
				continue
			}
			if end > uint32(len(data)) {
				t.Fatalf("valueSpan returned end %d, past the %d-byte buffer", end, len(data))
			}
			if end < off {
				t.Fatalf("valueSpan returned end %d, before the start offset %d", end, off)
			}
			for _, p := range ptrs {
				if p.width < 2 || p.width > 5 {
					t.Fatalf("pointer width %d is outside the MMDB's 2..5 range", p.width)
				}
				if uint64(p.offset)+uint64(p.width) > uint64(len(data)) {
					t.Fatalf("pointer at %d width %d overruns the %d-byte buffer", p.offset, p.width, len(data))
				}
			}
		}
	})
}

// FuzzReadSizeAt checks the length-prefix decoder against the MMDB spec for
// all four size classes.
func FuzzReadSizeAt(f *testing.F) {
	seeds := [][]byte{
		{},
		{0x00},
		{0x01, 0x02},
		{0x01, 0x02, 0x03},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, sizeBits := range []int{0, 28, 29, 30, 31} {
			n, next, err := readSizeAt(data, sizeBits, 0)
			if err != nil {
				continue
			}
			if next > uint32(len(data)) {
				t.Fatalf("next offset %d is past the %d-byte buffer", next, len(data))
			}
			switch sizeBits {
			case 29:
				if want := 29 + uint32(data[0]); n != want {
					t.Fatalf("size 29: got %d, want %d", n, want)
				}
			case 30:
				if want := 285 + uint32(data[0])<<8 + uint32(data[1]); n != want {
					t.Fatalf("size 30: got %d, want %d", n, want)
				}
			case 31:
				if want := 65821 + uint32(data[0])<<16 + uint32(data[1])<<8 + uint32(data[2]); n != want {
					t.Fatalf("size 31: got %d, want %d", n, want)
				}
			default:
				if n != uint32(sizeBits) {
					t.Fatalf("size %d: got %d, want it unchanged", sizeBits, n)
				}
			}
		}
	})
}

// FuzzReadPointerAt checks that pointer decoding never panics and stays
// inside the buffer.
func FuzzReadPointerAt(f *testing.F) {
	seeds := [][]byte{
		{},
		{0x01},
		{0x01, 0x02},
		{0x01, 0x02, 0x03},
		{0x01, 0x02, 0x03, 0x04},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for sizeBits := 0; sizeBits < 32; sizeBits++ {
			target, next, err := readPointerAt(data, sizeBits, 0)
			if err != nil {
				continue
			}
			if next > uint32(len(data)) {
				t.Fatalf("next offset %d is past the %d-byte buffer", next, len(data))
			}
			_ = target
		}
	})
}

// FuzzCanonicalize checks that canonicalizing an arbitrary search tree never
// panics and never loops forever. The node count is derived from the buffer
// so the target stays proportional to the input.
func FuzzCanonicalize(f *testing.F) {
	seeds := [][]byte{
		{},
		{0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 1},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, recordSize := range []uint64{24, 28, 32} {
			nodeBytes := recordSize / 4
			nodeCount := uint32(len(data) / int(nodeBytes))
			if nodeCount == 0 {
				continue
			}
			tree := data[:uint64(nodeCount)*nodeBytes]

			canon, err := canonicalize(testLogger, tree, nodeCount, recordSize)
			if err != nil {
				continue
			}
			if len(canon) == 0 {
				t.Fatalf("canonicalize returned no nodes")
			}
			// The canonical tree must be emittable: the tree writer has to
			// produce exactly one node's worth of bytes per canonical node.
			out, err := buildSearchTree(canon, recordSize)
			if err != nil {
				t.Fatalf("buildSearchTree of the canonicalized tree: %v", err)
			}
			if uint64(len(out)) != uint64(len(canon))*nodeBytes {
				t.Fatalf("tree size %d does not match %d nodes of %d bytes",
					len(out), len(canon), nodeBytes)
			}
		}
	})
}

// FuzzMinWidthForOffset checks that the pointer width chosen for a target
// offset is the smallest one that can actually hold it.
func FuzzMinWidthForOffset(f *testing.F) {
	f.Add(uint32(0))
	f.Add(uint32(2047))
	f.Add(uint32(2048))

	f.Fuzz(func(t *testing.T, target uint32) {
		w := minWidthForOffset(target)
		if w < 2 || w > 5 {
			t.Fatalf("minWidthForOffset(%d) = %d, outside the 2..5 range", target, w)
		}

		// The chosen width must encode the target.
		buf := make([]byte, w)
		if err := writePointer(buf, target, w); err != nil {
			t.Fatalf("writePointer(%d, %d): %v", target, w, err)
		}
		end, ptrs, err := valueSpan(buf, 0)
		if err != nil {
			t.Fatalf("valueSpan of the encoded pointer: %v", err)
		}
		if end != uint32(w) {
			t.Fatalf("encoded pointer spans %d bytes, want %d", end, w)
		}
		if len(ptrs) != 1 || ptrs[0].target != target {
			t.Fatalf("decoded pointer = %+v, want target %d", ptrs, target)
		}

		// A narrower width must not be able to hold the target, or the
		// chosen width was not minimal.
		if w > 2 {
			smaller := make([]byte, w-1)
			if err := writePointer(smaller, target, w-1); err == nil {
				t.Fatalf("target %d also fits in %d bytes, but %d was chosen",
					target, w-1, w)
			}
		}
	})
}
