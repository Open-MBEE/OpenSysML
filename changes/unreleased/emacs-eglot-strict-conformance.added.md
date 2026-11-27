- **Strict conformance under Eglot is documented and tested.** The Emacs README shows
  `sysml.strictConformance` as `(:sysml (:strictConformance t))` in Eglot's
  `eglot-workspace-configuration`, for every project or one through `.dir-locals.el`, and
  `M-x eglot-signal-didChangeConfiguration` as what applies a change without a restart. A live
  test holds the server to both.
