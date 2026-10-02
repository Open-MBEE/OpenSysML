---
name: testing-engine-browser
description: How to build sysml-engine for js/wasm and exercise it in a real browser through an external static harness — JSON-RPC envelopes, typed values, error recovery after bad calls, and startup/transfer measurements.
---

# Testing `sysml-engine` in a browser

## Setup

- Read `docs/reference/wasm.md`, `cmd/sysml-engine/serve_js.go`, `internal/frontend/engine/json.go`
  and `tests/wasm/testdata/` first.
- Keep the harness page outside the checkout. The targeted build can live outside it too:
  `make build-wasm-js WASM_COMMANDS=sysml-engine WASM_DIR=<harness-dir>/build`.
- Serve the page with `python3 -m http.server 8765 --bind 127.0.0.1 --directory <harness-dir>`.
- Load the matching `build/js/wasm_exec.js`. Instantiate the module with `new Go().importObject` and
  `WebAssembly.instantiateStreaming`; check the server sends `application/wasm`, and fall back to
  `arrayBuffer` if it does not.
- Run with no extra argv. Do not await `go.run(instance)` before checking `globalThis.sysmlEngine`:
  the engine stays alive on purpose. Record promise rejection and unexpected resolution as runtime
  events.

## Browser controls and assertions

- Give the page editable source textareas for `model.sysml` and `engine.sysml`, and
  Parse/Evaluate/ExecuteAction/ExecuteState/Instantiate controls.
- Use the lowerCamelCase fields from `json.go`. ParseSources takes `{documents:[{name,content}]}`.
  Reuse the `modelHash` it returns; never hardcode one.
- `sysmlEngine.call(method, JSON.stringify(params))` synchronously returns a JSON-RPC envelope as a
  string, not the bare protojson result.
- Fixture expectations: total realValue 6; speed quantity 5 metres/second; `Color::red` enum; Double
  input `x:{intValue:"6"}` returns y `"12"`; Switch visits `"on"` and instantiates a graph.
- SysML sequences are written `(1, 2, 3)`, not `[1,2,3]`. A parity-only test can treat matching
  error payloads as equal, so assert actual values in browser checks.
- Probe unknown modelHash (5), unsupported method (12), `schedule:"explore"` (12), malformed params
  JSON (3) and invalid source (diagnostics with spans). After every error, reparse edited source and
  confirm a new hash and the updated value.
- To enter fixture text with literal tabs into a textarea, paste it through the clipboard; typed tab
  keystrokes move focus to the buttons. Plain spaces are safe to type.

## Evidence

- Open DevTools Network before the first navigation; record transfer size and response headers.
- Take `performance.now()` when the global first appears: that is navigation-to-ready time. Record
  the local HTTP compression and cache configuration; an uncompressed localhost transfer is not
  comparable with a production gzip budget.
- Keep raw request/response envelopes and per-call durations, but drive every call through the
  visible UI.
- Capture console errors, unhandled rejections, Go panic/exit events, and that calls keep working
  after the error probes.
- Record the build commit and the displayed version. A later checkout does not change a wasm file
  that is already being served.

### Devin Secrets Needed

None. The page runs the engine entirely inside the browser.
