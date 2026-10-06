# Editors

Integrations that put OpenSysML inside a modeling tool or editor. Each lives in its own
directory here with its own build.

- **[VS Code](vscode/)** — the extension that runs `sysml-lsp` as its language server and hosts
  the diagram panel; see [`docs/guide/08-editors.md`](../docs/guide/08-editors.md) for what a user sees.

<!-- mdk: begin -->
- **[OpenSysML MDK](mdk/)** — a plugin for Cameo Systems Modeler 2026x Refresh1 (2024x
  Refresh3 as the minimum), positioned as the successor to OpenMBEE's Model Development Kit
  (MDK) for SysML v2 work; it adds an *OpenSysML* group to the browser and diagram context menus with
  Instantiate, Execute action, Execute state machine, Verify requirement/constraint, Evaluate
  calc and Run analysis. A SysML v2 project is exported through the textual notation service; a
  SysML v1 project leaves as a `.mdzip` and is migrated through `Convert(xmi→sysml)`; either is
  parsed and run on `sysml-grpc` through the Java client. Outcomes, diagnostics, final time and
  schedule land in a docking results window and as validation annotations on the Cameo elements,
  and — where MDK is installed — in DocGen documents through the bundled «JavaExtension» bridge.
  It builds against compile-only stubs of the OpenAPI, so no licence is needed in CI; the design
  is in [`docs/internals/design/mdk-plugin.md`](../docs/internals/design/mdk-plugin.md) and the
  MDK gap matrix in [`docs/project/mdk-parity.md`](../docs/project/mdk-parity.md).
<!-- mdk: end -->

- **Eclipse SysON** (`syson/`) — a plugin with a backend adapter and frontend dialog for
  [Eclipse SysON](https://github.com/eclipse-syson/syson), the Sirius Web based graphical SysML
  v2 workbench. Phases 1–2 provide right-click instantiate, execute, explore or verify operations
  on OpenSysML through the Java client in
  [`client/java/opensysml-client`](../client/java/opensysml-client), shows the results in SysON
  with diagnostics attached to the elements they concern. Parser checks on import and structural
  synchronization are designed only; the discovery against release `v2026.9.0`, module layout,
  call sequence and phased plan are in
  [`docs/internals/design/syson-plugin.md`](../docs/internals/design/syson-plugin.md).

## Versions

The editors carry the core version — `0.9.2` now — the same lockstep the clients
follow, even though nothing publishes them. `check_version.py --editors` checks
every manifest (the two package.json files and their locks, the Cameo and SysON
poms and their children's `<parent><version>`), and a release fails early when
one disagrees. The client references the editors build against —
`opensysml.client.version` in the Cameo pom and the `opensysml`
dependency in the SysON backend pom — are pinned to `client/java/pom.xml` the
same way.
