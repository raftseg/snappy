# snappy

A fork of [Snappy-Go](https://github.com/golang/snappy) at v1.0.0, kept free
of dependencies: the Snappy compression format, block and framed stream,
with assembly for amd64 and arm64 and pure Go elsewhere.

- **Module:** `github.com/raftseg/snappy`.
- **No dependencies:** `go.mod` requires nothing, and every package, tests
  included, imports only the Go standard library and this module.
  `internal/depcheck` fails the tests otherwise.
- **Same API and format as upstream.** `Decode` refuses, before
  allocating, a length its input cannot decode to. [FORK.md](FORK.md)
  lists every change.
- **Differential tests:** the assembly and the Go code are built side by
  side and must agree on valid, damaged and arbitrary input.

Upstream's README is [UPSTREAM-README](UPSTREAM-README). The license is
upstream's, BSD 3-Clause ([LICENSE](LICENSE), [AUTHORS](AUTHORS));
[NOTICE](NOTICE) lists what this fork changed.

## Relation to upstream

This fork's history starts at upstream's tag v1.0.0 (commit `43d5d4c`),
with upstream's full history below it. The branch `upstream-main` mirrors
upstream's default branch. Upstream fixes are taken with
`git cherry-pick -x`, so each commit names the upstream commit it came
from, and are listed in [upstream.json](upstream.json), which SBOM tools
read for this fork's pedigree. The one upstream commit after v1.0.0, a
bound on the length header's size, is taken.

## Testing

```
go test ./...
go test -tags noasm ./...     # the pure Go code in place of the assembly
go test -short -race ./...
go test -run XXX -fuzz FuzzDecode .    # and FuzzEncode, FuzzReader, FuzzStream
```

snappy is maintained alongside [raftseg](https://github.com/raftseg/raftseg).
