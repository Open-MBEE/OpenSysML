- **A design note for an Eclipse SysON plugin** (`docs/internals/design/syson-plugin.md`). It
  records, against SysON release `v2026.9.0`, how a jar contributes beans to the SysON backend
  and a React component to its frontend, the textual exporter and its gaps, the SysIDE-based
  importer and where a pre-import check sits, the partial SysML v2 REST API and the fact that
  SysON's standard-library `elementId`s equal the normative UUIDs `internal/semantic/identity`
  derives, how a qualified-name `Symbol.id` maps to an EMF element, how diagnostics reach the
  Validation view, and the verdict of `sysml -validate` on every textual model in the SysON
  repository; then the architecture of a future `editors/syson/`, a four-phase plan and the
  unknowns. `editors/README.md` now introduces each editor integration. Nothing is implemented.
