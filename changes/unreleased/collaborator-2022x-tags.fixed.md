- **SysML v1 migration reads the 2022x collaborator paragraph tags.** View Editor
  applications serialized by Cameo 2022x place a paragraph by `sectionId` and
  order it by `parentId`, with `viewId` naming the document's top view; every
  such paragraph was reported as a stray `names no view of the document` entry
  and dropped from the migrated document.
