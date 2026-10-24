- The MCP server's npm runner is also published as `@openmbee/opensysml-mcp`, the name the
  other OpenSysML npm packages share: `npx -y @openmbee/opensysml-mcp` starts the same
  server as `npx -y @openmbee/sysml-mcp`, from the same `@openmbee/sysml-mcp-<os>-<cpu>`
  platform packages at the same version, through an `opensysml-mcp` command beside
  `sysml-mcp`. The alias is generated from `packaging/npm/sysml-mcp` by
  `client/node/scripts/build-platform-packages.mjs` and published by the release's npm job
  and the nightly after `@openmbee/sysml-mcp`; the MCP Registry listing stays on
  `@openmbee/sysml-mcp`.
