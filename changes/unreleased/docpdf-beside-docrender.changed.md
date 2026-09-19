- **The PDF document backend sits beside the renderer it drives.** `internal/doc/docpdf` is
  `internal/doc/docpdf`, next to `internal/doc/docrender`, whose Markdown it converts; nothing
  changes for `sysml -render-document -doc-form pdf`.
