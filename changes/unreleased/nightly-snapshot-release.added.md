- **A nightly snapshot of `develop` is published as the prerelease `nightly`.** Every night
  the newest green `develop` commit is built into the same archives, raw `sysml-grpc`
  binaries and signed `SHA256SUMS.txt` a release ships, and published under the moving
  `nightly` tag; a version of the form `nightly-<yyyymmdd>-<commit>` tells a snapshot apart
  from a release. The snapshot is never marked latest, so `releases/latest`, Homebrew and the
  client packages keep following the stable line. The new *Nightly snapshots* page, linked
  from the landing page and the install guide, says where the snapshot is, what it contains,
  how to verify one and what to expect from it.
