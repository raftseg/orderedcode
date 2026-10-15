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
