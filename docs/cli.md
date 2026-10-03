---
description: The sysml command line — validator, analysis runner, interactive REPL, behavior engine, RDF exporter and document generator. Try the engine live in your browser.
---

# The `sysml` command line

One binary is the whole toolchain: validator, analysis runner, interactive REPL,
behavior engine, RDF exporter and document generator. The same runtime sits behind
`sysml-lsp` for editors and `sysml-grpc` for programs. Pick a tab to see it work.

<style>
  .osml-terminal {
    position: relative;
    background: #181d45;
    border: 1px solid rgba(255, 255, 255, .18);
    border-radius: .35rem;
    overflow: hidden;
    box-shadow: 0 .6rem 1.6rem rgba(0, 0, 0, .3);
  }
  .osml-terminal__bar {
    display: flex;
    align-items: center;
    gap: .28rem;
    padding: .4rem .55rem;
    background: linear-gradient(to bottom, #f4f4f4, #dcdcdc);
    border-bottom: 1px solid #b5b5b5;
  }
  .osml-terminal__btn {
    position: relative;
    width: .8rem;
    height: .8rem;
    border: 1px solid #a6a6a6;
    border-radius: .08rem;
    background: linear-gradient(to bottom, #fdfdfd, #e4e4e4);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, .7);
  }
  .osml-terminal__btn--min::after {
    content: "";
    position: absolute;
    left: 22%;
    right: 22%;
    bottom: 26%;
    height: .09rem;
    background: #4d4d4d;
  }
  .osml-terminal__btn--max::after {
    content: "";
    position: absolute;
    left: 24%;
    right: 24%;
    top: 24%;
    bottom: 24%;
    border: .09rem solid #4d4d4d;
    border-radius: 1px;
  }
  .osml-terminal__btn--close::before,
  .osml-terminal__btn--close::after {
    content: "";
    position: absolute;
    left: 22%;
    right: 22%;
    top: 47%;
    height: .09rem;
    background: #4d4d4d;
  }
  .osml-terminal__btn--close::before { transform: rotate(45deg); }
  .osml-terminal__btn--close::after  { transform: rotate(-45deg); }
  .osml-terminal__title {
    flex: 1;
    margin-left: .35rem;
    font-family: var(--md-text-font);
    font-size: .62rem;
    font-weight: bold;
    letter-spacing: 0;
    text-align: left;
    color: #3f3f3f;
    opacity: 1;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .osml-terminal__panes {
    display: grid;
    height: 22rem;
  }
  .osml-pane {
    grid-area: 1 / 1;
    position: relative;
    display: flex;
    flex-direction: column;
    min-height: 0;
    visibility: hidden;
    opacity: 0;
    transition: opacity .35s;
  }
  .osml-pane.is-active {
    visibility: visible;
    opacity: 1;
  }
  .osml-terminal pre {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: .9rem 1.4rem 3rem;
    font-size: .62rem;
    line-height: 1.65;
    color: #e8eaf6;
    overflow: auto;
    scrollbar-width: none;
    background: transparent;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .osml-terminal pre > code {
    display: block;
    padding: 0;
    background: transparent;
    box-shadow: none;
    color: inherit;
  }
  .osml-terminal .md-code__nav {
    display: none;
  }
  .osml-terminal pre::-webkit-scrollbar {
    display: none;
  }
  .osml-terminal .osml-prompt { color: #90caf9; }
  .osml-terminal .osml-ok     { color: #a5d6a7; }
  .osml-terminal .osml-err    { color: #ef9a9a; }
  .osml-terminal .osml-dim    { opacity: .68; }
  .osml-terminal .osml-hl     { color: #ffe082; }
  .md-typeset ul.osml-showcase__tabs {
    position: absolute;
    right: 0;
    bottom: 0;
    left: 0;
    display: flex;
    align-items: stretch;
    gap: .5rem;
    height: 1.65rem;
    margin: 0;
    padding: 0 .7rem;
    list-style: none;
    background: #245c2a;
    border-top: 1px solid rgba(255, 255, 255, .14);
    counter-reset: osmltab;
    overflow-x: auto;
    overflow-y: hidden;
    white-space: nowrap;
    font-family: var(--md-code-font-family, monospace);
  }
  .md-typeset .osml-showcase__tabs > li {
    display: flex;
    align-items: center;
    margin: 0;
    list-style: none;
    counter-increment: osmltab;
  }
  .md-typeset .osml-showcase__tabs > li::before {
    content: counter(osmltab) ":";
    margin-right: .15em;
    font-size: .56rem;
    color: rgba(214, 236, 216, .75);
  }
  .osml-showcase__tab {
    padding: 0;
    border: none;
    border-radius: 0;
    background: transparent;
    color: #d6ecd8;
    font: inherit;
    font-size: .58rem;
    line-height: 1.65rem;
    cursor: pointer;
    transition: color .2s;
  }
  .osml-showcase__tab:hover,
  .osml-showcase__tab:focus-visible {
    color: #fff;
  }
  .osml-showcase__tab[aria-selected="true"] {
    color: #fff;
    font-weight: bold;
  }
  .osml-showcase__tab[aria-selected="true"]::after {
    content: "*";
  }
  .osml-pane__more {
    position: absolute;
    right: .7rem;
    bottom: 2.15rem;
    padding: .3rem .7rem;
    border: 1px solid rgba(255, 255, 255, .25);
    border-radius: 1rem;
    background: rgba(20, 24, 60, .82);
    font-size: .62rem;
    color: #fff;
    opacity: .85;
    text-decoration: underline;
    transition: opacity .2s, border-color .2s;
  }
  .osml-pane__more:hover,
  .osml-pane__more:focus {
    opacity: 1;
    border-color: #fff;
  }
  .osml-live__editor {
    position: relative;
    border: 1px solid rgba(255, 255, 255, .18);
    border-bottom: none;
    border-radius: .35rem .35rem 0 0;
    background: #10132e;
    overflow: hidden;
  }
  .osml-live__filename {
    display: block;
    padding: .3rem .7rem;
    font-size: .58rem;
    letter-spacing: .04em;
    color: rgba(232, 234, 246, .55);
    border-bottom: 1px solid rgba(255, 255, 255, .1);
  }
  .osml-live__editor textarea {
    display: block;
    width: 100%;
    min-height: 11rem;
    padding: .6rem .8rem;
    background: transparent;
    border: none;
    outline: none;
    resize: vertical;
    color: #e8eaf6;
    font-family: var(--md-code-font-family, monospace);
    font-size: .6rem;
    line-height: 1.6;
  }
  .osml-live .osml-terminal {
    border-radius: 0 0 .35rem .35rem;
  }
  .osml-live__row {
    display: flex;
    align-items: center;
    gap: .6rem;
    padding: .45rem .7rem;
    border-top: 1px solid rgba(255, 255, 255, .12);
    background: rgba(255, 255, 255, .05);
  }
  .osml-live__prompt {
    display: flex;
    align-items: center;
    flex: 1;
    font-size: .62rem;
    color: #e8eaf6;
  }
  .osml-live__prompt input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: #e8eaf6;
    font: inherit;
    font-family: var(--md-code-font-family, monospace);
  }
  .osml-live__prompt input::placeholder {
    color: rgba(232, 234, 246, .4);
  }
  @media (prefers-reduced-motion: reduce) {
    .osml-pane { transition: none; }
  }
</style>

<div class="osml-showcase" data-osml-cli-showcase>
<div class="osml-terminal">
<div class="osml-terminal__bar"><span class="osml-terminal__title" data-osml-title>sysml — REPL</span>
<span class="osml-terminal__btn osml-terminal__btn--min"></span><span class="osml-terminal__btn osml-terminal__btn--max"></span><span class="osml-terminal__btn osml-terminal__btn--close"></span></div>
<div class="osml-terminal__panes">

<div class="osml-pane is-active" id="osml-pane-analysis" role="tabpanel" aria-labelledby="osml-tab-analysis" data-title="sysml — running an analysis">
<pre><code><span class="osml-prompt">$</span> sysml -quiet -analysis DeltaVBudget::saturnIBAscent delta-v-budget.sysml
<span class="osml-ok">✓ package DeltaVBudget</span>
<span class="osml-ok">✓ DeltaVBudget::saturnIBAscent</span>
  stage1DeltaV = 3037.6966629706967 [SI::'m/s']
  stage2DeltaV = 6432.955324716369 [SI::'m/s']
  margin = 70.65198768706614 [SI::'m/s']
  <span class="osml-ok">objective reachesOrbit: satisfied</span>
  <span class="osml-dim">standing: value (observed: 1 run under reverse)</span>

<span class="osml-prompt">$</span> sysml -quiet -analysis "DeltaVBudget::saturnIBAscent(<span class="osml-hl">required = 9800 ['m/s']</span>)" delta-v-budget.sysml
<span class="osml-ok">✓ package DeltaVBudget</span>
<span class="osml-err">✗ DeltaVBudget::saturnIBAscent(required = 9800 ['m/s'])</span>
  <span class="osml-dim">…</span>
  margin = -329.34801231293386 [SI::'m/s']
  <span class="osml-err">objective reachesOrbit: not satisfied: margin &gt; 0 ['m/s']</span>
  <span class="osml-dim">standing: value (observed: 1 run under reverse)</span>
</code></pre>
<a class="osml-pane__more" href="https://github.com/Open-MBEE/OpenSysML/blob/main/examples/runtime-showcase/README.md">The runtime showcase, with this model</a>
</div>

<div class="osml-pane" id="osml-pane-found" role="tabpanel" aria-labelledby="osml-tab-found" data-title="sysml — what only running it finds">
<pre><code><span class="osml-prompt">$</span> sysml -quiet -validate mass-rollup.sysml
<span class="osml-ok">✓ package MassRollup</span>
<span class="osml-ok">✓ mass-rollup.sysml: no errors</span>

<span class="osml-prompt">$</span> sysml -quiet -e MassRollup::saturnVWithIU.totalMass mass-rollup.sysml
<span class="osml-ok">✓ package MassRollup</span>
<span class="osml-err">sysml: evaluation failed: feature value saturnVWithIU.totalMass: feature value InstrumentUnit.totalMass: no value for feature mass</span>

<span class="osml-prompt">$</span> sysml -quiet -calc "DeltaVBudget::InjectionDeltaV(3.986E14 [SI::N], 6563000 [SI::m], 384400000 [SI::m])" delta-v-budget.sysml
<span class="osml-ok">✓ package DeltaVBudget</span>
<span class="osml-err">sysml: calc invocation failed: calc DeltaVBudget::InjectionDeltaV: result: type mismatch: cannot write 3135.1638390999387 [kg**0.5/s] (dimension M^0.5·T^-1) to a feature typed by SpeedValue (dimension L·T^-1)</span>
  <span class="osml-dim">standing: not covered (…)</span>
</code></pre>
<a class="osml-pane__more" href="https://github.com/Open-MBEE/OpenSysML/blob/main/examples/runtime-showcase/README.md">Every one of these, reproduced</a>
</div>

<div class="osml-pane" id="osml-pane-repl" role="tabpanel" aria-labelledby="osml-tab-repl" data-title="sysml — REPL">
<pre><code><span class="osml-prompt">sysml&gt;</span> part def Wheel { attribute diameter = 16.0; }
<span class="osml-ok">✓ part def Wheel</span>

<span class="osml-prompt">sysml&gt;</span> %instantiate Wheel
<span class="osml-ok">✓ Created instance of Wheel</span>
  <span class="osml-dim">ID: 1</span>

<span class="osml-prompt">sysml&gt;</span> %features Wheel
Instance: Wheel (ID: 1)
Features:
  diameter = 16.0
  <span class="osml-dim">…</span>

<span class="osml-prompt">sysml&gt;</span> calc add { in x; in y; x + y }
<span class="osml-ok">✓ calc add</span>

<span class="osml-prompt">sysml&gt;</span> %calc add 10 20
<span class="osml-ok">✓ add(10, 20)</span>
  <span class="osml-dim">= 30</span>
  <span class="osml-dim">standing: value (observed: 1 run under reverse)</span>
</code></pre>
<a class="osml-pane__more" href="../guide/04-repl/">The REPL, in the guide</a>
</div>

<div class="osml-pane" id="osml-pane-docs" role="tabpanel" aria-labelledby="osml-tab-docs" data-title="sysml — documents from the model">
<pre><code><span class="osml-prompt">$</span> sysml rover.sysml -render-document Hello::RoverReport
<span class="osml-ok">✓ package Hello</span>
<span class="osml-hl"># Rover Parts</span>

The rover's top-level parts, from the model:

<span class="osml-dim">&lt;!-- caption --&gt;</span>
<span class="osml-dim">*Top-level parts*</span>

| name |
| --- |
| arm |
| chassis |
| mast |

<span class="osml-prompt">$</span> sysml rover.sysml -render-document Hello::RoverReport <span class="osml-hl">-doc-form html</span> -o rover.html
<span class="osml-ok">✓ package Hello</span>
wrote rover.html (html, 4983 bytes)   <span class="osml-dim"># or -doc-form pdf</span>
</code></pre>
<a class="osml-pane__more" href="../manual/">The document generation manual</a>
</div>

<div class="osml-pane" id="osml-pane-check" role="tabpanel" aria-labelledby="osml-tab-check" data-title="sysml — checking a model">
<pre><code><span class="osml-prompt">$</span> sysml -validate vehicles.sysml
<span class="osml-err">vehicles.sysml:6:16: error: unresolved reference: Whel — did you mean Wheel?</span>
        part spare : Whel;
                     <span class="osml-err">^~~~</span>
sysml: vehicles.sysml did not analyse cleanly; no check was made

<span class="osml-prompt">$</span> echo $?
2

<span class="osml-prompt">$</span> sed -i 's/Whel/Wheel/' vehicles.sysml &amp;&amp; sysml -validate vehicles.sysml
<span class="osml-ok">✓ package Vehicles</span>
<span class="osml-ok">✓ vehicles.sysml: no errors</span>
</code></pre>
<a class="osml-pane__more" href="../guide/05-checking/">Checking a model, in the guide</a>
</div>

<div class="osml-pane" id="osml-pane-run" role="tabpanel" aria-labelledby="osml-tab-run" data-title="sysml — running a state machine">
<pre><code><span class="osml-prompt">sysml&gt;</span> state def TrafficLight {
<span class="osml-prompt">  ...&gt;</span>     entry; then green;
<span class="osml-prompt">  ...&gt;</span>     state green;  accept after 25 [SI::s] then yellow;
<span class="osml-prompt">  ...&gt;</span>     state yellow; accept after 5 [SI::s] then red;
<span class="osml-prompt">  ...&gt;</span>     state red;    accept after 30 [SI::s] then done;
<span class="osml-prompt">  ...&gt;</span> }
<span class="osml-ok">✓ state def TrafficLight</span>

<span class="osml-prompt">sysml&gt;</span> %state TrafficLight
<span class="osml-ok">✓ Started state machine executor for "TrafficLight"</span>
  <span class="osml-dim">Current state: green</span>

<span class="osml-prompt">sysml&gt;</span> %advance 25
<span class="osml-ok">✓ Advanced to 25.0 (1 event(s) processed)</span>
  <span class="osml-dim">Current state: yellow</span>

<span class="osml-prompt">sysml&gt;</span> %advance 35
<span class="osml-ok">✓ Advanced to 60.0 (2 event(s) processed)</span>
<span class="osml-ok">✓ State machine completed (a transition reached `done`)</span>
</code></pre>
<a class="osml-pane__more" href="../guide/06-behavior/">Executing behavior, in the guide</a>
</div>

<div class="osml-pane" id="osml-pane-rdf" role="tabpanel" aria-labelledby="osml-tab-rdf" data-title="sysml — the model as RDF">
<pre><code><span class="osml-prompt">$</span> sysml vehicles.sysml -convert ttl -o vehicles.ttl
<span class="osml-dim">note: RDF conversion is experimental: the mapping covers model structure and the behavior its bodies state, refuses what it cannot write back, …</span>
wrote vehicles.ttl (ttl, 5562 bytes)
<span class="osml-prompt">$</span> grep -A5 '^elmt:Vehicles__Wheel$' vehicles.ttl
<span class="osml-hl">elmt:Vehicles__Wheel</span>
    a sysml:PartDefinition ;
    sysml:qualifiedName "Vehicles::Wheel" ;
    sysml:elementId "Vehicles__Wheel" ;
    sysx:memberIndex "1"^^xsd:integer ;
    sysml:owningNamespace elmt:Vehicles ;

<span class="osml-prompt">$</span> sysml vehicles.ttl -convert sysml   <span class="osml-dim"># and back again</span>
<span class="osml-dim">note: RDF conversion is experimental: …</span>
package Vehicles {
    part def Wheel;
  <span class="osml-dim">…</span>

<span class="osml-prompt">$</span> sysml -query 'oslc.where=sysml:name="wheels"' vehicles.sysml
<span class="osml-ok">✓ package Vehicles</span>
Vehicles::Car::wheels  PartUsage
</code></pre>
<a class="osml-pane__more" href="../guide/07-saving-and-rdf/">Saving and RDF, in the guide</a>
</div>

<div class="osml-pane" id="osml-pane-python" role="tabpanel" aria-labelledby="osml-tab-python" data-title="python — the opensysml client">
<pre><code><span class="osml-prompt">$</span> pip install opensysml
<span class="osml-prompt">$</span> python
<span class="osml-prompt">&gt;&gt;&gt;</span> import opensysml
<span class="osml-prompt">&gt;&gt;&gt;</span> model = opensysml.load("vehicle.sysml")
<span class="osml-prompt">&gt;&gt;&gt;</span> model.ok
<span class="osml-ok">True</span>
<span class="osml-prompt">&gt;&gt;&gt;</span> model.eval("mass", subject="Demo::sedan")
<span class="osml-ok">1800.0</span>
<span class="osml-prompt">&gt;&gt;&gt;</span> vehicle = model.get("Demo::Vehicle")
<span class="osml-prompt">&gt;&gt;&gt;</span> vehicle.kind, vehicle.id
<span class="osml-ok">('partDef', 'Demo::Vehicle')</span>
<span class="osml-prompt">&gt;&gt;&gt;</span> built = model.instantiate("Demo::Vehicle")
<span class="osml-prompt">&gt;&gt;&gt;</span> built.mass
<span class="osml-ok">1500.0</span>
</code></pre>
<a class="osml-pane__more" href="../guide/09-clients/">The clients, in the guide</a>
</div>

        </div>
<ul class="osml-showcase__tabs" role="tablist" aria-label="What the toolchain does">
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-analysis" aria-controls="osml-pane-analysis" aria-selected="true">analysis</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-found" aria-controls="osml-pane-found" aria-selected="false" tabindex="-1">found</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-repl" aria-controls="osml-pane-repl" aria-selected="false" tabindex="-1">repl</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-docs" aria-controls="osml-pane-docs" aria-selected="false" tabindex="-1">docs</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-check" aria-controls="osml-pane-check" aria-selected="false" tabindex="-1">check</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-run" aria-controls="osml-pane-run" aria-selected="false" tabindex="-1">run</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-rdf" aria-controls="osml-pane-rdf" aria-selected="false" tabindex="-1">rdf</button></li>
  <li role="presentation"><button class="osml-showcase__tab" type="button" role="tab" id="osml-tab-python" aria-controls="osml-pane-python" aria-selected="false" tabindex="-1">python</button></li>
</ul>
      </div>
</div>
<script>
(function () {
  var DWELL = 4500;      // ms a pane stays once printed, before the next comes up
  var TYPE_MS = 14;      // per character of a command, plus up to as much again of jitter
  var LINE_MS = 30;      // per line of output
  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

  // Refills the pane's text nodes in place: commands typed per character, output per line;
  // the <pre> follows the tail until the reader scrolls it.
  function printer(pane) {
    var pre = pane.querySelector("pre");
    var code = pane.querySelector("pre > code");
    var nodes = [];
    var timer = null;
    var follow = true;
    if (code) {
      var walker = document.createTreeWalker(code, NodeFilter.SHOW_TEXT, null, false);
      var node;
      while ((node = walker.nextNode())) {
        nodes.push({ node: node, text: node.data, prompt: !!node.parentNode.closest(".osml-prompt") });
      }
    }

    function unfollow() { follow = false; }
    if (pre) {
      pre.addEventListener("wheel", unfollow, { passive: true });
      pre.addEventListener("touchmove", unfollow, { passive: true });
    }

    function restore() {
      nodes.forEach(function (n) { n.node.data = n.text; });
    }

    function cancel() {
      if (timer) { clearTimeout(timer); timer = null; }
      pane.classList.remove("is-printing");
      restore();
    }

    function run(done) {
      cancel();
      if (reduceMotion.matches || !nodes.length) { done(); return; }
      nodes.forEach(function (n) { n.node.data = ""; });
      pre.scrollTop = 0;
      pre.scrollLeft = 0;
      follow = true;
      pane.classList.add("is-printing");
      var i = 0, pos = 0, typing = false;

      function step() {
        if (i >= nodes.length) {
          timer = null;
          pane.classList.remove("is-printing");
          done();
          return;
        }
        var n = nodes[i], delay;
        if (n.prompt) {
          n.node.data = n.text;
          pos = n.text.length;
          typing = true;
          delay = TYPE_MS * 6;
        } else if (typing) {
          var ch = n.text.charAt(pos++);
          n.node.data += ch;
          if (ch === "\n") typing = false;
          delay = TYPE_MS + Math.random() * TYPE_MS;
        } else {
          var end = n.text.indexOf("\n", pos);
          end = end < 0 ? n.text.length : end + 1;
          n.node.data += n.text.slice(pos, end);
          pos = end;
          delay = LINE_MS;
        }
        if (pos >= n.text.length) { i++; pos = 0; }
        if (follow) pre.scrollTop = pre.scrollHeight;
        timer = setTimeout(step, delay);
      }
      step();
    }

    return { run: run, cancel: cancel };
  }

  function setUp(root) {
    var tabs = Array.prototype.slice.call(root.querySelectorAll("[role=tab]"));
    var panes = Array.prototype.slice.call(root.querySelectorAll("[role=tabpanel]"));
    var printers = panes.map(printer);
    var title = root.querySelector("[data-osml-title]");
    var current = 0;
    var timer = null;
    var autoplay = false;

    function show(index, focus) {
      printers[current].cancel();
      current = (index + panes.length) % panes.length;
      panes.forEach(function (pane, i) {
        pane.classList.toggle("is-active", i === current);
      });
      tabs.forEach(function (tab, i) {
        var active = i === current;
        tab.setAttribute("aria-selected", active ? "true" : "false");
        tab.tabIndex = active ? 0 : -1;
        if (active && focus) tab.focus();
      });
      if (title) title.textContent = panes[current].dataset.title || "";
      printers[current].run(function () { schedule(DWELL); });
    }

    function stop() {
      autoplay = false;
      if (timer) { clearTimeout(timer); timer = null; }
    }

    function schedule(delay) {
      if (timer) clearTimeout(timer);
      timer = null;
      if (!autoplay || reduceMotion.matches) return;
      timer = setTimeout(advance, delay);
    }

    function advance() {
      timer = null;
      if (!root.isConnected) { stop(); return; }
      if (root.matches(":hover") || root.contains(document.activeElement)) { schedule(1000); return; }
      show(current + 1, false);
    }

    tabs.forEach(function (tab, i) {
      tab.addEventListener("click", function () { stop(); show(i, false); });
      tab.addEventListener("keydown", function (event) {
        var targets = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: panes.length - 1 };
        if (typeof targets[event.key] !== "number") return;
        event.preventDefault();
        stop();
        show(targets[event.key], true);
      });
    });

    autoplay = !reduceMotion.matches;
    show(0, false);
  }

  Array.prototype.forEach.call(document.querySelectorAll("[data-osml-cli-showcase]"), setUp);
})();
</script>







<h2>Run it in your browser</h2>
<p>The same engine compiles to WebAssembly — this is a real <code>sysml</code>
session running in the page, no server involved. Edit the model, <code>%parse</code> it,
then evaluate, instantiate and execute against it.</p>
<div class="osml-live">
<div class="osml-live__editor">
<span class="osml-live__filename">model.sysml</span>
<textarea id="osml-live-src" spellcheck="false">package gatedemo {
	private import ScalarValues::*;

	part def Widget {
		attribute mass : Real = 3.0;
	}

	attribute total : Real = 6.0;

	action greet { }
}
</textarea>
</div>
<div class="osml-terminal osml-live">
<div class="osml-terminal__bar"><span class="osml-terminal__title">sysml-engine — wasm, in this page</span>
<span class="osml-terminal__btn osml-terminal__btn--min"></span><span class="osml-terminal__btn osml-terminal__btn--max"></span><span class="osml-terminal__btn osml-terminal__btn--close"></span></div>
<pre class="osml-live__out"><code><span class="osml-dim">$ sysml model.sysml — engine not loaded</span>
<span class="osml-dim">  press "load engine" — downloads ~7 MB once, then runs entirely here</span>
</code></pre>
<div class="osml-live__row">
<button class="osml-showcase__tab" id="osml-live-load" type="button">Load engine</button>
<span class="osml-live__prompt"><span class="osml-prompt">sysml&gt;</span>&nbsp;<input id="osml-live-in" type="text" placeholder="%help for commands — or just an expression like gatedemo::total" disabled autocomplete="off" spellcheck="false"></span>
</div>
</div>
</div>
<script>
(function () {
  var out = document.querySelector('.osml-live__out');
  var input = document.getElementById('osml-live-in');
  var src = document.getElementById('osml-live-src');
  var load = document.getElementById('osml-live-load');
  var modelHash = null;
  var esc = function (s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); };
  function println(html) {
    var code = out.querySelector('code');
    code.innerHTML = code.innerHTML.replace(/\n$/, '') + '\n' + html + '\n';
    out.scrollTop = out.scrollHeight;
  }
  function call(method, params) {
    return JSON.parse(globalThis.sysmlEngine.call(method, JSON.stringify(params)));
  }
  function parse() {
    var t0 = performance.now();
    var parsed = call('ParseSources', { documents: [{ name: 'model.sysml', content: src.value }] });
    if (parsed.error) { println('<span class="osml-err">✗ ' + esc(JSON.stringify(parsed.error)) + '</span>'); return; }
    modelHash = parsed.result && parsed.result.modelHash;
    var diags = (parsed.result && parsed.result.diagnostics) || [];
    println('<span class="osml-ok">✓ model.sysml</span> <span class="osml-dim">parsed in ' +
      (performance.now() - t0).toFixed(0) + ' ms · ' + diags.length + ' diagnostics · hash ' +
      esc(String(modelHash).slice(0, 12)) + '…</span>');
    diags.slice(0, 8).forEach(function (d) {
      println('  <span class="osml-err">' + esc(d.message || JSON.stringify(d)) + '</span>');
    });
  }
  var commands = {
    '%help': function () {
      println('  <span class="osml-dim">%parse — parse the model source · %eval &lt;expr&gt; or a bare expression · %instantiate &lt;symbol&gt; · %run &lt;action&gt; · %state &lt;state machine&gt; · %reset</span>');
    },
    '%parse': parse,
    '%reset': function () { modelHash = null; println('<span class="osml-dim">model dropped</span>'); },
    '%instantiate': function (sym) {
      if (!need(sym)) return;
      var r = call('Instantiate', { modelHash: modelHash, symbolId: sym });
      var res = r.result || {};
      if (res.error) { println('<span class="osml-err">✗ ' + esc(res.error) + '</span>'); return; }
      var inst = res.instance || {};
      var feats = Object.keys(inst.featureValues || {});
      println('<span class="osml-ok">✓ instance ' + esc(inst.id == null ? '?' : inst.id) + '</span> of <span class="osml-hl">' + esc(inst.typeSymbolId || sym) + '</span>' +
        (feats.length ? ' <span class="osml-dim">— ' + feats.length + ' feature values: ' + feats.slice(0, 6).map(esc).join(', ') + (feats.length > 6 ? ', …' : '') + '</span>' : ''));
    },
    '%run': function (sym) {
      if (!need(sym)) return;
      var r = call('ExecuteAction', { modelHash: modelHash, actionSymbolId: sym });
      var res = r.result || {};
      if (res.error) { println('<span class="osml-err">✗ ' + esc(res.error) + '</span>'); return; }
      println('<span class="osml-ok">✓ ran</span> ' + esc(sym) +
        ' <span class="osml-dim">— finalTime ' + (res.finalTime || 0) + ', ' + Object.keys(res.outputs || {}).length + ' outputs</span>');
    },
    '%state': function (sym) {
      if (!need(sym)) return;
      var r = call('ExecuteState', { modelHash: modelHash, stateMachineSymbolId: sym });
      var res = r.result || {};
      if (res.error) { println('<span class="osml-err">✗ ' + esc(res.error) + '</span>'); return; }
      println('<span class="osml-ok">✓ visited</span> ' + esc((res.statesVisited || []).join(' → ') || '(none)') +
        ' <span class="osml-dim">— finalTime ' + (res.finalTime || 0) + '</span>');
    }
  };
  function need(x) {
    if (!modelHash) { println('<span class="osml-err">✗ no model — %parse first</span>'); return false; }
    if (!x) { println('<span class="osml-err">✗ needs a symbol, e.g. gatedemo::Widget</span>'); return false; }
    return true;
  }

  load.addEventListener('click', function () {
    load.disabled = true;
    println('<span class="osml-dim">loading sysml-engine.wasm (~7 MB)…</span>');
    var s = document.createElement('script');
    s.src = '/assets/wasm_exec.js';
    s.onload = function () {
      if (!('DecompressionStream' in window)) {
        println('<span class="osml-err">✗ this browser cannot inflate the module — try a current Chrome, Firefox or Safari</span>');
        load.disabled = false;
        return;
      }
      var go = new Go();
      var done = fetch('/assets/sysml-engine.wasm.gz')
        .then(function (r) { return r.body.pipeThrough(new DecompressionStream('gzip')); })
        .then(function (rs) { return new Response(rs).arrayBuffer(); })
        .then(function (b) { return WebAssembly.instantiate(b, go.importObject); });
      done.then(function (res) {
        go.run(res.instance);
        (function wait() {
          if (!globalThis.sysmlEngine) { setTimeout(wait, 120); return; }
          println('<span class="osml-ok">✓ engine loaded</span> — ' + esc(globalThis.sysmlEngine.version || 'unknown'));
          parse();
          input.disabled = false;
          input.focus();
        })();
      }).catch(function (e) {
        println('<span class="osml-err">✗ engine failed to load: ' + esc(e) + '</span>');
      });
    };
    document.body.appendChild(s);
  });

  input.addEventListener('keydown', function (e) {
    if (e.key !== 'Enter') return;
    var line = input.value.trim();
    if (!line) return;
    input.value = '';
    println('<span class="osml-prompt">sysml&gt;</span> ' + esc(line));
    var sp = line.indexOf(' ');
    var cmd = sp < 0 ? line : line.slice(0, sp);
    var arg = sp < 0 ? '' : line.slice(sp + 1).trim();
    var fn = commands[cmd];
    if (fn) { fn(arg); return; }
    if (cmd === '%eval') line = arg;
    if (!modelHash) { println('<span class="osml-err">✗ no model — %parse first</span>'); return; }
    var r = call('Evaluate', { modelHash: modelHash, expression: line });
    var res = r.result || {};
    if (res.error) {
      println('<span class="osml-err">✗ ' + esc(res.error) + '</span>');
    } else {
      println('  <span class="osml-ok">=</span> ' + esc(JSON.stringify(res.result)).replace(/"result":/, ''));
    }
  });
})();
</script>


## What it does

<div class="osml-eco__grid">
<div class="osml-eco__card">
  <span class="osml-eco__tag">-analysis</span>
  <h3>Run analyses</h3>
  <p>Execute a model's analysis cases — evaluated features with units carried and
  checked, objectives decided against what the run finds. The delta-v budget above
  is the <a href="https://github.com/Open-MBEE/OpenSysML/blob/develop/examples/runtime-showcase/README.md">runtime showcase</a>.</p>
  <p class="osml-eco__links"><a href="../guide/03-command-line/">The command line</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">sysml&gt;</span>
  <h3>Explore in a REPL</h3>
  <p>Parse, instantiate, evaluate and inspect a live session — every <code>%</code> command and
  its arguments, plus what keeps your model across restarts.</p>
  <p class="osml-eco__links"><a href="../guide/04-repl/">The REPL</a> · <a href="../reference/repl-commands/">REPL commands</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">-check</span>
  <h3>Validate and check</h3>
  <p>Name resolution, typing and constraint tiers — and what a validator alone cannot
  reach, which the runtime modes below do.</p>
  <p class="osml-eco__links"><a href="../guide/05-checking/">Checking models</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">-run</span>
  <h3>Run behavior</h3>
  <p>Actions, state machines, calculations, analyses and requirements, run on a clock —
  with a step budget if a model does not finish on its own.</p>
  <p class="osml-eco__links"><a href="../guide/06-behavior/">Behavior</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">-convert</span>
  <h3>Export RDF</h3>
  <p>Write a model out as Turtle for graph stores — the mapping, what is not mapped,
  and why the experimental flag is honest.</p>
  <p class="osml-eco__links"><a href="../guide/07-saving-and-rdf/">Saving and RDF</a> · <a href="../reference/rdf-mapping/">RDF mapping</a></p>
</div>
<div class="osml-eco__card">
  <span class="osml-eco__tag">-render-document</span>
  <h3>Generate documents</h3>
  <p>Markdown, HTML and PDF documents driven by the model's own document queries —
  the full manual is a chapter of its own.</p>
  <p class="osml-eco__links"><a href="../manual/">Document generation manual</a></p>
</div>
</div>

## Reference

- **[CLI](reference/cli.md)** — every flag, the modes, and the exit codes
- **[REPL commands](reference/repl-commands.md)** — every `%` command and its arguments
- **[Environment variables](reference/environment.md)** — resource limits for a single run
