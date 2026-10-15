# How this snappy differs from Snappy-Go v1.0.0

This package is a fork of [Snappy-Go](https://github.com/golang/snappy) at
tag v1.0.0 (BSD 3-Clause license, see LICENSE). This file lists every
change and why it was made. Every modified file says so in its header.

## Taken from upstream

- b6a66aa, "Limit length of varint in decodedLen": the block length header
  may be at most five bytes, as in the C++ implementation.

## Module and layout

- The module path is `github.com/raftseg/snappy`, and `go.mod` requires Go
  1.24 or later. The package's import comment and `cmd/snappytool` name the
  new path. gofmt added `//go:build` lines and reformatted comments.
- `internal/depcheck` fails the tests if `go.mod` requires anything or any
  Go file imports anything outside the standard library and this module.
- CI runs the tests on amd64 and arm64, each with its assembly and with
  `-tags noasm`, which builds the pure Go code in its place.
