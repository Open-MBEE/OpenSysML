- `Convert` reports the semantic check's diagnostics alongside the converted output when
  `ConvertRequest.include_check_diagnostics` is set, each with its own severity and code as
  `ParseFile` and `GetDiagnostics` report them; the `convert_check_diagnostics` capability names it.
  Off, the default, only syntax errors are reported, as before.
