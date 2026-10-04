import { parentPort } from "node:worker_threads";
import {
  loadGoConstructor,
  serveWasmPort,
  type WasmPortLike,
} from "../core/wasm.js";
import { loadNodeWasm, moduleSpecifier } from "./wasm-source.js";

if (parentPort === null) {
  throw new Error("the sysml-wasm worker must run in a worker thread");
}

serveWasmPort(parentPort as unknown as WasmPortLike, {
  loadWasm: loadNodeWasm,
  loadGo: (wasmExec) => loadGoConstructor(wasmExec === undefined ? undefined : moduleSpecifier(wasmExec)),
});
