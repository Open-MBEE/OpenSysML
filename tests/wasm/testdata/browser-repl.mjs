// Drives docs/assets/sysml-repl.js — the page's host for the sysml REPL — under
// node: each walkthrough of the tours file runs in a fresh session with the
// example files mounted, and every step's output is printed as one JSON line.
//
// Usage: node browser-repl.mjs <wasm_exec.js> <sysml-repl.js> <sysml.wasm> <tours.json> <examples dir>
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const [wasmExec, host, binary, toursFile, examplesDir] = process.argv.slice(2);
await import(pathToFileURL(wasmExec).href);
await import(pathToFileURL(host).href);
const module = await WebAssembly.compile(fs.readFileSync(binary));
const files = {};
for (const name of fs.readdirSync(examplesDir).filter(n => n.endsWith('.sysml'))) {
	files['examples/runtime-showcase/' + name] = fs.readFileSync(path.join(examplesDir, name), 'utf8');
}
const { tours } = JSON.parse(fs.readFileSync(toursFile, 'utf8'));

for (const tour of tours) {
	let out = '', waiting = null;
	const ready = () => new Promise(resolve => { waiting = resolve; });
	let next = ready();
	const session = await globalThis.osmlRepl.start({
		module, files,
		onOutput: text => { out += text; },
		onWaiting: () => { const w = waiting; waiting = null; if (w) w(); },
	});
	await next;
	out = '';
	for (const [i, step] of tour.steps.entries()) {
		const t0 = performance.now();
		for (const line of step.input || []) {
			next = ready();
			session.send(line + '\n');
			await next;
		}
		const ms = Math.round(performance.now() - t0);
		console.log(JSON.stringify({ tour: tour.id, step: i, ms, output: out }));
		out = '';
	}
	session.eof();
	await session.done;
}

// Ctrl-C at a continuation prompt must drop the unfinished declaration, so the
// next line is read at the primary prompt.
{
	let out = '', waiting = null;
	const ready = () => new Promise(resolve => { waiting = resolve; });
	let next = ready();
	const session = await globalThis.osmlRepl.start({
		module, files,
		onOutput: text => { out += text; },
		onWaiting: () => { const w = waiting; waiting = null; if (w) w(); },
	});
	await next;
	out = '';
	for (const send of [() => session.send('package Unfinished {\n'), () => session.interrupt(), () => session.send('6 * 7\n')]) {
		next = ready();
		send();
		await next;
	}
	console.log(JSON.stringify({ tour: 'interrupt', step: 0, output: out }));
	session.eof();
	await session.done;
}
process.exit(0);
