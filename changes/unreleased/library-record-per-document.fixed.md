- A library loaded from `OPENSYSML_LIBRARY_PATH` that holds two copies of a file now reports
  the same diagnostics on a warm start as on a cold one. The record cache keyed a library
  document's facts on its content, so two copies of one file shared one record, and the
  second start restored the supertypes of each copy from the record the other copy wrote:
  a connection end then inherited `self` through both copies of `Parts::Part` and warned
  `Duplicate of inherited member name 'self' from Object, Part` where the first start had
  warned nothing. The key now names the document as well, so each copy restores the facts
  its own parse derived.
