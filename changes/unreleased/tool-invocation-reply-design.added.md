- **A design note on running programs not written for the tool protocol**
  (`docs/internals/design/bring-your-own-engines.md`, section *Tools: composed invocations and
  structured replies*). Two optional blocks of the `OPENSYSML_TOOLS` entry are specified:
  `invocation`, which composes the command line, environment, working directory, standard input
  and an input file from the values the model binds, through a placeholder grammar with its
  rendering and escaping rules over a minimal base environment; and `reply`, which reads the
  outputs from JSON (by pointer), CSV (by column and row), key–value lines, or the exit status,
  on standard output or in a file the tool wrote, with the typed error each fault raises and
  where it is named. The note also specifies sequence-valued outputs, `ToolExecution` on a
  `calc def`, the `-tool-dry-run`/`%tool` surfaces, the recorded provenance, the security
  invariants and the delivery order. Nothing is implemented; an entry without the two blocks
  keeps today's meaning byte for byte, and the analysis-framework note points to the new section.
