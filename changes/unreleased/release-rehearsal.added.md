- **A release can be rehearsed on a branch before the tag.** The
  `release_rehearsal` pipeline parameter runs the `release` workflow on a
  release branch with every check live and the irreversible commands — cosign
  signing, `ghr`, `twine upload`, `npm publish`, `mvn deploy`,
  `cargo publish` — skipped, so a rehearsal proves everything a tag can fail
  on except the uploads themselves. See *Rehearsing the release* in
  `docs/project/releasing.md`.
