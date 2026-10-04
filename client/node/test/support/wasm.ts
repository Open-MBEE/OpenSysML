import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  renameSync,
  unlinkSync,
} from "node:fs";
import { join } from "node:path";
import { packageRoot, repoRoot } from "./service.js";

export interface WasmArtifacts {
  wasm: string;
  wasmExec: string;
}

let artifacts: Promise<WasmArtifacts | undefined> | undefined;

/** Uses supplied WebAssembly artifacts or builds the combined module once. */
export function wasmArtifacts(): Promise<WasmArtifacts | undefined> {
  artifacts ??= Promise.resolve().then(resolveArtifacts);
  return artifacts;
}

function resolveArtifacts(): WasmArtifacts | undefined {
  const wasm = process.env.OPENSYSML_WASM;
  const wasmExec = process.env.OPENSYSML_WASM_EXEC;
  if (wasm !== undefined || wasmExec !== undefined) {
    if (wasm === undefined || wasmExec === undefined) {
      throw new Error("OPENSYSML_WASM and OPENSYSML_WASM_EXEC must be set together");
    }
    if (!existsSync(wasm) || !existsSync(wasmExec)) {
      throw new Error("the configured WebAssembly module or wasm_exec.js does not exist");
    }
    return { wasm, wasmExec };
  }

  let goroot: string;
  try {
    goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
  } catch (cause) {
    if (isMissingGo(cause) && process.env.OPENSYSML_REQUIRE_WASM !== "1") {
      return undefined;
    }
    throw new Error("Go is required to build the sysml-wasm test module", { cause });
  }

  const dir = join(packageRoot, "build", "wasm");
  const output = join(dir, "sysml-wasm.wasm");
  const runtime = join(goroot, "lib", "wasm", "wasm_exec.js");
  if (!existsSync(runtime)) {
    throw new Error(`the Go runtime script does not exist: ${runtime}`);
  }
  if (!existsSync(output)) {
    mkdirSync(dir, { recursive: true });
    const staged = `${output}.${String(process.pid)}.stage`;
    try {
      execFileSync(
        "go",
        [
          "build",
          "-trimpath",
          "-ldflags=-s -w",
          "-o",
          staged,
          "./cmd/sysml-wasm",
        ],
        {
          cwd: repoRoot,
          env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
          stdio: "inherit",
        },
      );
      renameSync(staged, output);
    } finally {
      if (existsSync(staged)) {
        unlinkSync(staged);
      }
    }
  }
  return { wasm: output, wasmExec: runtime };
}

function isMissingGo(error: unknown): boolean {
  return (
    error instanceof Error &&
    "code" in error &&
    (error as NodeJS.ErrnoException).code === "ENOENT"
  );
}
