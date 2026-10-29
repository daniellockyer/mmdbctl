package verify

import (
	"testing"
	"time"

	"github.com/oschwald/maxminddb-golang/v2/mmdbdata"
)

// TestCanonicalRecordRejectsCyclicContainers is the regression test for the
// unbounded recursion in the canonical decoder.
//
// A record whose containers point back at each other — here an array whose
// single element is a pointer to the array itself — sends the decoder into
// an endless loop. The upstream reader bounds its own decoder's nesting
// depth, but this decoder implements the Unmarshaler interface and so
// bypasses that bound, so it needs one of its own.
func TestCanonicalRecordRejectsCyclicContainers(t *testing.T) {
	// An array (kind 11) of one element, where the element is a class-0
	// pointer back to the array's own offset.
	//
	//   offset 0: 0x02        extended kind, size 2
	//   offset 1: 0x04        extended kind 11 (array)
	//   offset 2: 0x20        pointer, class 0
	//   offset 3: 0x00        pointer payload -> target offset 0
	cyclic := []byte{0x02, 0x04, 0x20, 0x00}

	done := make(chan error, 1)
	go func() {
		var rec canonicalRecord
		done <- rec.UnmarshalMaxMindDB(mmdbdata.NewDecoder(cyclic, 0))
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error decoding a cyclic record")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("canonical decode hung on a cyclic record")
	}
}
