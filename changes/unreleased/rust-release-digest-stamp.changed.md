- **Stamp Rust release digests into each published crate.** The publish job adds the release tag's
  service-asset digests from its checksum manifest, so a crates.io installation can verify and
  download the binary it was built against by default. A Git-checkout build or a request for
  another release still needs a matching pin or `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`; Rust does not
  verify the manifest's Sigstore signature itself.
