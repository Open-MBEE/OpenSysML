# Editors

Integrations that put OpenSysML inside a modeling tool or editor. Each lives in its own
directory here with its own build.

- **[VS Code](vscode/)** — the extension that runs `sysml-lsp` as its language server and hosts
  the diagram panel; see [`docs/guide/08-editors.md`](../docs/guide/08-editors.md) for what a user sees.

<!-- cameo: begin -->
- **Cameo Systems Modeler** (`cameo/`, not yet started) — a plugin for Cameo 2026x Refresh1
  (2024x Refresh3 as the minimum) that exports the selected package or project, migrates it
  from SysML v1 to v2 through `Convert(xmi→sysml)` — or takes a SysML v2 project's textual
  export directly — parses and runs it on `sysml-grpc` through the Java client, and lands the
  verdicts on the Cameo elements they came from; the discovery and design are in
  [`docs/internals/design/cameo-plugin.md`](../docs/internals/design/cameo-plugin.md).
<!-- cameo: end -->

- **Eclipse SysON** (`syson/`) — a plugin with a backend adapter and frontend dialog for
  [Eclipse SysON](https://github.com/eclipse-syson/syson), the Sirius Web based graphical SysML
  v2 workbench. Phases 1–2 provide right-click instantiate, execute, explore or verify operations
  on OpenSysML through the Java client in
  [`client/java/opensysml-client`](../client/java/opensysml-client), shows the results in SysON
  with diagnostics attached to the elements they concern. Parser checks on import and structural
  synchronization are designed only; the discovery against release `v2026.9.0`, module layout,
  call sequence and phased plan are in
  [`docs/internals/design/syson-plugin.md`](../docs/internals/design/syson-plugin.md).

No code lives here for [OpenCode](https://opencode.ai): the checkout's
[`opencode.json`](../opencode.json) declares `sysml-lsp` to it, and the same block in a user's
global configuration enables the server for every project; see
[the guide](../docs/guide/08-editors.md#opencode).
