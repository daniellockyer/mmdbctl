package lib

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzCmdImportCSV drives the comma-separated import path with arbitrary
// bytes.
func FuzzCmdImportCSV(f *testing.F) {
	redirectStderr(f)

	seeds := [][]byte{
		[]byte("network,country\n1.2.3.0/24,US\n"),
		[]byte("network,country\n1.2.3.0/24\n"),
		[]byte("start_ip,end_ip,country\n1.2.3.4,1.2.3.255,US\n"),
		[]byte("\"quoted,field\",x\n"),
		[]byte(""),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		in := filepath.Join(dir, "in.csv")
		out := filepath.Join(dir, "out.mmdb")
		if err := os.WriteFile(in, data, 0o644); err != nil {
			t.Fatalf("write input: %v", err)
		}

		_ = CmdImport(CmdImportFlags{
			Ip:    6,
			Size:  32,
			Merge: "none",
			In:    in,
			Out:   out,
			Csv:   true,
		}, []string{}, func() {})
	})
}
