- **Named standard-library elements carry the normative element ids the KerML and SysML
  specifications fix for them.** `ScalarValues::Real` is `14c0aa22-5489-59b5-b438-ded26e83ba31`
  here as it is in the pilot implementation and on every conforming SysML v2 API server, instead
  of the encoded name `ScalarValues__Real`, so a graph, a Flexo project or an element-by-element
  comparison that names a library element names the same element on both sides. The id is the
  name-based UUID (`uuid5`) the norm prescribes: the library package's over the OMG specification
  URL and its name, a named member's over the package id and its qualified name, and the owning
  membership's over the same with `/owningMembership` appended. The RDF mapping writes the
  normative id as the IRI tail, `sysml:elementId` and owning-membership IRI of every named element
  of the bundled library and reads it back without inventing an `@IdentityMetadata::ElementId`
  for it; the Flexo sync treats it as neither declared nor mintable; the LSP hover states it with
  its source and language (`Element id `14c0aa22-…` (normative, KerML)`), and the code action that
  mints an id is no longer offered on a library element. A declared `@ElementId` still wins, and a
  user element without one keeps its encoded name. Unnamed, aliased and shadowed library elements
  keep derived ids, since the norm gives them none that agrees across implementations.
  `TestPilotLibraryXMI` asserts every derived id and owning membership against the pilot's
  `sysml.library.xmi` at the pinned release (`./scripts/download-pilot-library-xmi.sh`), and CI
  requires the download.
