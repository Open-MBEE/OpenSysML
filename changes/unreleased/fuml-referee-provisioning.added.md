- **The fUML reference implementation's activity tests are provisioned as an oracle for the
  action executor.** `./scripts/download-fuml-suite.sh` fetches ModelDriven's pinned
  `fUML-Tests.uml`, `fUML-Exception-Tests.uml`, the foundational library and the `fuml-1.5.0a`
  jar with its Maven runtime dependencies, every one by checksum, into the ignored `build/fuml/`;
  `make fuml-expected` runs the implementation over both models through a small Java driver
  that selects each activity by XMI id and records its outputs, `Execute`/`Fire`/`Output` trace
  and provenance in `docs/project/fuml-referee-expected.json`. The record is committed and
  `internal/fuml` reads it back, refusing one whose provenance is not the current pin's, so the
  ordinary test gate never runs Java. See `docs/project/fuml-referee.md`.
