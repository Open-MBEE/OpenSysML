- **SysML v1 migration reads only open model formats.** `-convert` from `xmi` takes OMG XMI 2.5.1
  with UML 2.5 and the OMG SysML 1.x profile, and Eclipse UML2 `.uml` files as Papyrus writes
  them (`uml` is the new `-from` synonym; `.uml` is recognized by extension). The reader no
  longer opens a modeling tool's proprietary project archive (`.mdzip`): a zip archive is
  refused with an error asking for the tool's XMI export. Only the OMG and Papyrus namespaces
  of the SysML profile classify elements; a tool's own customization stereotypes over SysML
  are preserved as applied-stereotype comments like any other custom profile rather than
  special-cased. The canonical fixture and its goldens are a vendor-neutral XMI export.
