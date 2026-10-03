---
description: The OpenSysML stack and ecosystem — three independent SysML v2 implementations over a version-controlled model store, all open source.
---

# The stack and the ecosystem

Open source finally speaks SysML v2: three independently built implementations of the
OMG standard — a Go runtime, a Rust toolchain, and OMG's own reference build — reading
and writing the same models, with an open, version-controlled store underneath to hold
what they produce. No seat licenses. No vendor lock-in.

## The stack

Four projects, independently maintained, built against the same open interfaces — a
runtime, a toolchain, the reference implementation itself, and a model store to tie
them together.

<div class="osml-eco__grid">
  <div class="osml-eco__card">
    <p class="osml-eco__path">Open-MBEE / OpenSysML</p>
    <h3>OpenSysML</h3>
    <p class="osml-eco__role">The runtime</p>
    <p>A complete SysML v2 and KerML implementation in Go — language server,
    interactive REPL, and an execution runtime that instantiates parts, evaluates
    constraints, and runs action and state behavior on a clock, not just validates
    them. Ships with a Python client over gRPC.</p>
    <p class="osml-eco__meta">Go · Apache-2.0</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Open-MBEE/OpenSysML">GitHub</a> ·
      <a href="../guide/04-repl/">Try the runtime</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <p class="osml-eco__path">Open-MBEE / sysml-toolkit</p>
    <h3>sysml-toolkit</h3>
    <p class="osml-eco__role">The tool kit</p>
    <p>A Rust workspace covering parsing, formatting, linting, JSON/CBOR interchange,
    constraint solving, and span-preserving model transformation, plus a language
    server and Python and WebAssembly bindings. Built for programmatic pipelines and
    CI, not just editing.</p>
    <p class="osml-eco__meta">Rust · Apache-2.0</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Open-MBEE/sysml-toolkit">GitHub</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <p class="osml-eco__path">Systems-Modeling / SysML-v2-Pilot-Implementation</p>
    <h3>SysML v2 Pilot Implementation</h3>
    <p class="osml-eco__role">The reference</p>
    <p>The OMG Systems Modeling Community's own implementation — Eclipse Xtext
    editors, PlantUML visualization, and a Jupyter kernel. Both projects above track
    its grammar and standard library as their conformance baseline; this is the
    ground truth.</p>
    <p class="osml-eco__meta">Java / Xtext · EPL-2.0</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation">GitHub</a>
    </p>
  </div>
  <div class="osml-eco__card osml-eco__card--hub">
    <p class="osml-eco__path">Open-MBEE / flexo-mms-*</p>
    <h3>Flexo MMS</h3>
    <p class="osml-eco__role">The model store</p>
    <p>A collection of microservices forming a version-controlled home for model data
    whose native form is RDF — a model is a graph other tools can query, diff, and
    merge, not a file they have to reparse. One SPARQL 1.1 quadstore underneath, JSON
    Web Tokens between services, and a SysML v2 API layer on top, so it plugs into the
    same open interchange the tools above already speak.</p>
    <p class="osml-eco__meta">Kotlin / Scala · Apache-2.0 · multiple services</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Open-MBEE/flexo-mms.openmbee.org">GitHub</a>
    </p>
  </div>
</div>

## The wider ecosystem

OpenSysML isn't a single product — it's a stack plus the people, standards bodies, and
growing library ecosystem around it.

<div class="osml-eco__grid">
  <div class="osml-eco__card">
    <span class="osml-eco__tag">Docs · CC BY 4.0</span>
    <h3>OpenSysML Wiki</h3>
    <p>Guides, decisions, and working notes for the project — the place to read up
    before digging into any one tool in the stack. Content is CC BY 4.0, separate from
    the code licenses above.</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Open-MBEE/opensysml.github.io/wiki">Read the wiki</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <span class="osml-eco__tag">Standards body</span>
    <h3>OMG SysML</h3>
    <p>The Object Management Group's home for the SysML specification itself — the
    standard that every project in the stack implements.</p>
    <p class="osml-eco__links">
      <a href="https://www.omg.org/spec/SysML">omg.org/spec/SysML</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <span class="osml-eco__tag">Standards body</span>
    <h3>INCOSE</h3>
    <p>The International Council on Systems Engineering — the professional society
    behind much of the practice and methodology SysML v2 is built to support.</p>
    <p class="osml-eco__links">
      <a href="https://www.incose.org">incose.org</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <span class="osml-eco__tag">Parent community</span>
    <h3>OpenMBEE</h3>
    <p>The Open Model-Based Engineering Environment — open-source tooling for
    managing, integrating, and publishing systems models at scale, and the broader
    community OpenSysML grew out of.</p>
    <p class="osml-eco__links">
      <a href="https://www.openmbee.org">openmbee.org</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <span class="osml-eco__tag">Growing</span>
    <h3>Libraries built on OpenSysML</h3>
    <p>Parsers, exporters, editor integrations, and other open-source libraries built
    around SysML v2. Reach out through the wiki to get yours listed as this space
    grows.</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Open-MBEE/opensysml.github.io/wiki">What's listed so far</a>
    </p>
  </div>
  <div class="osml-eco__card">
    <span class="osml-eco__tag">Community</span>
    <h3>Join in</h3>
    <p>OpenSysML is community-run under OpenMBEE — code, models, docs and review are
    all welcome.</p>
    <p class="osml-eco__links">
      <a href="https://github.com/Open-MBEE/open-mbee.github.io/wiki/Participate-in-OpenMBEE-and-OpenSysML">Participate</a> ·
      <a href="https://numfocus.org/donate-to-openmbee">Sponsor</a>
    </p>
  </div>
</div>

## Licensing

- The OpenSysML stack uses a dual-license model: **Apache-2.0** for code and
  **CC BY 4.0** for wiki text, diagrams, and other written content.
- The one exception is the OMG reference implementation, which is licensed
  **EPL-2.0** rather than Apache-2.0.
- See the stack above for the specific license per project.
