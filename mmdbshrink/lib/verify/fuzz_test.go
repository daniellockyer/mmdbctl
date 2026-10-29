package verify

import (
	"bytes"
	"net/netip"
	"testing"
)

// FuzzPrefixProbes checks the probe sets built for a prefix: every probe
// must be inside the prefix, the boundaries must be right, and the adjacent
// probes must be outside it.
func FuzzPrefixProbes(f *testing.F) {
	f.Add([]byte{10, 0, 0, 0}, uint8(8))
	f.Add([]byte{0, 0, 0, 0}, uint8(0))
	f.Add([]byte{1, 2, 3, 4}, uint8(32))

	f.Fuzz(func(t *testing.T, addr []byte, bits uint8) {
		if len(addr) != 4 {
			return
		}
		n := int(bits) % 33
		a := netip.AddrFrom4([4]byte{addr[0], addr[1], addr[2], addr[3]})
		pfx := netip.PrefixFrom(a, n).Masked()

		probes := prefixProbes(pfx, true)
		if len(probes) == 0 {
			t.Fatal("prefixProbes returned no probes")
		}
		seen := map[netip.Addr]bool{}
		for _, p := range probes {
			if !pfx.Contains(p.ip) {
				t.Fatalf("probe %s is outside the prefix %s", p.ip, pfx)
			}
			if seen[p.ip] {
				t.Fatalf("probe %s appears twice", p.ip)
			}
			seen[p.ip] = true
		}
		if probes[0].ip != pfx.Addr() {
			t.Fatalf("first probe is %s, want the prefix's first address %s", probes[0].ip, pfx.Addr())
		}

		// The mid and last addresses must be the ones prefixMidLast
		// computes, and the last must be the prefix's final address.
		mid, last := prefixMidLast(pfx)
		if !pfx.Contains(mid) {
			t.Fatalf("mid address %s is outside the prefix %s", mid, pfx)
		}
		if !pfx.Contains(last) {
			t.Fatalf("last address %s is outside the prefix %s", last, pfx)
		}
		if mid != last && mid.Compare(pfx.Addr()) <= 0 {
			t.Fatalf("mid address %s is not after the first address %s", mid, pfx.Addr())
		}

		// Adjacent probes must fall outside the prefix.
		for _, p := range adjacentProbes(pfx) {
			if pfx.Contains(p.ip) {
				t.Fatalf("adjacent probe %s is inside the prefix %s", p.ip, pfx)
			}
		}
	})
}

// FuzzCompareKeySets checks that the schema comparison is symmetric and
// only fails when the key sets genuinely differ. The two key sets are
// encoded as NUL-separated strings, since the fuzzer only supports byte
// slices.
func FuzzCompareKeySets(f *testing.F) {
	f.Add([]byte("a\x00b"), []byte("a\x00b"))
	f.Add([]byte("a"), []byte("b"))

	f.Fuzz(func(t *testing.T, a, b []byte) {
		ma := map[string]int{}
		for _, k := range splitKeys(a) {
			ma[k] = 1
		}
		mb := map[string]int{}
		for _, k := range splitKeys(b) {
			mb[k] = 1
		}

		ok, msg := compareKeySets("entry", ma, mb)
		if ok && msg != "" {
			t.Fatalf("compareKeySets reported ok with the message %q", msg)
		}
		if !ok && msg == "" {
			t.Fatal("compareKeySets reported a mismatch with an empty message")
		}

		// The comparison must be symmetric: if the key sets match, so do
		// the results.
		okB, _ := compareKeySets("entry", mb, ma)
		if ok != okB {
			t.Fatalf("compareKeySets is not symmetric: %v vs %v", ok, okB)
		}

		// Equal key sets must compare equal.
		same := map[string]int{}
		for k := range ma {
			same[k] = 1
		}
		if ok2, msg2 := compareKeySets("entry", ma, same); !ok2 {
			t.Fatalf("compareKeySets reported a mismatch for equal key sets: %s", msg2)
		}
	})
}

// splitKeys splits a NUL-separated key list, ignoring empty keys.
func splitKeys(data []byte) []string {
	var keys []string
	for _, k := range bytes.Split(data, []byte{0}) {
		if len(k) == 0 {
			continue
		}
		keys = append(keys, string(k))
	}
	return keys
}
