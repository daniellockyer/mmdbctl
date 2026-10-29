package lib

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzCmdImportTSV drives the tab-separated import path with arbitrary bytes.
// Unlike the CSV branch, the TSV reader does not enforce a consistent field
// count, so rows shorter than the header are the interesting input.
func FuzzCmdImportTSV(f *testing.F) {
	redirectStderr(f)

	seeds := [][]byte{
		[]byte("network\tcountry\n1.2.3.0/24\tUS\n"),
		[]byte("network\tcountry\n1.2.3.0/24\n"),
		[]byte("start_ip\tend_ip\tcountry\n1.2.3.4\t1.2.3.255\tUS\n"),
		[]byte("start_ip\tend_ip\tcountry\n1.2.3.4\tUS\n"),
		[]byte("network\tcountry\n"),
		[]byte(""),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		in := filepath.Join(dir, "in.tsv")
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
			Tsv:   true,
		}, []string{}, func() {})
	})
}
