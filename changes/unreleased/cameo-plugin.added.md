- **A Cameo Systems Modeler plugin runs models on OpenSysML** (`editors/cameo/`). Right-clicking
  an element in the containment tree or on a diagram offers an *OpenSysML* group with Instantiate,
  Execute action, Execute state machine, Verify requirement/constraint, Evaluate calc and Run
  analysis. A SysML v2 project (Cameo 2026x) is exported through the textual notation service; a
  SysML v1 project is saved as a `.mdzip` and migrated through `Convert`; either is parsed and run
  on `sysml-grpc` through the Java client, off the event thread with progress and cancel. Outcomes,
  diagnostics, final time and the state schedule appear in a docking *OpenSysML Results* window
  (double-click selects the element in the browser) and as validation annotations on the elements,
  matched by qualified name on both paths. The module compiles against compile-only stubs of the
  OpenAPI so CI needs no licence; `CAMEO_HOME` compiles it against a real installation, and the
  `dist` build stages the digest-pinned service binaries for every platform into a Resource
  Manager zip.
