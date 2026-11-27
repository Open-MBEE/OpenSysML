- **`M-.` on a standard library name reaches its declaration in Emacs.** The server locates
  a library declaration under the `sysml-stdlib:` scheme, which names no file: lsp-mode
  answered "No definitions found" and Eglot opened an empty buffer. Both clients now show the
  declaration in the read-only library buffer the diagram view already uses, and list a
  reference found in the library the same way.
