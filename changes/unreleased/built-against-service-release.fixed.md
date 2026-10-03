- **Clients use their built-against service release by default.** Python and Java download and
  verify that release. Rust tries its built-against release but verifies pinned digests only, so
  for releases without a pin it falls back to `$PATH` with a warning or errors if no executable is
  available.
