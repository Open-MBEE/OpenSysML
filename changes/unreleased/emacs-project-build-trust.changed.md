- **Emacs asks before running a checkout's own build.** The first time a project's
  `bin/sysml-lsp` or `bin/sysml` would be started, under lsp-mode or Eglot, Emacs asks
  whether to run what is built there and saves the answer in the new
  `sysml-project-builds` option, so each checkout is asked about once and a refusal is
  remembered. A batch Emacs runs neither, and a checkout reached over TRAMP is recorded
  under its remote name. Until now lsp-mode started a project's `bin/sysml-lsp` unasked.
