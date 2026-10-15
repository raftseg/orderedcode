// Copyright 2026 The raftseg authors. Licensed under the Apache License,
// Version 2.0.

package orderedcode

// Property tests for the encoding's promises, over generated tuples of every
// item type in both directions:
//   - round trip: Parse returns what Append encoded, and nothing remains;
//   - prefix: the encoding of a tuple's first items is a prefix of the
//     tuple's encoding;
//   - order: encodings compare as the tuples do, item by item;
//   - canonical: whatever Parse accepts, Append writes back byte for byte,
//     so no value has two encodings that parse.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
)

type kind int

const (
	kString kind = iota
	kInfinity
	kStringOrInfinity
	kFloat
	kInt
	kUint
	kTrailing // last item only
	numKinds
)

var kindNames = [...]string{"string", "struct{}", "StringOrInfinity", "float64", "int64", "uint64", "TrailingString"}

func (k kind) String() string { return kindNames[k] }

type value struct {
	k    kind
	decr bool
	s    string
	inf  bool
	f    float64
	i    int64
	u    uint64
}

func (v value) String() string {
	d := ""
	if v.decr {
		d = "Decr "
	}
	switch v.k {
	case kString, kTrailing:
		return fmt.Sprintf("%s%v(%q)", d, v.k, v.s)
	case kInfinity:
		return d + "Infinity"
	case kStringOrInfinity:
		if v.inf {
			return d + "StringOrInfinity(Infinity)"
		}
		return fmt.Sprintf("%sStringOrInfinity(%q)", d, v.s)
	case kFloat:
		return fmt.Sprintf("%sfloat64(%v, %#x)", d, v.f, math.Float64bits(v.f))
	case kInt:
		return fmt.Sprintf("%sint64(%d)", d, v.i)
	default:
		return fmt.Sprintf("%suint64(%d)", d, v.u)
	}
}

// item returns the value as Append takes it.
func (v value) item() any {
	var x any
	switch v.k {
	case kString:
		x = v.s
	case kInfinity:
		x = Infinity
	case kStringOrInfinity:
		x = StringOrInfinity{String: v.s, Infinity: v.inf}
	case kFloat:
		x = v.f
	case kInt:
		x = v.i
	case kUint:
		x = v.u
	case kTrailing:
		x = TrailingString(v.s)
	}
	if v.decr {
		return Decr(x)
	}
	return x
}

// target returns a pointer for Parse to decode an item of v's kind and
// direction into, and a function that reads the decoded value.
func (v value) target() (any, func() value) {
	out := value{k: v.k, decr: v.decr}
	var p any
	var read func() value
	switch v.k {
	case kString:
		p, read = &out.s, func() value { return out }
	case kInfinity:
		var x struct{}
		p, read = &x, func() value { return out }
	case kStringOrInfinity:
		var x StringOrInfinity
		p, read = &x, func() value { out.s, out.inf = x.String, x.Infinity; return out }
	case kFloat:
		p, read = &out.f, func() value { return out }
	case kInt:
		p, read = &out.i, func() value { return out }
	case kUint:
		p, read = &out.u, func() value { return out }
	case kTrailing:
		var x TrailingString
		p, read = &x, func() value { out.s = string(x); return out }
	}
	if v.decr {
		p = Decr(p)
	}
	return p, read
}

// same reports whether a and b are the same value, as the encoding sees it:
// -0 is +0, and NaNs are compared by their bits.
func same(a, b value) bool {
	if a.k == kFloat {
		if a.f == 0 && b.f == 0 {
			return a.decr == b.decr
		}
		return math.Float64bits(a.f) == math.Float64bits(b.f) && a.decr == b.decr
	}
	return a == b
}

// compare compares two values of the same kind and direction in the order
// the encoding promises. Floats must not be NaN.
func compare(a, b value) int {
	c := 0
	switch a.k {
	case kString, kTrailing:
		c = strings.Compare(a.s, b.s)
	case kInfinity:
	case kStringOrInfinity:
		switch {
		case a.inf && b.inf:
		case a.inf:
			c = 1
		case b.inf:
			c = -1
		default:
			c = strings.Compare(a.s, b.s)
		}
	case kFloat:
		c = cmp3(a.f < b.f, a.f > b.f)
	case kInt:
		c = cmp3(a.i < b.i, a.i > b.i)
	case kUint:
		c = cmp3(a.u < b.u, a.u > b.u)
	}
	if a.decr {
		return -c
	}
	return c
}

func cmp3(less, greater bool) int {
	switch {
	case less:
		return -1
	case greater:
		return 1
	}
	return 0
}

// gen draws values from fuzzer input, biased towards the values the
// encoding treats specially: escaped bytes, boundaries between encoded
// lengths, signs, infinities and NaNs. Exhausted input reads as zeros.
type gen struct{ b []byte }

func (g *gen) byte() byte {
	if len(g.b) == 0 {
		return 0
	}
	c := g.b[0]
	g.b = g.b[1:]
	return c
}

func (g *gen) uint64() uint64 {
	var b [8]byte
	n := int(g.byte() % 9) // significant bytes, so that small numbers are common
	for i := 8 - n; i < 8; i++ {
		b[i] = g.byte()
	}
	u := binary.BigEndian.Uint64(b[:])
	switch g.byte() % 8 {
	case 0: // just below or at a power of two
		return 1<<(u%64) - uint64(g.byte()%3)
	case 1:
		return math.MaxUint64 - uint64(g.byte()%4)
	}
	return u
}

var specialBytes = []byte{0x00, 0x01, 0xfe, 0xff, 'a', 'b'}

func (g *gen) string() string {
	n := int(g.byte() % 12)
	b := make([]byte, n)
	for i := range b {
		c := g.byte()
		if c < 0x80 {
			c = specialBytes[int(c)%len(specialBytes)]
		}
		b[i] = c
	}
	return string(b)
}

func (g *gen) value(k kind, decr bool) value {
	v := value{k: k, decr: decr}
	switch k {
	case kString, kTrailing:
		v.s = g.string()
	case kStringOrInfinity:
		if g.byte()%4 == 0 {
			v.inf = true
		} else {
			v.s = g.string()
		}
	case kFloat:
		switch g.byte() % 8 {
		case 0:
			v.f = []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64}[g.byte()%8]
		case 1:
			v.f = float64(int64(g.uint64())) / 16
		default:
			v.f = math.Float64frombits(g.uint64())
		}
	case kInt:
		v.i = int64(g.uint64())
		if g.byte()%2 == 0 {
			v.i = -v.i
		}
	case kUint:
		v.u = g.uint64()
	}
	return v
}

// tuple draws up to 5 kinds and directions; a TrailingString can only be last.
func (g *gen) schema() []value {
	n := 1 + int(g.byte()%5)
	vs := make([]value, n)
	for i := range vs {
		c := g.byte()
		vs[i].k = kind(int(c) % int(numKinds))
		if vs[i].k == kTrailing && i < n-1 {
			vs[i].k = kString
		}
		vs[i].decr = c&0x80 != 0
	}
	return vs
}

func (g *gen) tuple(schema []value) []value {
	out := make([]value, len(schema))
	for i, s := range schema {
		out[i] = g.value(s.k, s.decr)
	}
	return out
}

func appendAll(t testing.TB, vs []value) []byte {
	items := make([]any, len(vs))
	for i, v := range vs {
		items[i] = v.item()
	}
	b, err := Append(nil, items...)
	if err != nil {
		t.Fatalf("Append(%v): %v", vs, err)
	}
	return b
}

// parseAll parses enc with the kinds and directions of schema.
func parseAll(enc string, schema []value) ([]value, string, error) {
	targets := make([]any, len(schema))
	reads := make([]func() value, len(schema))
	for i, s := range schema {
		targets[i], reads[i] = s.target()
	}
	rem, err := Parse(enc, targets...)
	if err != nil {
		return nil, "", err
	}
	out := make([]value, len(schema))
	for i := range out {
		out[i] = reads[i]()
	}
	return out, rem, nil
}

func checkRoundTrip(t *testing.T, vs []value) {
	enc := appendAll(t, vs)
	got, rem, err := parseAll(string(enc), vs)
	if err != nil {
		t.Fatalf("Parse(Append(%v) = %q): %v", vs, enc, err)
	}
	if rem != "" {
		t.Fatalf("Parse(Append(%v) = %q) left %q", vs, enc, rem)
	}
	for i := range vs {
		if !same(got[i], vs[i]) {
			t.Fatalf("Parse(Append(%v) = %q) = %v", vs, enc, got)
		}
	}
	for j := 1; j < len(vs); j++ {
		if p := appendAll(t, vs[:j]); !bytes.HasPrefix(enc, p) {
			t.Fatalf("Append(%v) = %q is not a prefix of Append(%v) = %q", vs[:j], p, vs, enc)
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	seed(f, 32)
	f.Fuzz(func(t *testing.T, data []byte) {
		g := &gen{data}
		checkRoundTrip(t, g.tuple(g.schema()))
	})
}

// orderable reports whether the encoding promises an order between a and b:
// NaNs have none, and Decr(TrailingString) cannot order a string against
// its own extensions (see TestDecrTrailingStringPrefixes).
func orderable(a, b []value) bool {
	for i := range a {
		if a[i].k == kFloat && (math.IsNaN(a[i].f) || math.IsNaN(b[i].f)) {
			return false
		}
		if a[i].k == kTrailing && a[i].decr && (strings.HasPrefix(a[i].s, b[i].s) || strings.HasPrefix(b[i].s, a[i].s)) {
			return false
		}
		if compare(a[i], b[i]) != 0 {
			return true
		}
	}
	return true
}

func checkOrder(t *testing.T, a, b []value) {
	if !orderable(a, b) {
		return
	}
	want := 0
	for i := range a {
		if want = compare(a[i], b[i]); want != 0 {
			break
		}
	}
	ea, eb := appendAll(t, a), appendAll(t, b)
	if got := bytes.Compare(ea, eb); got != want {
		t.Fatalf("Append(%v) = %q\nAppend(%v) = %q\ncompare as %d; want %d", a, ea, b, eb, got, want)
	}
}

func FuzzOrder(f *testing.F) {
	seed(f, 32)
	f.Fuzz(func(t *testing.T, data []byte) {
		g := &gen{data}
		schema := g.schema()
		a := g.tuple(schema)
		// b shares a prefix with a, so that comparisons reach every item.
		b := append([]value(nil), a...)
		i := int(g.byte()) % len(b)
		b[i] = g.value(schema[i].k, schema[i].decr)
		for j := i + 1; j < len(b); j++ {
			b[j] = g.value(schema[j].k, schema[j].decr)
		}
		checkOrder(t, a, b)
		checkOrder(t, b, a)
	})
}

// checkCanonical parses enc with schema and, if Parse accepts it, checks
// that Append writes exactly the bytes Parse consumed.
func checkCanonical(t *testing.T, enc string, schema []value) {
	got, rem, err := parseAll(enc, schema)
	if err != nil {
		return
	}
	consumed := enc[:len(enc)-len(rem)]
	if again := appendAll(t, got); string(again) != consumed {
		t.Fatalf("Parse(%q) as %v accepted %q as %v, which Append writes as %q", enc, schema, consumed, got, again)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte{0, byte(kUint)}, []byte("\x01\x00"))
	f.Add([]byte{0, byte(kInt)}, []byte("\xc0\x00"))
	f.Add([]byte{0, byte(kFloat)}, []byte("\x00\x3f\x80\x00\x00\x00\x00\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, schemaBytes, enc []byte) {
		checkCanonical(t, string(enc), (&gen{schemaBytes}).schema())
	})
}

func seed(f *testing.F, n int) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < n; i++ {
		b := make([]byte, 16+rng.Intn(100))
		rng.Read(b)
		f.Add(b)
	}
}

// TestProperties runs the three fuzz properties on many random inputs, so
// that plain go test covers them well.
func TestProperties(t *testing.T) {
	n := 200000
	if testing.Short() {
		n = 20000
	}
	rng := rand.New(rand.NewSource(2))
	buf := make([]byte, 160)
	for i := 0; i < n; i++ {
		rng.Read(buf)
		g := &gen{append([]byte(nil), buf...)}
		schema := g.schema()
		a := g.tuple(schema)
		checkRoundTrip(t, a)
		b := g.tuple(schema)
		if j := int(g.byte()) % len(a); j > 0 {
			copy(b, a[:j]) // share a prefix
		}
		checkOrder(t, a, b)
		enc := g.b // the rest of the input, as an encoding
		if g.byte()%2 == 0 {
			enc = appendAll(t, a) // or a valid encoding, damaged
			if len(enc) > 0 {
				enc[rng.Intn(len(enc))] ^= byte(1 << rng.Intn(8))
			}
		}
		checkCanonical(t, string(enc), schema)
	}
}

// TestNumbersExhaustively checks every encoding of one or two bytes, and
// three-byte encodings starting with every byte, as int64, uint64 and
// float64 in both directions, for canonical parsing; and order and round
// trips across every boundary between encoded lengths.
func TestNumbersExhaustively(t *testing.T) {
	for _, k := range []kind{kInt, kUint, kFloat} {
		for _, decr := range []bool{false, true} {
			schema := []value{{k: k, decr: decr}}
			for n := 0; n < 1<<16; n++ {
				checkCanonical(t, string([]byte{byte(n >> 8), byte(n)}), schema)
				checkCanonical(t, string([]byte{byte(n)}), schema)
				checkCanonical(t, string([]byte{byte(n >> 8), byte(n), 0x00, 0xff, 0x80}), schema)
			}
		}
	}
	var ints []int64
	for b := 0; b < 64; b++ {
		for d := int64(-2); d <= 2; d++ {
			ints = append(ints, int64(1)<<b+d, -(int64(1)<<b)+d)
		}
	}
	ints = append(ints, math.MaxInt64, math.MaxInt64-1, math.MinInt64, math.MinInt64+1)
	for i := int64(-70000); i <= 70000; i++ {
		ints = append(ints, i)
	}
	for _, decr := range []bool{false, true} {
		for _, i := range ints {
			a := []value{{k: kInt, decr: decr, i: i}}
			checkRoundTrip(t, a)
			if i != math.MaxInt64 {
				checkOrder(t, a, []value{{k: kInt, decr: decr, i: i + 1}})
			}
			u := []value{{k: kUint, decr: decr, u: uint64(i)}}
			checkRoundTrip(t, u)
			if uint64(i) != math.MaxUint64 {
				checkOrder(t, u, []value{{k: kUint, decr: decr, u: uint64(i) + 1}})
			}
			f := []value{{k: kFloat, decr: decr, f: math.Float64frombits(uint64(i))}}
			checkRoundTrip(t, f)
		}
	}
}

// TestNonCanonicalRejected lists encodings that upstream's Parse accepted
// although Append never writes them, each a second encoding of a value
// that sorts apart from the first.
func TestNonCanonicalRejected(t *testing.T) {
	for _, tc := range []struct {
		k    kind
		decr bool
		enc  string
		as   string // what upstream decoded it as
	}{
		{kUint, false, "\x01\x00", "0, encoded \\x00"},
		{kUint, false, "\x02\x00\x05", "5, encoded \\x01\\x05"},
		{kUint, true, "\xfe\xff", "Decr 0, encoded \\xff"},
		{kInt, false, "\xc0\x00", "0, encoded \\x80"},
		{kInt, false, "\xc0\x3f", "63, encoded \\xbf"},
		{kInt, false, "\x3f\xff", "-1, encoded \\x7f"},
		{kInt, false, "\xe0\x00\x10", "16, encoded \\x90"},
		{kInt, false, "\xff\xc0\x80\x00\x00\x00\x00\x00\x00\x00", "math.MinInt64, from a positive encoding"},
		{kInt, true, "\x3f\xff", "Decr 0, encoded \\x7f"},
		{kFloat, false, "\x00\x3f\x80\x00\x00\x00\x00\x00\x00\x00", "+0, from the int64 encoding of math.MinInt64"},
		{kFloat, false, "\xc0\x00", "+0, encoded \\x80"},
	} {
		got, _, err := parseAll(tc.enc, []value{{k: tc.k, decr: tc.decr}})
		if err == nil {
			t.Errorf("Parse(%q) as %v = %v (upstream: %s); want an error", tc.enc, tc.k, got, tc.as)
		}
	}
}

// TestDecrTrailingStringPrefixes documents a limit of the encoding, kept for
// compatibility: a TrailingString has no terminator, so in decreasing order
// a string still sorts before its extensions, as in increasing order.
// Strings that are not prefixes of each other are ordered as promised.
func TestDecrTrailingStringPrefixes(t *testing.T) {
	a := []value{{k: kTrailing, decr: true, s: "a"}}
	ab := []value{{k: kTrailing, decr: true, s: "ab"}}
	b := []value{{k: kTrailing, decr: true, s: "b"}}
	if bytes.Compare(appendAll(t, a), appendAll(t, ab)) != -1 {
		t.Error(`Decr(TrailingString("a")) no longer sorts before Decr(TrailingString("ab")); update the documentation`)
	}
	checkOrder(t, a, b)
	checkOrder(t, ab, b)
}
