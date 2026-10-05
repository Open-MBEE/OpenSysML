// The combined js host: globalThis.sysmlWasm.call(method, paramsJSON) returns
// the JSON-RPC envelope synchronously.
import fs from 'node:fs';
import { pathToFileURL } from 'node:url';

const [wasmExec, binary] = process.argv.slice(2);
await import(pathToFileURL(wasmExec).href);
const go = new Go();
const { instance } = await WebAssembly.instantiate(fs.readFileSync(binary), go.importObject);
go.run(instance);
await new Promise(r => setImmediate(r));

const content = 'package Demo { part def Item; }';
const call = (method, params) => globalThis.sysmlWasm.call(method, JSON.stringify(params));
const parseFile = globalThis.sysmlWasm.call('ParseFile', JSON.stringify({ content }));
const hash = JSON.parse(parseFile).result?.modelHash;
console.log('version', globalThis.sysmlWasm.version);
console.log('ParseFile', parseFile);
console.log('Evaluate', call('Evaluate', { modelHash: hash, expression: '1 + 1' }));
console.log('GetSymbol', call('GetSymbol', { modelHash: hash, symbolId: 'Demo::Item' }));
console.log('GetDiagnostics', call('GetDiagnostics', { modelHash: hash }));
console.log('malformed', globalThis.sysmlWasm.call('ParseFile', '{'));
console.log('afterMalformed', call('GetDiagnostics', { modelHash: hash }));
process.exit(0);
