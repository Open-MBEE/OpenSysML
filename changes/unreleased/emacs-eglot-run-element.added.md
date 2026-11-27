- **`M-x sysml-eglot-run-element` runs an element under Eglot.** Eglot shows no code lenses, so the
  Run and Evaluate lenses `sysml-lsp` offers could be used under lsp-mode only. The command asks
  the server for the buffer's lenses and runs the one on the name at point, or one read from the
  minibuffer, with `sysml` in a compilation buffer, as lsp-mode's lens does. Finding `sysml`,
  choosing the model paths and refusing to run unsaved buffers moved from `sysml-lsp.el` to
  `sysml-mode.el`, where both clients share them.
