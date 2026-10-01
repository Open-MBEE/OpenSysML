# WebAssembly builds

OpenSysML is Go, and Go compiles it for two WebAssembly targets. This page says what each is
for, how to build and run them, what works in them, and what a WebAssembly host cannot do —
with the message each limitation answers with, so a refusal is never mistaken for a defect.

No WebAssembly artifact ships in a release: releases are native binaries for Linux, macOS and
Windows. The WebAssembly builds are built from source, for a host that runs modules rather than
executables.

## Building

```bash
make build-wasm          # both targets
make build-wasm-wasip1   # or one
make build-wasm-js
```

The output is one directory per target, four commands each, stamped with the same version
information a native build carries:

```
bin/wasm/wasip1/{sysml,sysml-lsp,sysml-grpc,sysml-engine}.wasm
bin/wasm/js/{sysml,sysml-lsp,sysml-grpc,sysml-engine}.wasm
bin/wasm/js/wasm_exec.js      # the runtime a browser page includes
```

`wasm_exec.js` is the library, not a runner: a page loads it with a `<script>` tag, and Node
runs these modules with `$(go env GOROOT)/lib/wasm/wasm_exec_node.js`, which loads its own copy
beside it.

`make build` is unchanged and stays native; a WebAssembly build is always asked for.

## Running a WASI build

`wasip1` binaries run under any WASI preview 1 runtime. [Wasmtime](https://wasmtime.dev/) is
the one Go's own runner uses:

```bash
wasmtime run --dir=/ --env PWD="$PWD" bin/wasm/wasip1/sysml.wasm model.sysml -e 'total'
```

`--dir=/` preopens the filesystem root, which is how the working directory becomes reachable: a
runtime that preopens nothing can run `-version`, but can open no model. `--env PWD="$PWD"` is
what makes `model.sysml` resolve where you are — a `wasip1` program's `os.Getwd` is the `PWD` it
was given and nothing else, so without it the path resolves against `/`.

The Go toolchain ships the same runner as a script,
`$(go env GOROOT)/lib/wasm/go_wasip1_wasm_exec`: it opens `/`, supplies the working
directory and environment each runtime needs, and uses `wasmtime` unless `GOWASIRUNTIME` names
wasmer, wazero or wasmedge.

A host without one of those can use Node's WASI implementation directly:

```js
// wasi.mjs — node wasi.mjs <binary.wasm> [args...]
import { WASI } from 'node:wasi';
import fs from 'node:fs';

const [binary, ...args] = process.argv.slice(2);
const wasi = new WASI({
  version: 'preview1',
  args: [binary, ...args],
  // PWD is what a wasip1 program resolves relative paths against; process.cwd()
  // keeps it right where the shell does not export it.
  env: { ...process.env, PWD: process.cwd() },
  preopens: { '/': '/' },   // a deployment opens only the directories it needs
  returnOnExit: true,
});
const module = await WebAssembly.compile(fs.readFileSync(binary));
const instance = await WebAssembly.instantiate(module, wasi.getImportObject());
process.exit(wasi.start(instance));
```

Node's WASI host cannot block waiting for a pipe: a read from one with nothing ready comes back
`EAGAIN` rather than waiting. Drive its processes from a file, or from arguments — `-version`,
`-validate`, `-eval`, `-engines` and the prompt itself all work that way — and use a runtime that
reads pipes properly for anything that speaks a protocol over its standard input.

## Running a `js` build

`js` binaries run through the toolchain's `wasm_exec`, under Node or in a browser:

```bash
node --stack-size=8192 --no-concurrent-sparkplug "$(go env GOROOT)/lib/wasm/wasm_exec_node.js" bin/wasm/js/sysml.wasm -version
# the same, with the stack size the runner itself sets:
"$(go env GOROOT)/lib/wasm/go_js_wasm_exec" bin/wasm/js/sysml.wasm -version
```

Three constraints apply to `js` builds:

- **argv and the environment share about 8 KiB.** It writes both into linear memory at a fixed
  offset and refuses past `wasmMinDataAddr`; a large environment fails before `main` runs with
  `total length of command line and environment variables exceeds limit`. Run it with a small
  environment — `PATH`, `HOME`, `TMPDIR` are enough for these commands.
- **The stack needs raising.** Node's default JavaScript stack is too small for deep model
  traversal; `--stack-size=8192` is what Go's own runner passes.
- **Node can hang at exit.** A command that exits right after starting (`-version`, say)
  occasionally prints its output and then never exits: Node deadlocks while shutting down if its
  background compiler is still running. `--no-concurrent-sparkplug` avoids it.

In a browser, nothing wires a page's input to the module's standard input: an embedder provides
that itself. The commands take their input from arguments and files, so the parts that need no
interactive stream work as they do under Node.

## The execution engine

`sysml-engine` is the in-process half of [the service](service-transports.md): the execution
RPCs — `ParseSources`, `Evaluate`, `Instantiate`, `ExecuteAction`, `ExecuteState` — answered
with the same JSON `sysml-grpc` emits, but without the protobuf machinery, the analysis
framework or a held-object store, which is what keeps a WebAssembly build small enough to
embed in a page. A model parsed through it hashes to the same `modelHash` the service
returns, and a request a service client encodes decodes identically here.

The `js` build installs a host surface instead of reading a pipe: load it through
`wasm_exec.js` with no arguments and `globalThis.sysmlEngine` appears with

```js
const answer = JSON.parse(sysmlEngine.call(method, paramsJSON));
sysmlEngine.version;
```

where `call` is synchronous — it runs the method on the JS thread and returns the JSON-RPC
response envelope (`{"jsonrpc":"2.0","id":null,"result":…}` or `…"error":{"code":…,"message":…}}`)
as a string. Pass `-stdio` and the `js` build serves the pipe instead, as the native and
`wasip1` builds always do: the same `Content-Length` frames and JSON-RPC 2.0 bodies
`sysml-grpc -transport stdio` speaks, answered sequentially.

Measured on a `go1.25` `js/wasm` build of this tree: about 27.7 MB of module,
6.9 MB gzipped, 4.9 MB under Brotli.

Requests decode the lowerCamelCase field names protojson and protobuf-es emit; the proto
snake_case spellings protojson also accepts are not read. What it does not serve is refused
rather than dropped: an exploring schedule is answered
Unimplemented with `exploration is not served by sysml-engine: exploring schedules are served
by sysml-grpc`, and every other method name — verification, document queries, tools — answers
`<Method> is not served by sysml-engine`. Within the served methods: diagnostics are the
parser's syntax diagnostics only, since the engine does not run the analysis tier; a
`ParseSources` response carries no `roots`; and a frame whose `Content-Type` is a protobuf
body is answered `only application/json bodies are served`. Tool-backed engines are the
service's job too — the engine builds a runtime context directly, so behavior an external
engine would compute stays on `sysml-grpc`.

## What works

Everything that is the language implementation rather than the host around it:

- `-validate`, `-e`/`-eval`, `-query`, `-convert`, `-render` and every document form but PDF,
  `-engines`, and the rest of [the CLI](cli.md);
- the [prompt](../guide/04-repl.md), reading a line at a time from whatever the host wires up;
- `sysml-lsp -stdio`, the whole language server protocol over standard input and output;
- `sysml-grpc -transport stdio`, the service over the same framing, with its JSON-RPC bodies.

## What does not

Each of these is refused with the reason, not left to fail on the platform's own words.

| Not available | What it answers |
|---|---|
| External processes — SMT solvers, external engines, PDF rendering, `-compile` | `a WebAssembly build cannot start external processes`, named for what could not run: `an SMT solver cannot run: …` in `sysml -engines`, `weasyprint cannot run: …` from `-doc-form pdf`, `codegen: cc cannot run: …` from `-compile`, `codegen: go cannot run: …` from `-compile -target go`. Go's WebAssembly targets start no process at all, so installing the tool cannot help and the message says so instead of advising it |
| `sysml-grpc -transport grpc` and `-transport connect` | `… binds an address, and a WebAssembly build's network reaches only the process it runs in …; use -transport stdio`. Go's `net` on these targets reaches only the same process, so a listener would report an address no client outside could dial and wait on it forever |
| HTTP clients, such as the client that reads and pushes a Flexo branch | A transport error from the request. Outbound requests have no socket to make |
| Prompt history, tab completion, `Ctrl-C`, terminal width | Nothing is faked: lines are read without editing, renderings are written unbounded, and an interrupt is the host's to deliver |
| "Is it a terminal?" — for `at a terminal` status and for `-` on standard input | Never a terminal. Neither WASI preview 1 nor a browser offers the query, so a `-` reads the lines it is sent and ends at end of input, exactly as a redirected pipe does natively |

The prompt is the one behavior that differs by *degree*: a native build writes it only where
stdin is a terminal, and a WebAssembly build always writes it, because a host that cannot say
whether its input is a terminal still needs one to be usable. A script that does not want it can
drop it.

## The gate

```bash
make wasm-check
```

compiles and vets the whole tree for both targets, links each command, and runs them under Node
— the WASI host above for `wasip1`, `wasm_exec` for `js`. Node 24 or later is required: Node 22's
WASI host crashes on the `wasip1` modules. Without Node, the run half skips with the reason, and
`OPENSYSML_REQUIRE_WASM=1`, which both CI systems set, turns that skip into a failure so a green
run cannot be a skipped one.

The `wasip1` cases are driven with files and end each server at end of input, for the reason
under [Running a WASI build](#running-a-wasi-build); the `js` cases hold a pipe open across a
request and its answer, and run a full service session and a full language server
`initialize`/`shutdown`. Both halves say so where they differ.
