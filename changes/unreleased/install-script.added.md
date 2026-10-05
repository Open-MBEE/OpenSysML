- **An install script for every supported platform.** `curl -fsSL https://opensysml.org/install.sh | sh`
  on Linux and macOS, `irm https://opensysml.org/install.ps1 | iex` on Windows: each works out
  the operating system and architecture, downloads the matching release bundle together with
  `SHA256SUMS.txt`, installs nothing whose digest does not match, and runs the installed
  `sysml` and `sysml-lsp` before reporting where they went. Options pick a release tag or the
  nightly snapshot, the tools (`sysml`, `sysml-lsp`, `sysml-grpc` or `all`), the destination
  and a release mirror; `--dry-run` shows the choice, and `--verify-signature` additionally
  checks the manifest's cosign signature. The Windows script installs the portable ZIP for the
  current user, preferring the signed build when the release carries one and its Authenticode
  signatures verify on this machine, and puts it on the user `PATH`. The scripts live at the repository root, are published from the documentation
  site, and are tested in CI against a release served from localhost.
