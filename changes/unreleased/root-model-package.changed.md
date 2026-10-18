- A SysML v1 migration writes a root `uml:Model` as `package <ModelName> { … }`, as the OMG
  transformation maps it, instead of writing its members at the file's top level. Qualified
  names are written from that package, `$::` staying only where a member still shadows the
  global name; an unnamed Model is named after the file; a member of the Model named like a
  standard library root package (`Requirements`, `Views`, …) keeps its name, only a root
  declaration being renamed; the report's Model row reads `written as package <name>`.
