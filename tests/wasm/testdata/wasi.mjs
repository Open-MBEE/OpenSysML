// The WASI preview 1 host the gate runs wasip1 binaries under: Node ships one, and
// the runtimes Go's own go_wasip1_wasm_exec asks for (wasmtime, wasmer, wazero,
// wasmedge) are not installed where CI runs.
//
// Usage: node wasi.mjs <binary.wasm> [args...]
//
// Standard input, output and error are Node's own, so a test drives the process by
// feeding its stdin. '/' is preopened over '/', which lets a test name fixtures by
// their absolute path; a deployment would preopen only the directories its models
// live in. PWD is what a wasip1 program resolves relative paths against — its
// os.Getwd is that variable and nothing else — so it is set from the working
// directory of this process, which is where a relative argument would mean.
import { WASI } from 'node:wasi';
import fs from 'node:fs';

const [binary, ...args] = process.argv.slice(2);
const wasi = new WASI({
	version: 'preview1',
	args: [binary, ...args],
	env: { ...process.env, PWD: process.cwd() },
	preopens: { '/': '/' },
	returnOnExit: true,
});
const module = await WebAssembly.compile(fs.readFileSync(binary));
const instance = await WebAssembly.instantiate(module, wasi.getImportObject());
process.exit(wasi.start(instance));
