- Deciding whether an imported name is hidden (KerML 7.2.5.4: two imports bringing distinct
  elements under one name) no longer enumerates everything a namespace's imports bring, nor the
  imports of the namespaces they re-export from. It looks the one name up through each import
  instead, so analysing a document chain where each package imports the previous one is linear
  again rather than quadratic in the chain's depth: `sysml -validate` over 800 such documents, and
  the gRPC `ParseSources` call that the validation service makes over them, take about a quarter
  of the time they did since the hiding was introduced, as they did before it. The
  `Duplicate of imported member name` audit still enumerates once per declared namespace, and
  the hidden names, their hints and the audit's warnings are unchanged.
