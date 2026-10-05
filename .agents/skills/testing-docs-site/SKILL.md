---
name: testing-docs-site
description: How to build and browser-test the MkDocs documentation site locally — genuine Material instant navigation, Back/forward restoration, palette toggle and responsive landing-page behavior — without changing repository configuration.
---

# Testing the documentation site in a browser

Use a virtualenv with `pip install -r docs-requirements.txt`. Do not change repository configuration
to serve a local preview: build to a directory outside the checkout and serve that with
`python -m http.server`.

For instant-navigation testing, the build's `site_url` must match the local origin, port included.
Material's sitemap resolver can rewrite protocol and hostname without rewriting the port, so a
production build served on a nonstandard local port can silently fall back to full navigation.

From the repository root, build through the MkDocs API with runtime overrides:

```python
from mkdocs.config import load_config
from mkdocs.commands.build import build
config = load_config(
    "mkdocs.yml",
    site_url="http://localhost:8766/",
    site_dir="<outside-checkout>/site-browser-test",
    strict=True,
)
build(config)
```

Serve that directory on the matching port. Verify instant navigation rather than assuming it: read
`performance.timeOrigin`, click an internal link through the UI, and read it again; it must not
change. Test browser Back too, and check visually that page-specific UI is removed and restored
without duplicates.

Use the header palette toggle for light and dark modes, and DevTools responsive mode for narrow
widths. Verify pixels, not just element presence.

If browser inspection tooling appears to alter `target="_blank"` attributes, repeat the external-link
tests from a fresh page without annotated DOM inspection before the click. Read-only CDP inspection
plus native mouse input distinguishes tooling interference from the built page's behavior.

When the change under test is in `overrides/` — a stylesheet or template — do not
assume `mkdocs serve` reloaded it. The live reload may keep serving the old asset
through a browser hard refresh. Restart the server, wait for the build and serving
messages, reload, and confirm the loaded file carries the intended rule before
diagnosing a CSS fix as ineffective. Do not change repository configuration solely
to make a preview work.

### The in-browser engines

The landing diagram and the CLI page's REPL load generated assets: `docs/assets/sysml-engine.wasm.gz`,
`sysml-repl.wasm.gz`, `wasm_exec.js` and `repl-examples/`. They are gitignored; if any is missing,
run `make docs-engine-assets` before building the site.

The REPL walkthroughs' inputs and expected output strings live in `docs/assets/repl-walkthroughs.json`.
Run each step through its button and check only newly appended terminal output. Wait for the input
queue to drain and `is-busy` to clear before sending the next command.

To check the SMT refusal, declare a valid constraint first, for example
`constraint def Positive { in x : ScalarValues::Integer; x > 0 }`, then run `%check Positive`. Expect
the message that a WebAssembly build cannot start external processes.

Completion considers every matching name, standard library included. `%inst` has several candidates;
`%instanti` has one. For a shared-prefix test, pick names that no library name shares.

At mobile width, navigate through the header menu. `label[for="__drawer"]` also matches a hidden
overlay, so scope automation to `header label[for="__drawer"]`. Compare the document's width with the
viewport's; a code textarea's own horizontal scrolling is not page overflow.

### Devin Secrets Needed

None for the public local site and public outbound link checks.
