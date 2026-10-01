// The js host surface of sysml-engine: run with no -stdio, the binary installs
// globalThis.sysmlEngine and blocks; call(method, paramsJSON) answers the
// JSON-RPC response body synchronously. Drives ParseSources and Evaluate on the
// model it is handed, printing each answer on a line of its own.
//
// Usage: node engine.mjs <wasm_exec.js> <sysml-engine.wasm> <model.sysml>
import fs from 'node:fs';
import { pathToFileURL } from 'node:url';

const [wasmExec, binary, model] = process.argv.slice(2);
await import(pathToFileURL(wasmExec).href);
const go = new Go();
const { instance } = await WebAssembly.instantiate(fs.readFileSync(binary), go.importObject);
go.run(instance);
await new Promise(r => setImmediate(r));

const source = fs.readFileSync(model, 'utf8');
const parse = JSON.parse(globalThis.sysmlEngine.call('ParseSources', JSON.stringify({
	documents: [{ name: 'model.sysml', content: source }],
})));
const hash = parse.result?.modelHash;
console.log('modelHash', hash ?? JSON.stringify(parse));
const evaluate = JSON.parse(globalThis.sysmlEngine.call('Evaluate', JSON.stringify({
	modelHash: hash, expression: 'gatedemo::total',
})));
console.log('total', JSON.stringify(evaluate.result ?? evaluate.error));
// The engine blocks by design, so ending is the caller's own exit.
process.exit(0);
