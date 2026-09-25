// The WASI preview 1 host the gate runs wasip1 binaries under: Node ships one, and
// the runtimes Go's own go_wasip1_wasm_exec asks for (wasmtime, wasmer, wazero,
// wasmedge) are not installed where CI runs.
//
// Usage: node wasi.mjs <binary.wasm> [args...]
//
// Standard input, output and error are Node's own, so a test drives the process by
// feeding its stdin. '/' is preopened over '/', which lets a test name fixtures by
// their absolute path; a deployment would preopen only the directories its models
// live in.
import { WASI } from 'node:wasi';
import fs from 'node:fs';

const [binary, ...args] = process.argv.slice(2);
const wasi = new WASI({
	version: 'preview1',
	args: [binary, ...args],
	env: Object.fromEntries(Object.entries(process.env)),
	preopens: { '/': '/' },
	returnOnExit: true,
});
const module = await WebAssembly.compile(fs.readFileSync(binary));
const instance = await WebAssembly.instantiate(module, wasi.getImportObject());
process.exit(wasi.start(instance));
