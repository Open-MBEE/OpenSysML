- The npm and Maven release jobs now stamp the release's five `sysml-grpc` digests into the
  `release-digests.json` the `@openmbee/opensysml` tarball and the `org.openmbee:opensysml` jar
  ship, as the Python wheel and Rust crate already did, and fail unless the packed artifact pins
  the tag; a published Node or Java client therefore downloads its own release on a pin without
  the optional sigstore dependencies. The committed shared table is brought up to date with every
  signed release through `v0.9.2`, from the cosign-verified `SHA256SUMS.txt` of each. The tag's pipeline now
  ends by opening a `chore/pin-vX.Y.Z` pull request against `develop` that pins the release
  in the committed table and every client copy, and a hygiene test fails a release branch
  whose predecessor the committed table does not pin.
