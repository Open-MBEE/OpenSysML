# Editors

Integrations that put OpenSysML inside a modeling tool or editor. Each lives in its own
directory here with its own build.

- **[VS Code](vscode/)** — the extension that runs `sysml-lsp` as its language server and hosts
  the diagram panel; see [`docs/guide/08-editors.md`](../docs/guide/08-editors.md) for what a user sees.

<!-- cameo: begin -->
- **[Cameo Systems Modeler](cameo/)** — a plugin for Cameo 2026x Refresh1 (2024x Refresh3 as
  the minimum) that adds an *OpenSysML* group to the browser and diagram context menus with
  Instantiate, Execute action, Execute state machine, Verify requirement/constraint, Evaluate
  calc and Run analysis. A SysML v2 project is exported through the textual notation service; a
  SysML v1 project leaves as a `.mdzip` and is migrated through `Convert(xmi→sysml)`; either is
  parsed and run on `sysml-grpc` through the Java client. Outcomes, diagnostics, final time and
  schedule land in a docking results window and as validation annotations on the Cameo elements.
  It builds against compile-only stubs of the OpenAPI, so no licence is needed in CI; the design
  is in [`docs/internals/design/cameo-plugin.md`](../docs/internals/design/cameo-plugin.md).
<!-- cameo: end -->
