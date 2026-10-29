package verify

import (
	"bytes"
	"testing"

	"github.com/oschwald/maxminddb-golang/v2/mmdbdata"
)

// FuzzCanonicalRecord checks that the canonical encoding of an arbitrary
// MMDB value never panics, and that the same value always canonicalizes to
// the same bytes: the canonical form is what verify compares records by, so
// it must be deterministic.
func FuzzCanonicalRecord(f *testing.F) {
	seeds := [][]byte{
		mmdbCtrl(7, 2),
		mmdbString("a"),
		mmdbUint32(7),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var rec canonicalRecord
		err := rec.UnmarshalMaxMindDB(mmdbdata.NewDecoder(data, 0))
		if err != nil {
			return
		}

		// Decoding the same bytes again must produce the same canonical
		// form.
		var again canonicalRecord
		if err := again.UnmarshalMaxMindDB(mmdbdata.NewDecoder(data, 0)); err != nil {
			t.Fatalf("second canonical decode failed: %v", err)
		}
		if !bytes.Equal(rec.data, again.data) {
			t.Fatalf("canonical encoding is not deterministic:\n%x\n%x", rec.data, again.data)
		}
	})
}
