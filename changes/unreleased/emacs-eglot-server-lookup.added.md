- **Eglot finds the server as lsp-mode does.** `sysml-eglot.el` starts
  `sysml-lsp-server-path` if set, then `bin/sysml-lsp` in the project root, then `sysml-lsp`
  on `exec-path`, and passes `sysml-lsp-server-args`, so a checkout that ran `make build`
  runs the server it just built under either client.
