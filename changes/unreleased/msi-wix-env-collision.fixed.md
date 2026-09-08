- **The Windows MSI builds again on the GitHub runners.** `scripts/build-msi.sh` read the
  command that runs `wix` from the `WIX` environment variable, which the preinstalled WiX v3
  on `windows-latest` already exports as its installation directory
  (`C:\Program Files (x86)\WiX Toolset v3.14\`), so the `msi` job of the v0.6.0 release failed
  with `error: C:\Program is required` and no `opensysml-0.6.0-windows-amd64.msi` was published.
  The override is now `WIX_CMD`.
