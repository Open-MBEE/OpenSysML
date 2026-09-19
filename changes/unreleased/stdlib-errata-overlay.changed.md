- **The declared errata overlay now covers the bundled standard library.** `internal/errata`
  accepts entries under `internal/workspace/libs/stdlib` beside the example corpora, and the nine
  dimension defects the expression type checker reports in the published `SI.sysml` and
  `USCustomaryUnits.sysml` are its entries, each with a citation, a derivation and the published
  line it must still match. Three have one reading with the declared dimension and carry a
  correction (`eV*m^-2/kg` → `eV*m^2/kg`, `m^3/C*m^3*s^-1*A^-1` → `m^3/C`, `229835/900 [K]` →
  `(229835/900) [K]`); the other six are documented without one. The library a process loads
  (`libs.BundledSource`, `libs.DefaultSource` and the generated `stdlib.snapshot`) is the
  published text with the corrected lines substituted on read — the vendored bytes are never
  edited, `libs.EmbeddedSource` still serves them as published, and a directory named by
  `OPENSYSML_LIBRARY_PATH` is read as it stands. A library read fails rather than serve the file
  uncorrected when a declared line no longer matches, and two entries naming one line are refused.
  `TestExprTypeCheckPublishedStdlibDefects` pins all nine findings over the published text;
  `TestExprTypeCheckNoStdlibFalsePositives` pins exactly the six uncorrected ones over the bundled
  library. Derivations are in `docs/project/omg-issues.md` ("Defects in the vendored quantity
  libraries"); nothing is filed upstream.
