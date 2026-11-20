- The parser now rejects the definition and usage prefixes that the SysML v2 grammar does not
  offer on these declarations, each as a syntax error at the prefix keyword:
  `variation metadata def` and `individual metadata def` (a metadata definition admits only
  `abstract`), `individual attribute def` and `individual port def` (only an occurrence
  definition admits `individual`), and `abstract`, `variation` or `individual` before a
  `metadata` usage. They were accepted without a diagnostic, even under `-strict`, and set
  `isVariation`, `isIndividual` and an implied multiplicity on the element (#2420).
