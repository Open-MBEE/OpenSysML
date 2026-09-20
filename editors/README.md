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
