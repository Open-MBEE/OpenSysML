---
description: OpenSysML client libraries — the Go, Python, Node, Java, Rust, Julia and MATLAB APIs that talk to the same runtime behind sysml, sysml-lsp and sysml-grpc.
---

# Client libraries

The same runtime drives `sysml`, `sysml-lsp` and `sysml-grpc`. The clients below talk
to that runtime — most through the `sysml-grpc` service, so a client gets exactly what
the CLI gets, over the wire. See [from your own program](guide/09-clients.md) for the
worked example and [service transports](reference/service-transports.md) for what the
service serves.

<div class="osml-eco__grid">
<div class="osml-eco__card">
  <span class="osml-eco__tag">Published · go install</span>
  <h3>Go</h3>
  <p>The native client — the runtime's own language, so every type is the real one.
  <code>go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest</code>, or import
  <code>client/opensysml</code>.</p>
  <p class="osml-eco__links"><a href="../reference/api/">Go packages</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Published · PyPI</span>
  <h3>Python</h3>
  <p><code>pip install opensysml</code> — the package downloads and verifies the matching
  <code>sysml-grpc</code> binary itself, so nothing else needs installing. Generated typed
  classes, signed releases.</p>
  <p class="osml-eco__links"><a href="../reference/python-api/">Python API</a> · <a href="https://pypi.org/project/opensysml/">PyPI</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Not yet published</span>
  <h3>Node / TypeScript</h3>
  <p>The <code>@openmbee/opensysml</code> client, including a browser entry point. Pin a release
  tag or build from <code>client/node</code> until it reaches npm.</p>
  <p class="osml-eco__links"><a href="../reference/clients/">Choosing a client</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Not yet published</span>
  <h3>Java</h3>
  <p>The Java client library. Pin a release tag or build from source until it reaches
  Maven Central.</p>
  <p class="osml-eco__links"><a href="../reference/clients/">Choosing a client</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Not yet published</span>
  <h3>Rust</h3>
  <p>The Rust client library. Pin a release tag or build from source until it reaches
  crates.io.</p>
  <p class="osml-eco__links"><a href="../reference/clients/">Choosing a client</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Not yet published</span>
  <h3>Julia &amp; MATLAB</h3>
  <p>The Julia package and the MATLAB toolbox. Build from source until they reach a
  registry.</p>
  <p class="osml-eco__links"><a href="../reference/clients/">Choosing a client</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">LSP</span>
  <h3>Editors</h3>
  <p><code>sysml-lsp</code> speaks plain Language Server Protocol — VS Code has a prebuilt
  extension, and the same server works in any LSP-aware editor.</p>
  <p class="osml-eco__links"><a href="../guide/08-editors/">Editors</a> · <a href="../reference/lsp/">LSP extensions</a></p>
</div>
</div>
