package scores

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"math"
	"slices"
	"testing"
)

func encodeBreakdown(t testing.TB, digits []byte, declared uint32) string {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, declared)
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(digits); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// FuzzDecodeBreakdown: arbitrary text never panics, and a success is a bounded digit sequence.
func FuzzDecodeBreakdown(f *testing.F) {
	f.Add("")
	f.Add("AAAAAA==")
	f.Add(encodeBreakdown(f, []byte("0012345"), 7))
	f.Add(encodeBreakdown(f, []byte("0012345"), 6))
	f.Add(encodeBreakdown(f, []byte("9"), 1))
	f.Fuzz(func(t *testing.T, s string) {
		out, err := DecodeBreakdown(s)
		if err != nil {
			if out != nil {
				t.Fatal("output with an error")
			}
			return
		}
		if len(out) == 0 || len(out) > maxBreakdown {
			t.Fatalf("length %d", len(out))
		}
		for _, d := range out {
			if d > 5 {
				t.Fatalf("digit %d", d)
			}
		}
	})
}

// FuzzBreakdownRoundTrip: a well-formed encoding of judgment digits decodes to the same digits,
// and a wrong declared length is refused.
func FuzzBreakdownRoundTrip(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5}, uint32(0))
	f.Add([]byte{5}, uint32(1))
	f.Fuzz(func(t *testing.T, seq []byte, delta uint32) {
		if len(seq) == 0 || len(seq) > 5000 {
			return
		}
		digits := make([]byte, len(seq))
		for i, d := range seq {
			seq[i] = d % 6
			digits[i] = '0' + seq[i]
		}
		got, err := DecodeBreakdown(encodeBreakdown(t, digits, uint32(len(seq))))
		if err != nil || !bytes.Equal(got, seq) {
			t.Fatalf("round trip of %v: %v, %v", seq, got, err)
		}
		if delta != 0 {
			if _, err := DecodeBreakdown(encodeBreakdown(t, digits, uint32(len(seq))+delta)); err == nil {
				t.Fatalf("declared length %d+%d accepted", len(seq), delta)
			}
		}
	})
}

// FuzzBuildTimeline checks the shape of a timeline from any digit sequence.
func FuzzBuildTimeline(f *testing.F) {
	f.Add([]byte{0, 5, 5, 1})
	f.Add(bytes.Repeat([]byte{5}, 1200))
	f.Add(bytes.Repeat([]byte{0, 1, 2}, 333))
	f.Fuzz(func(t *testing.T, seq []byte) {
		for i := range seq {
			seq[i] %= 6
		}
		tl := BuildTimeline(7, seq)
		n := len(seq)
		if tl.Total != n || len(tl.Points) != min(n, maxPoints) || len(tl.Counts) != len(tl.Points) {
			t.Fatalf("shape total=%d points=%d counts=%d for n=%d", tl.Total, len(tl.Points), len(tl.Counts), n)
		}
		sum := 0
		var perDigit [6]int
		for _, c := range tl.Counts {
			for d, k := range c {
				sum += k
				perDigit[d] += k
			}
		}
		if sum != n {
			t.Fatalf("counts sum %d, want %d", sum, n)
		}
		var want [6]int
		misses := 0
		acc := 0.0
		for _, d := range seq {
			want[d]++
			acc += weights[d]
			if d == missDigit {
				misses++
			}
		}
		if perDigit != want {
			t.Fatalf("per digit %v, want %v", perDigit, want)
		}
		for _, p := range tl.Points {
			if p < 0 || p > 100 || math.IsNaN(p) {
				t.Fatalf("point %v", p)
			}
		}
		if n > 0 {
			if last, exact := tl.Points[len(tl.Points)-1], math.Round(math.Max(acc/float64(n), 0)*100)/100; last != exact {
				t.Fatalf("final point %v, want %v", last, exact)
			}
		}
		if len(tl.Misses) != min(misses, maxMisses) || !slices.IsSorted(tl.Misses) {
			t.Fatalf("misses %d (sorted=%v), want %d", len(tl.Misses), slices.IsSorted(tl.Misses), min(misses, maxMisses))
		}
		for i, m := range tl.Misses {
			if seq[m] != missDigit || (i > 0 && tl.Misses[i-1] == m) {
				t.Fatalf("miss index %d invalid", m)
			}
		}
	})
}

// FuzzDecodeMods: names are unique known mods, the rate is one of the table rates, and the PB key ignores
// exactly the ignored mods.
func FuzzDecodeMods(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1<<19 | 1<<20 | 1<<6))
	f.Add(int64(-1))
	f.Fuzz(func(t *testing.T, mods int64) {
		rate, names := decodeMods(mods)
		seen := map[string]bool{}
		for _, n := range names {
			if seen[n] {
				t.Fatalf("duplicate %q", n)
			}
			seen[n] = true
		}
		if len(names) > len(modFlags) {
			t.Fatalf("%d names", len(names))
		}
		ok := rate == 1.0
		for _, s := range speedFlags {
			ok = ok || s.rate == rate
		}
		if !ok {
			t.Fatalf("rate %v", rate)
		}
		if (mods&keyMask)&(1<<18|1<<21) != 0 || (mods&keyMask)&disqualifyMask != 0 {
			t.Fatalf("key mask keeps an ignored or disqualifying bit: %b", mods&keyMask)
		}
		if mods&^(1<<18|1<<21|1<<19|1<<20) != mods&keyMask {
			t.Fatalf("key mask drops a counting bit: %b vs %b", mods&keyMask, mods)
		}
	})
}
