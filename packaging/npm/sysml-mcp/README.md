# @openmbee/sysml-mcp

The [OpenSysML](https://github.com/Open-MBEE/OpenSysML) Model Context Protocol server,
`sysml-mcp`, as an npm command: it serves SysML v2 and KerML to coding agents over stdio —
the language server's diagnostics, navigation, editing and rendering over the model
documents under a workspace root; the runtime's parsing, evaluation, execution,
verification, querying and conversion; and persistent REPL sessions — as tools an agent
calls.

```sh
npx -y @openmbee/sysml-mcp -root .
```

This package has no dependencies. Its `sysml-mcp` command resolves the platform package
for the machine's `process.platform`/`process.arch` from its optional dependencies, at
exactly its own version, and runs the release binary that package carries with the
arguments and stdio it was given: stdout stays the protocol stream, signals the host
sends are passed on, and the command exits with the binary's status. The platform
packages, `@openmbee/sysml-mcp-<os>-<cpu>` for linux/darwin × x64/arm64 and win32 x64,
each hold the release `sysml-mcp` binary, the LICENSE and the manual page. On any other
platform, or after an install with `--omit=optional`, the command names the missing
package on stderr and exits non-zero; `go install
github.com/Open-MBEE/OpenSysML/cmd/sysml-mcp@latest` builds the server there.

Node 20.11 or later is required. The same server is also published as release archives
and through `install.sh`, Homebrew, Scoop and the MSI; see the
[MCP server reference](https://github.com/Open-MBEE/OpenSysML/blob/main/docs/reference/sysml-mcp.md)
for those, for every tool, and for the `-root`, `-tools` and `-strict` options.

## Configuring a host

Each block starts the server in the project, so `-root` can be left at its default, the
working directory. `sysml-mcp -print-config <host> -print-config-npx` writes the same
blocks from an installed binary.

Claude Code:

```sh
claude mcp add --transport stdio sysml -- npx -y @openmbee/sysml-mcp -root .
```

Codex:

```sh
codex mcp add sysml -- npx -y @openmbee/sysml-mcp -root .
```

Cursor (`.cursor/mcp.json`), Gemini CLI (`.gemini/settings.json`) and Devin
(Settings > MCP Marketplace > Add custom):

```json
{
  "mcpServers": {
    "sysml": {
      "command": "npx",
      "args": ["-y", "@openmbee/sysml-mcp", "-root", "."]
    }
  }
}
```

VS Code (`.vscode/mcp.json`):

```json
{
  "servers": {
    "sysml": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@openmbee/sysml-mcp", "-root", "."]
    }
  }
}
```

OpenCode (`opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "sysml": {
      "type": "local",
      "command": ["npx", "-y", "@openmbee/sysml-mcp", "-root", "."],
      "enabled": true
    }
  }
}
```

Claude Code also reads `.mcp.json` in the project, with the `mcpServers` block above and
`"type": "stdio"` before the command.

## Versions

The package version is the OpenSysML release it runs: `@openmbee/sysml-mcp@X.Y.Z` pins
`@openmbee/sysml-mcp-<os>-<cpu>@X.Y.Z`, built from the `vX.Y.Z` release binaries, and
`npx -y @openmbee/sysml-mcp@X.Y.Z` runs that release. Pre-releases are published under
the `next` dist-tag and nightly snapshots under `nightly`; `latest` is always a release.
`@openmbee/opensysml-mcp` is the same runner under the name the other OpenSysML npm packages
share (`@openmbee/opensysml`, `@openmbee/opensysml-wasm`): generated from this package at
publish time, it pins the same platform packages at the same version and adds `opensysml-mcp`
as a second name for the command.

## License

Apache-2.0, as OpenSysML.
