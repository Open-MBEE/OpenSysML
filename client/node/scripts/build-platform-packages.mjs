// Builds the per-platform sysml-grpc packages and optional WASM asset package
// from the release binaries the CI release job produces. Publishes nothing.
//
// Usage: node scripts/build-platform-packages.mjs --binaries <dir> [--wasm <dir>] [--version X.Y.Z] [--out <dir>]
// The directories hold release assets with .sha256 sidecars.

import { createHash } from "node:crypto";
import {
  chmodSync,
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const clientRoot = resolve(here, "..");

/** The platforms a release builds, mapped to npm's os/cpu names. */
const PLATFORMS = [
  { asset: "sysml-grpc-linux-amd64", os: "linux", cpu: "x64", binary: "sysml-grpc" },
  { asset: "sysml-grpc-linux-arm64", os: "linux", cpu: "arm64", binary: "sysml-grpc" },
  { asset: "sysml-grpc-darwin-amd64", os: "darwin", cpu: "x64", binary: "sysml-grpc" },
  { asset: "sysml-grpc-darwin-arm64", os: "darwin", cpu: "arm64", binary: "sysml-grpc" },
  { asset: "sysml-grpc-windows-amd64.exe", os: "win32", cpu: "x64", binary: "sysml-grpc.exe" },
];

function parseArgs(argv) {
  const args = {
    binaries: undefined,
    wasm: undefined,
    out: join(clientRoot, "packages"),
    version: undefined,
  };
  for (let index = 0; index < argv.length; index += 2) {
    const flag = argv[index];
    const value = argv[index + 1];
    if (value === undefined) {
      fail(`${flag} needs a value`);
    }
    if (flag === "--binaries") args.binaries = resolve(value);
    else if (flag === "--wasm") args.wasm = resolve(value);
    else if (flag === "--out") args.out = resolve(value);
    else if (flag === "--version") args.version = value;
    else fail(`unknown flag ${flag}`);
  }
  if (args.binaries === undefined) {
    fail("--binaries <dir> is required: the directory holding the release binaries");
  }
  args.pkg = JSON.parse(readFileSync(join(clientRoot, "package.json"), "utf8"));
  args.version ??= args.pkg.version;
  return args;
}

function fail(message) {
  process.stderr.write(`build-platform-packages: ${message}\n`);
  process.exit(1);
}

/** Refuses a binary whose bytes do not match its published .sha256 sidecar. */
function verify(path) {
  const sidecar = `${path}.sha256`;
  if (!existsSync(sidecar)) {
    fail(`${sidecar} is missing; a binary is packaged only against its published digest`);
  }
  const expected = readFileSync(sidecar, "utf8").trim().split(/\s+/)[0];
  const actual = createHash("sha256").update(readFileSync(path)).digest("hex");
  if (expected !== actual) {
    fail(`${path} hashes to ${actual}, but its sidecar says ${expected}`);
  }
  return actual;
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  rmSync(args.out, { recursive: true, force: true });
  const built = [];

  for (const platform of PLATFORMS) {
    const source = join(args.binaries, platform.asset);
    if (!existsSync(source)) {
      fail(`${source} is missing; run the release build first`);
    }
    const digest = verify(source);
    const name = `${args.pkg.name}-sysml-grpc-${platform.os}-${platform.cpu}`;
    const directory = join(args.out, `sysml-grpc-${platform.os}-${platform.cpu}`);
    mkdirSync(join(directory, "bin"), { recursive: true });
    const destination = join(directory, "bin", platform.binary);
    copyFileSync(source, destination);
    chmodSync(destination, 0o755);

    writeFileSync(
      join(directory, "package.json"),
      `${JSON.stringify(
        {
          name,
          version: args.version,
          description: `sysml-grpc service binary for ${platform.os}-${platform.cpu}`,
          license: "Apache-2.0",
          repository: {
            type: "git",
            url: "git+https://github.com/Open-MBEE/OpenSysML.git",
            directory: "client/node",
          },
          os: [platform.os],
          cpu: [platform.cpu],
          files: ["bin", "README.md"],
        },
        null,
        2,
      )}\n`,
    );
    writeFileSync(
      join(directory, "README.md"),
      `# ${name}\n\n` +
        `The \`sysml-grpc\` service binary for ${platform.os}-${platform.cpu}. Installed as an\n` +
        `optional dependency of [\`${args.pkg.name}\`](https://www.npmjs.com/package/${args.pkg.name});\n` +
        "there is nothing to import here.\n\n" +
        `SHA-256 of \`bin/${platform.binary}\`: \`${digest}\`\n`,
    );
    built.push({ name, directory, digest });
  }

  if (args.wasm !== undefined) {
    const assets = ["sysml-wasm.wasm", "wasm_exec.js"];
    const digests = Object.fromEntries(
      assets.map((asset) => {
        const source = join(args.wasm, asset);
        if (!existsSync(source)) {
          fail(`${source} is missing; run the release build first`);
        }
        return [asset, verify(source)];
      }),
    );
    const name = `${args.pkg.name}-wasm`;
    const directory = join(args.out, "sysml-wasm");
    mkdirSync(directory, { recursive: true });
    for (const asset of assets) {
      copyFileSync(join(args.wasm, asset), join(directory, asset));
    }
    writeFileSync(
      join(directory, "package.json"),
      `${JSON.stringify(
        {
          name,
          version: args.version,
          description: "Combined sysml-wasm WebAssembly module and matching Go runtime",
          license: "Apache-2.0",
          repository: {
            type: "git",
            url: "git+https://github.com/Open-MBEE/OpenSysML.git",
            directory: "client/node",
          },
          files: ["sysml-wasm.wasm", "wasm_exec.js", "README.md"],
          exports: {
            "./sysml-wasm.wasm": "./sysml-wasm.wasm",
            "./wasm_exec.js": "./wasm_exec.js",
            "./package.json": "./package.json",
          },
        },
        null,
        2,
      )}\n`,
    );
    writeFileSync(
      join(directory, "README.md"),
      `# ${name}\n\n` +
        "The combined `sysml-wasm` WebAssembly module and its matching Go runtime. " +
        `Install this package alongside [\`${args.pkg.name}\`](https://www.npmjs.com/package/${args.pkg.name}); ` +
        "Node's `connectWasm()` discovers it automatically when no module is supplied.\n\n" +
        `SHA-256 of \`sysml-wasm.wasm\`: \`${digests["sysml-wasm.wasm"]}\`\n\n` +
        `SHA-256 of \`wasm_exec.js\`: \`${digests["wasm_exec.js"]}\`\n`,
    );
    built.push({
      name,
      directory,
      digest: digests["sysml-wasm.wasm"],
      digests,
    });
  }

  writeFileSync(
    join(args.out, "packages.json"),
    `${JSON.stringify({ version: args.version, packages: built }, null, 2)}\n`,
  );
  for (const entry of built) {
    process.stdout.write(`${entry.name}@${args.version}  ${entry.digest}\n`);
  }
  process.stdout.write(`\n${built.length} packages in ${args.out}; nothing was published.\n`);
}

main();
