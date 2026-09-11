- **SysML v1 migration reads the open model formats, tool-neutrally.** `-convert` from `xmi`
  takes OMG XMI 2.5.1 with UML 2.5 and the OMG SysML 1.x profile, Eclipse UML2 `.uml` files as
  Papyrus writes them (`uml` is the new `-from` synonym; `.uml` is recognized by extension), and
  a zip archive holding the XMI, a `.mdzip` project among them. Only the OMG and Papyrus
  namespaces of the SysML profile classify elements — matched by host and path, so a lookalike
  namespace elsewhere does not — and a tool's own customization stereotypes over SysML are
  preserved as applied-stereotype comments like any other custom profile rather than
  special-cased. The canonical fixture and its goldens are a vendor-neutral XMI export.
