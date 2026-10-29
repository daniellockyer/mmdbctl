package verify

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

// netipMustParseAddr parses an address that is known to be valid.
func netipMustParseAddr(s string) netip.Addr {
	return netip.MustParseAddr(s)
}

// selfPointerMMDB builds a minimal MMDB whose data section holds a
// self-referential pointer: the value at data offset 0 is a class-0 pointer
// whose target is data offset 0.
func selfPointerMMDB() []byte {
	const (
		nodeCount  = 1
		recordSize = 24
		dataOffset = nodeCount + 16
	)
	tree := []byte{0, 0, dataOffset, 0, 0, dataOffset}
	// Data section: ctrl 0x20 = pointer kind, class 0, payload byte 0x00
	// -> points at data offset 0, i.e. itself.
	data := []byte{0x20, 0x00}
	metadata := mmdbMap(map[string][]byte{
		"binary_format_major_version": mmdbUint16(2),
		"binary_format_minor_version": mmdbUint16(0),
		"build_epoch":                 mmdbUint64(0),
		"database_type":               mmdbString("Self-Pointer"),
		"description":                 mmdbMap(map[string][]byte{"en": mmdbString("test")}),
		"ip_version":                  mmdbUint16(4),
		"languages":                   mmdbArray(mmdbString("en")),
		"node_count":                  mmdbUint32(nodeCount),
		"record_size":                 mmdbUint16(recordSize),
	})
	return bytes.Join([][]byte{
		tree,
		make([]byte, 16),
		data,
		append([]byte{0xab, 0xcd, 0xef}, []byte("MaxMind.com")...),
		metadata,
	}, nil)
}

// TestCanonicalRecordRejectsSelfPointer is the regression test for the
// infinite loop in the canonical decoder.
//
// The canonical record implements the Unmarshaler interface, which bypasses
// the reader's own depth guard, so a self-referential pointer in the data
// section reached the decoder's pointer resolution and it looped forever
// chasing the pointer. The file is malformed, and every other decode path
// rejects it, so the canonical decoder must too — with an error, not a hang.
func TestCanonicalRecordRejectsSelfPointer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "self.mmdb")
	if err := os.WriteFile(path, selfPointerMMDB(), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	done := make(chan error, 1)
	go func() {
		var rec canonicalRecord
		done <- db.Lookup(netipMustParseAddr("1.2.3.4")).Decode(&rec)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error decoding a self-referential pointer")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("canonical decode hung on a self-referential pointer")
	}
}
