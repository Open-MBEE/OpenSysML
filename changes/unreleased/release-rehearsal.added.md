- **A release can be rehearsed on a branch before the tag.** The
  `release_rehearsal` pipeline parameter runs the `release` workflow on a
  release branch with every check live and the irreversible commands — cosign
  signing, `ghr`, `twine upload`, `npm publish`, `mvn deploy`,
  `cargo publish` — skipped. What a green rehearsal does and does not prove
  is listed under *Rehearsing the release* in `docs/project/releasing.md`.
