- **Clients use their built-against service release by default.** Python and Java download and
  verify that release. A Rust crate published from a release tag also verifies its built-against
  release: the publish job stamps its digests from the release checksum manifest. A Git-checkout
  build or a request for another release still needs a pin or
  `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`; Rust does not verify the manifest's Sigstore signature.
