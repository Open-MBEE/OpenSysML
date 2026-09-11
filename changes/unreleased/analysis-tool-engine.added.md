- **Actions annotated `ToolExecution` run through an external tool.** A directory named by
  `OPENSYSML_TOOLS` holds one JSON file per tool — its `toolName`, `version`, `executable` and the
  `variables` it accepts — and each registers a `tool:<name>` engine that `sysml -engines`,
  `%engines` and `ListEngines` list with its process status like `solve`. A performance of an
  action carrying `AnalysisTooling::ToolExecution` starts the tool once, writes one JSON request
  (`toolName`, `uri`, `inputs` keyed by `ToolVariable` name with value and unit) to its standard
  input, reads one JSON reply from its standard output and binds the `outputs` to the action's
  parameters converted to their declared units; the body is never run. A tool with no entry is
  refused with `tool 'ModelCenter' is not registered; set OPENSYSML_TOOLS`; a non-zero exit,
  malformed or missing output, an unknown output, a unit the model does not declare or the
  timeout `OPENSYSML_TOOL_TIMEOUT` (default `10s`) fails the performance with a typed error, and
  no value is ever invented. A tool's answer stands at strength *observed*; equal inputs
  answered differently are noted as a divergence of the run.
