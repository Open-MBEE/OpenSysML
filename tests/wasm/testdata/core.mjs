// The JS host surface of sysml-core: run with no -stdio, the binary installs
// globalThis.sysmlCore and call(method, paramsJSON) returns one envelope.
import fs from 'node:fs';
import { pathToFileURL } from 'node:url';

const [wasmExec, binary, modelPath] = process.argv.slice(2);
await import(pathToFileURL(wasmExec).href);
const go = new Go();
const { instance } = await WebAssembly.instantiate(fs.readFileSync(binary), go.importObject);
go.run(instance);
await new Promise(r => setImmediate(r));

const content = fs.readFileSync(modelPath, 'utf8');
const source = { documents: [{ name: 'core-validation.sysml', content }] };
const parsedEnvelope = globalThis.sysmlCore.call('ParseSources', JSON.stringify(source));
const parsed = JSON.parse(parsedEnvelope).result;
console.log('version', globalThis.sysmlCore.version);
console.log('ParseSources', parsedEnvelope);
console.log('ParseFile', globalThis.sysmlCore.call('ParseFile', JSON.stringify({ content })));
console.log('GetDiagnostics', globalThis.sysmlCore.call('GetDiagnostics', JSON.stringify({ modelHash: parsed.modelHash })));
console.log('GetSymbol', globalThis.sysmlCore.call('GetSymbol', JSON.stringify({
	modelHash: parsed.modelHash,
	symbolId: 'CoreValidation::two',
})));
console.log('malformed', globalThis.sysmlCore.call('ParseSources', '{'));
console.log('afterMalformed', globalThis.sysmlCore.call('GetDiagnostics', JSON.stringify({ modelHash: parsed.modelHash })));
console.log('Evaluate', globalThis.sysmlCore.call('Evaluate', '{}'));
process.exit(0);
