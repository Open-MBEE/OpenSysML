- **Added a SysML v1 to v2 transformation census.** `docs/project/sysml-v1-transformation-census.md`
  enumerates every mapping class of OMG's pinned SysML v1 to v2 transformation model and ties each
  to the migrator code and test that carries it out, with a per-verdict status; `go run -C tools
  ./cmd/transformation-census -check` gates the document, its citations and the scope measurements
  against the committed baseline, and `./scripts/download-sysml-v1tov2.sh` provisions the pinned
  model the extraction is compared against.
