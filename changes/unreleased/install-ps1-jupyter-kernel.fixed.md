- **`install.ps1` installs the Jupyter kernel.** `-Tools sysml-jupyter-kernel` was refused as an
  unknown tool and `-Tools all` stopped at `sysml-grpc`, although `install.sh` had accepted both
  since the kernel became a release asset. The Windows script now downloads
  `sysml-jupyter-kernel-<os>-<arch>[.exe]`, checks it against `SHA256SUMS.txt`, installs and runs
  it with the other tools, and names it in `-DryRun` output; a release without it is reported as
  the manifest not listing the kernel (published from v0.10.0). When the signed Windows build is
  chosen, the kernel, which that build does not carry, still comes from the release's own asset
  and manifest.
