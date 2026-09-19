- **The PDF document backend sits beside the renderer it drives.** `internal/docpdf` is
  `internal/core/docpdf`, next to `internal/core/docrender`, whose Markdown it converts; nothing
  changes for `sysml -render-document -doc-form pdf`.
