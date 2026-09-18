// Builds the per-platform npm packages that carry the sysml-grpc binary, from
// the release binaries the CI release job produces. Publishes nothing.
//
// Usage: node scripts/build-platform-packages.mjs --binaries <dir> [--version X.Y.Z] [--out <dir>]
// <dir> holds the release assets: sysml-grpc-<goos>-<goarch>[.exe] with .sha256 sidecars.

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
  const args = { binaries: undefined, out: join(clientRoot, "packages"), version: undefined };
  for (let index = 0; index < argv.length; index += 2) {
    const flag = argv[index];
    const value = argv[index + 1];
    if (value === undefined) {
      fail(`${flag} needs a value`);
    }
    if (flag === "--binaries") args.binaries = resolve(value);
    else if (flag === "--out") args.out = resolve(value);
    else if (flag === "--version") args.version = value;
    else fail(`unknown flag ${flag}`);
  }
  if (args.binaries === undefined) {
    fail("--binaries <dir> is required: the directory holding the release binaries");
  }
  args.version ??= JSON.parse(readFileSync(join(clientRoot, "package.json"), "utf8")).version;
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
    const name = `@opensysml/sysml-grpc-${platform.os}-${platform.cpu}`;
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
            directory: "clients/node",
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
        "optional dependency of [`@opensysml/client`](https://www.npmjs.com/package/@opensysml/client);\n" +
        "there is nothing to import here.\n\n" +
        `SHA-256 of \`bin/${platform.binary}\`: \`${digest}\`\n`,
    );
    built.push({ name, directory, digest });
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
