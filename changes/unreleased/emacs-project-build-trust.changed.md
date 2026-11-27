- **Emacs asks before running a checkout's own build.** The first time a project's
  `bin/sysml-lsp` or `bin/sysml` would be started, under lsp-mode or Eglot, Emacs asks
  whether to run what is built there, so each checkout is asked about once and a refusal
  is remembered. Answers are saved in the new `sysml-project-builds-file`, never in the
  Custom file, and the new `sysml-project-builds` option stands before them. A batch
  Emacs runs neither, and a checkout reached over TRAMP is recorded under its remote
  name. A build passed over for `exec-path` is named in a message, and in the error when
  no other is found; a restart or reconnect does not run a build no longer allowed.
  A server named in `sysml-lsp-server-path` is run as named, without the question.
  Until now lsp-mode started a project's `bin/sysml-lsp` unasked.
