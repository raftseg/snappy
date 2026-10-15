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

## Both implementations in every build

Upstream compiled the Go code (`decode_other.go`, `encode_other.go`) only
where there is no assembly, so the two were never run side by side. They
are now built everywhere, with their functions renamed `decodeGo`,
`emitLiteralGo`, `emitCopyGo`, `extendMatchGo` and `encodeBlockGo`
(`encodeBlockGo` calls the Go versions of the others), and `pure.go` maps
the original names to them where there is no assembly, as before. Behaviour
is unchanged; the tests below compare the two.

## Decode bounds the length it allocates for

A block starts with its decoded length. Upstream's `Decode` allocated that
many bytes before decoding, so five bytes claiming 4 GiB made it allocate
4 GiB and then report the input corrupt. A block of n encoded bytes can
decode to at most 64 bytes per 3 bytes of copies, plus 11 for a final
2-byte copy (`maxDecodedLen`); `Decode` now reports a longer claim as
corrupt before allocating. Every input it accepted before decodes as
before, and every input it now refuses failed before too (the tests check
this on every input they decode). `DecodedLen` still reports the header's
claim. Test: `TestDecodeImpossibleLengthDoesNotAllocate`; `TestMaxDecodedLen`
builds the densest encodings and checks the bound is reached but not
exceeded. `Reader` was already bounded, by the framing format's 64 KiB
chunks.

## Tests

Upstream's tests are kept unchanged. `fork_test.go` adds:

- differential tests: on amd64 and arm64, the assembly and the Go code run
  on the same input and must agree exactly: `decode` on valid, damaged and
  arbitrary blocks, and `encodeBlock`, `emitLiteral`, `emitCopy` and
  `extendMatch` on generated input within their preconditions;
- round trips of the block format, checking `MaxEncodedLen` and
  `DecodedLen`, and of the stream format through `Writer` and
  `NewBufferedWriter` with random write sizes, flushes and resets, read
  back through `Read` and `ReadByte` in random pieces;
- hostile input: every decode re-encodes and decodes back, damaged
  streams fail cleanly, and output is bounded by input.

`FuzzDecode`, `FuzzEncode`, `FuzzReader` and `FuzzStream` drive them from
fuzzer input, and `TestDifferentialAndRoundTrip` runs them over a varied
corpus in plain `go test`.
