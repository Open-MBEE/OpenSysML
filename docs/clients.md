---
description: OpenSysML client libraries — the Go, Python, Node, Java, Rust, Julia and MATLAB APIs that talk to the same runtime behind sysml, sysml-lsp and sysml-grpc.
---

# Client libraries

The same runtime drives `sysml`, `sysml-lsp` and `sysml-grpc`. The clients below talk
to that runtime — most through the `sysml-grpc` service, so a client gets exactly what
the CLI gets, over the wire. See [client selection](reference/clients.md) to compare
the libraries and [service transports](reference/service-transports.md) for what the
service serves.

<div class="osml-eco__grid">
<div class="osml-eco__card">
  <span class="osml-eco__tag">Published · go install</span>
  <h3>Go</h3>
  <p>The native client — the runtime's own language, so every type is the real one.
  <code>go install github.com/Open-MBEE/OpenSysML/cmd/sysml@latest</code>, or import
  <code>client/opensysml</code>.</p>
  <p class="osml-eco__links"><a href="../reference/api/">Go packages</a> ·
  <a href="../guide/09-clients/#from-go">Go walkthrough</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Published · PyPI</span>
  <h3>Python</h3>
  <p><code>pip install opensysml</code> — the package downloads and verifies the matching
  <code>sysml-grpc</code> binary itself, so nothing else needs installing. Generated typed
  classes, signed releases.</p>
  <p class="osml-eco__links"><a href="python/">Python client</a> ·
  <a href="../reference/python-api/">Python API</a> ·
  <a href="https://pypi.org/project/opensysml/">PyPI</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Published · npm</span>
  <h3>Node / TypeScript</h3>
  <p><code>npm install @openmbee/opensysml</code> installs the client and its per-platform
  <code>sysml-grpc</code> binary package.</p>
  <p class="osml-eco__links"><a href="node/">Node client</a> ·
  <a href="../reference/node-api/">Node API</a> ·
  <a href="https://www.npmjs.com/package/@openmbee/opensysml">npm</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Build from a checkout</span>
  <h3>Java</h3>
  <p>Not on Maven Central; build and install the Java client from a checkout.</p>
  <p class="osml-eco__links"><a href="java/">Java client</a> ·
  <a href="../reference/java-api/">Java API</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Published · crates.io</span>
  <h3>Rust</h3>
  <p><code>opensysml = "0.9"</code> installs the client from crates.io.</p>
  <p class="osml-eco__links"><a href="rust/">Rust client</a> ·
  <a href="../reference/rust-api/">Rust API</a> ·
  <a href="https://crates.io/crates/opensysml">crates.io</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Build from source</span>
  <h3>Julia</h3>
  <p>Not in Julia General; develop the package from a checkout.</p>
  <p class="osml-eco__links"><a href="julia/">Julia client</a> ·
  <a href="../reference/julia-api/">Julia API</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">Source package</span>
  <h3>MATLAB</h3>
  <p>Add the source package to the MATLAB or Octave path.</p>
  <p class="osml-eco__links"><a href="matlab/">MATLAB client</a> ·
  <a href="../reference/matlab-api/">MATLAB API</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">LSP</span>
  <h3>Editors</h3>
  <p><code>sysml-lsp</code> speaks plain Language Server Protocol — VS Code has a prebuilt
  extension, and the same server works in any LSP-aware editor.</p>
  <p class="osml-eco__links"><a href="../guide/08-editors/">Editors</a> · <a href="../reference/lsp/">LSP extensions</a></p>
</div>
</div>
