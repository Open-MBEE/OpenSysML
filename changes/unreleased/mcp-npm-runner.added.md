- `npx -y @openmbee/sysml-mcp` runs the MCP server with nothing preinstalled: the
  dependency-free npm package `@openmbee/sysml-mcp` (`packaging/npm/sysml-mcp`) picks the
  platform package `@openmbee/sysml-mcp-<os>-<cpu>` for the machine, at its own version, and
  runs the release `sysml-mcp` binary it carries with the caller's arguments and stdio,
  forwarding signals and exiting with the binary's status. The five platform packages (with
  LICENSE and the man page) are built by `client/node/scripts/build-platform-packages.mjs`
  from the release binaries, verified against their `.sha256` sidecars, and published by the
  release's npm job and the nightly beside `@openmbee/opensysml`, platform packages first.
  `sysml-mcp -print-config <host> -print-config-npx` writes the host's block in that form.
