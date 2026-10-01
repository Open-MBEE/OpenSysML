// The js host surface of sysml-syntax: run with no -stdio, the binary installs
// globalThis.sysmlSyntax and blocks; call(method, paramsJSON) answers the
// JSON-RPC response body synchronously. Drives Parse, Format and Tokens on the
// fixtures it is handed — plus one method the command does not serve — printing
// each answer on a line of its own.
//
// Usage: node syntax.mjs <wasm_exec.js> <sysml-syntax.wasm> <malformed.sysml> <unformatted.sysml> <tokens.sysml>
import fs from 'node:fs';
import { pathToFileURL } from 'node:url';

const [wasmExec, binary, malformed, unformatted, tokens] = process.argv.slice(2);
await import(pathToFileURL(wasmExec).href);
const go = new Go();
const { instance } = await WebAssembly.instantiate(fs.readFileSync(binary), go.importObject);
go.run(instance);
await new Promise(r => setImmediate(r));

console.log('version', globalThis.sysmlSyntax.version);
for (const [method, params] of [
	['Parse', { content: fs.readFileSync(malformed, 'utf8') }],
	['Format', { content: fs.readFileSync(unformatted, 'utf8') }],
	['Tokens', { content: fs.readFileSync(tokens, 'utf8') }],
	['Evaluate', {}],
]) {
	const answer = globalThis.sysmlSyntax.call(method, JSON.stringify(params));
	console.log(method, answer);
}
// The command blocks by design, so ending is the caller's own exit.
process.exit(0);
