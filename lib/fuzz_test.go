package lib

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// FuzzTsvReader checks the reader's contract: every record it returns is the
// tab-split of the corresponding input line with trailing CR/LF removed, and
// reading terminates with io.EOF.
func FuzzTsvReader(f *testing.F) {
	seeds := [][]byte{
		[]byte("a\tb\nc\td\n"),
		[]byte("a\tb"),
		[]byte(""),
		[]byte("\n"),
		[]byte("a\r\n"),
		[]byte("a\t\tb\n"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewTsvReader(bytes.NewReader(data))

		// The expected records are the input split into lines, each with
		// trailing CR/LF stripped (the reader trims all of them) and then
		// split on tabs.
		lines := strings.Split(string(data), "\n")
		want := make([][]string, 0, len(lines))
		for _, line := range lines {
			want = append(want, strings.Split(strings.TrimRight(line, "\r\n"), "\t"))
		}

		for i := 0; ; i++ {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("record %d: unexpected error: %v", i, err)
			}
			if i >= len(want) {
				t.Fatalf("record %d: got more records than the %d lines in the input", i, len(want))
			}
			if !reflect.DeepEqual(rec, want[i]) {
				t.Fatalf("record %d: got %q, want %q", i, rec, want[i])
			}
		}
	})
}

// FuzzTsvWriterReaderRoundTrip checks that a record survives a write/read
// round trip through the TSV writer and reader, for records that contain no
// field separator and no trailing carriage return.
func FuzzTsvWriterReaderRoundTrip(f *testing.F) {
	f.Add("hello")
	f.Add("")
	f.Add("with spaces")

	f.Fuzz(func(t *testing.T, s string) {
		if strings.ContainsAny(s, "\t\r\n") {
			return
		}

		var buf bytes.Buffer
		w := NewTsvWriter(&buf)
		if err := w.Write([]string{s}); err != nil {
			t.Fatalf("write: %v", err)
		}
		w.Flush()
		if err := w.Error(); err != nil {
			t.Fatalf("writer error: %v", err)
		}

		rec, err := NewTsvReader(&buf).Read()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if !reflect.DeepEqual(rec, []string{s}) {
			t.Fatalf("round trip: got %q, want %q", rec, []string{s})
		}
	})
}

// buildFuzzMap turns a byte slice into a map[string]any whose value types
// cover every branch of mapInterfaceToStr. Each triple of bytes is one entry:
// a type tag, a key length, and the key and value bytes that follow.
//
// The native fuzzer only supports scalar argument types, so the bytes stand
// in for a structured input.
func buildFuzzMap(data []byte) map[string]any {
	m := map[string]any{}
	for i := 0; i+2 < len(data); {
		tag := data[i]
		keyLen := int(data[i+1])
		i += 2
		if i+keyLen > len(data) {
			break
		}
		key := string(data[i : i+keyLen])
		i += keyLen
		if i >= len(data) {
			break
		}
		val := data[i]
		i++

		switch tag % 6 {
		case 0:
			m[key] = string([]byte{val})
		case 1:
			m[key] = float64(val) / 3
		case 2:
			m[key] = val%2 == 0
		case 3:
			m[key] = []any{float64(val), "x"}
		case 4:
			m[key] = map[string]any{"nested": float64(val)}
		case 5:
			m[key] = nil
		}
	}
	return m
}

// FuzzMapInterfaceToStr checks the record-to-string conversion used by the
// read and export commands: strings and numbers convert predictably, nested
// values become JSON, and the conversion never panics.
func FuzzMapInterfaceToStr(f *testing.F) {
	f.Add([]byte{0, 1, 'a', 'b'})
	f.Add([]byte{1, 1, 'a', 3})
	f.Add([]byte{3, 1, 'a', 1, 2, 'x'})

	f.Fuzz(func(t *testing.T, data []byte) {
		m := buildFuzzMap(data)

		got := mapInterfaceToStr(m)
		if got == nil {
			t.Fatalf("mapInterfaceToStr returned nil for %v", m)
		}
		if len(got) != len(m) {
			t.Fatalf("got %d entries for %d keys", len(got), len(m))
		}
		for k, v := range m {
			gotv, ok := got[k]
			if !ok {
				t.Fatalf("key %q missing from the result", k)
			}
			switch val := v.(type) {
			case string:
				if gotv != val {
					t.Fatalf("key %q: got %q, want %q", k, gotv, val)
				}
			case float64:
				if gotv != strconv.FormatFloat(val, 'f', 6, 64) {
					t.Fatalf("key %q: got %q, want the float's %f form", k, gotv, val)
				}
			default:
				// Everything else is JSON-encoded.
				want, err := json.Marshal(val)
				if err != nil {
					t.Fatalf("key %q: marshal: %v", k, err)
				}
				if gotv != string(want) {
					t.Fatalf("key %q: got %q, want %q", k, gotv, string(want))
				}
			}
		}
	})
}

// FuzzConvertToMMDBType checks that every JSON-shaped value converts to an
// MMDB value without panicking, and that the conversion is faithful for the
// scalar types.
func FuzzConvertToMMDBType(f *testing.F) {
	f.Add([]byte(`"s"`))
	f.Add([]byte(`1`))
	f.Add([]byte(`true`))
	f.Add([]byte(`["a",2]`))
	f.Add([]byte(`{"k":"v"}`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return
		}

		got, err := ConvertToMMDBType(v)
		if err != nil {
			t.Fatalf("ConvertToMMDBType(%#v): %v", v, err)
		}
		if got == nil {
			t.Fatalf("ConvertToMMDBType(%#v) returned a nil value", v)
		}
	})
}

// FuzzTraverseDataSection walks the data section of an arbitrary file with
// the same code path the metadata command uses, checking that it never
// panics and never reports negative sizes.
func FuzzTraverseDataSection(f *testing.F) {
	f.Add([]byte{0x40, 'a'}, int64(0), int64(2))
	f.Add([]byte{0x00, 0x01}, int64(0), int64(2))
	f.Add([]byte{}, int64(0), int64(0))

	f.Fuzz(func(t *testing.T, data []byte, start, end int64) {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.mmdb")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		// Keep the offsets inside the file so the walker actually reads
		// bytes; out-of-range offsets just produce an error.
		n := int64(len(data))
		start = start % (n + 1)
		end = start + end%(n+1)

		sizes, err := traverseDataSection(path, start, end)
		if err != nil {
			return
		}
		if sizes.PointerSize < 0 || sizes.Utf8StringSize < 0 || sizes.DoubleSize < 0 ||
			sizes.BytesSize < 0 || sizes.Unsigned16bitIntSize < 0 || sizes.Unsigned32bitIntSize < 0 ||
			sizes.Signed32bitIntSize < 0 || sizes.Unsigned64bitIntSize < 0 ||
			sizes.Unsigned128bitIntSize < 0 || sizes.MapKeyValueCount < 0 ||
			sizes.ArrayLength < 0 || sizes.FloatSize < 0 {
			t.Fatalf("negative type sizes: %+v", sizes)
		}
	})
}
