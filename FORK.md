# How this orderedcode differs from Google's orderedcode v0.0.1

This package is a fork of [orderedcode](https://github.com/google/orderedcode)
at tag v0.0.1 (Apache License 2.0, see LICENSE). This file lists every
change and why it was made. Every modified file says so in its header.

## Module and layout

- The module path is `github.com/raftseg/orderedcode`, and `go.mod`
  requires Go 1.24 or later. The package's import comment names the new
  path, and gofmt reformatted its doc comments.
- `internal/depcheck` fails the tests if `go.mod` requires anything or any
  Go file imports anything outside the standard library and this module.

## Parsing accepts only the encodings Append writes

Upstream's `Parse` accepted some float64, int64 and uint64 encodings that
`Append` never writes: numbers with leading zero bytes (`"\x01\x00"` for
uint64 0, whose encoding is `"\x00"`), int64 encodings longer than needed
(`"\xc0\x00"` for 0, whose encoding is `"\x80"`), a ten-byte positive
encoding that overflowed to math.MinInt64, and the int64 encoding of
math.MinInt64 read as float64 +0. Each gave a value a second encoding that
sorts apart from the first, so a key checked by parsing it could still sort
where its value does not belong, and two keys that parse to the same values
could differ. `Parse` now re-encodes each number it reads and returns the
corrupt-input error unless the bytes match. Everything `Append` writes
parses as before. Test: `TestNonCanonicalRejected`.

## Tests

Upstream's tests are kept unchanged. `fork_test.go` adds property tests
over generated tuples of every item type, in both directions, biased
towards escaped bytes, boundaries between encoded lengths, signs,
infinities and NaNs:

- round trip: `Parse` returns what `Append` encoded, with nothing left;
- prefix: the encoding of a tuple's first items prefixes its encoding;
- order: encodings compare as the tuples do;
- canonical: whatever `Parse` accepts, `Append` writes back byte for byte.

`FuzzRoundTrip`, `FuzzOrder` and `FuzzParse` drive them from fuzzer input;
`TestProperties` runs them on random input in plain `go test`; and
`TestNumbersExhaustively` parses every one- and two-byte input as each
number type and direction and checks round trips and order across every
boundary between encoded lengths.

They also document a limit of the encoding, kept for compatibility: a
`TrailingString` has no terminator, so in decreasing order it still sorts
before its own extensions (`TestDecrTrailingStringPrefixes`). The type's
documentation now says so.
