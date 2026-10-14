- Deciding whether an imported name is hidden (KerML 7.2.5.4: two imports bringing distinct
  elements under one name) no longer enumerates everything a namespace's imports bring, nor the
  imports of the namespaces they re-export from. It looks the one name up through each import
  instead, and walks the re-exporting namespaces once per name rather than once per re-exported
  entry, so analysing a chain of documents each re-exporting the previous one (`public import
  P::*`) no longer re-enumerates the whole chain for every document: `sysml -validate` and the
  gRPC `ParseSources` call over 200 such documents take an eighth of the time. The
  `Duplicate of imported member name` audit still enumerates once per declared namespace, and
  the hidden names, their hints and the audit's warnings are unchanged.
