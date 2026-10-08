- **`python -m jupyter_opensysml_kernel` passes only the kernel's own flags to the bundled
  `sysml-jupyter-kernel`.** The launcher accepts `-connection-file FILE` (the file must exist),
  `-verbose`, `-version`, `-man` and `-print-kernelspec`, and refuses anything else with
  `error: ... is not a kernel flag` rather than forwarding it. The files an install writes get
  fixed permissions: the kernel binary `0755`, `kernel.json` `0644`.
