# orderedcode

A fork of [Google's orderedcode](https://github.com/google/orderedcode) at
v0.0.1, kept free of dependencies: a byte encoding of typed tuples
(strings, integers, floats, an infinity, each optionally in decreasing
order) whose bytes sort in the same order as the tuples.

- **Module:** `github.com/raftseg/orderedcode`.
- **No dependencies:** `go.mod` requires nothing, and every package, tests
  included, imports only the Go standard library and this module.
  `internal/depcheck` fails the tests otherwise.
- **Same API and encoding as upstream v0.0.1.** `Parse` accepts only the
  encodings `Append` writes, so each value has exactly one.
  [FORK.md](FORK.md) lists every change.
- **Property tests:** round trip, prefix, order and canonical parsing,
  over generated tuples and from fuzzer input.

Upstream's README is [UPSTREAM-README.md](UPSTREAM-README.md). The license
is upstream's, Apache 2.0 ([LICENSE](LICENSE)); [NOTICE](NOTICE) lists what
this fork changed.

## Relation to upstream

This fork's history starts at upstream's tag v0.0.1 (commit `9a3d15e`,
upstream's `master`), with upstream's full history below it. The branch
`upstream-main` mirrors upstream's default branch. Upstream fixes are taken
with `git cherry-pick -x`, so each commit names the upstream commit it came
from, and are listed in [upstream.json](upstream.json), which SBOM tools
read for this fork's pedigree.

## Testing

```
go test ./...
go test -short -race ./...
go test -run XXX -fuzz FuzzOrder .        # and FuzzRoundTrip, FuzzParse
```

orderedcode is maintained alongside [raftseg](https://github.com/raftseg/raftseg).
