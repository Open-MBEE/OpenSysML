- **The development tools are a nested Go module, `tools/`.** `tools/go.mod` requires the
  product module through `replace … => ../`, so the tools build against the working tree while
  `go build ./...` and `go test ./...` at the root stay product-only; `make test` and `make lint`
  run both modules. The library snapshot generator and the ontology table generator are its first
  residents, at `tools/gen/snapshot` and `tools/gen/ontology`, invoked with
  `go run -C tools ./gen/<name>` (the `go:generate` directives and `make stdlib-snapshot-check`
  follow); both resolve the repository root through `tools/oracle/repo` rather than the working
  directory.
- **The errata overlay is split from the registry.** The entry type, the overlay applied to the
  bundled standard library on read and the library's own entries are the product's
  `internal/core/libs/errata`; `internal/errata` keeps the registry the oracles read — the corpus
  entries, the published roots and the corrected copy of a corpus root — and builds on it.
