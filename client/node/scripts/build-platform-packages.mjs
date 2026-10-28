// Builds the per-platform binary packages and the optional WASM asset package
// from the release binaries the CI release job produces. Publishes nothing.
//
// Usage: node scripts/build-platform-packages.mjs [--binaries <dir>] [--mcp <dir>] [--wasm <dir>] [--version X.Y.Z] [--out <dir>]
// The directories hold release assets with .sha256 sidecars: --binaries the
// sysml-grpc binaries @openmbee/opensysml runs, --mcp the sysml-mcp binaries
// @openmbee/sysml-mcp (packaging/npm/sysml-mcp) runs, --wasm the WASM assets.
// With --mcp the runner's alias package, @openmbee/opensysml-mcp, is generated
// from packaging/npm/sysml-mcp beside the platform packages (see buildMCPAlias).

import { createHash } from "node:crypto";
import {
  chmodSync,
  copyFileSync,
  cpSync,
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
const repoRoot = resolve(clientRoot, "..", "..");
const mcpRoot = join(repoRoot, "packaging", "npm", "sysml-mcp");

/**
 * The second npm name the runner is published under, matching the other
 * OpenSysML packages (@openmbee/opensysml, @openmbee/opensysml-wasm); it
 * pins the same @openmbee/sysml-mcp-<os>-<cpu> platform packages.
 */
const MCP_ALIAS = "@openmbee/opensysml-mcp";

/** The platforms a release builds, mapped to npm's os/cpu names. */
const PLATFORMS = [
  { suffix: "linux-amd64", os: "linux", cpu: "x64" },
  { suffix: "linux-arm64", os: "linux", cpu: "arm64" },
  { suffix: "darwin-amd64", os: "darwin", cpu: "x64" },
  { suffix: "darwin-arm64", os: "darwin", cpu: "arm64" },
  { suffix: "windows-amd64.exe", os: "win32", cpu: "x64" },
];

/** The release binary and the binary name inside a package, for a command on a platform. */
function platformBinary(command, platform) {
  const exe = platform.os === "win32" ? ".exe" : "";
  return { asset: `${command}-${platform.suffix}`, binary: `${command}${exe}` };
}

function parseArgs(argv) {
  const args = {
    binaries: undefined,
    mcp: undefined,
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
    else if (flag === "--mcp") args.mcp = resolve(value);
    else if (flag === "--wasm") args.wasm = resolve(value);
    else if (flag === "--out") args.out = resolve(value);
    else if (flag === "--version") args.version = value;
    else fail(`unknown flag ${flag}`);
  }
  if (args.binaries === undefined && args.mcp === undefined && args.wasm === undefined) {
    fail("nothing to build: give --binaries <dir> (sysml-grpc), --mcp <dir> (sysml-mcp) or --wasm <dir>");
  }
  args.pkg = JSON.parse(readFileSync(join(clientRoot, "package.json"), "utf8"));
  args.mcpPkg = JSON.parse(readFileSync(join(mcpRoot, "package.json"), "utf8"));
  if (args.version === undefined && args.mcp !== undefined && args.mcpPkg.version !== args.pkg.version) {
    fail(
      `${args.mcpPkg.name} is ${args.mcpPkg.version} but ${args.pkg.name} is ${args.pkg.version}; ` +
        "both are published from one release tag at one version",
    );
  }
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

function writeManifest(directory, manifest) {
  writeFileSync(join(directory, "package.json"), `${JSON.stringify(manifest, null, 2)}\n`);
}

/**
 * Builds one package per platform around a release binary: the binary under
 * bin/, the family's extra files beside it, and a README naming the digest.
 *
 * @param {object} family
 * @param {string} family.command the binary's command name, as the release asset is prefixed
 * @param {(platform: object) => string} family.name the package name for a platform
 * @param {string} family.directory the directory the package's source tree names
 * @param {string} family.description what the binary is
 * @param {string} family.parent the package whose optionalDependencies name these
 * @param {{source: string, destination: string}[]} family.extras files copied into every package
 */
function buildBinaryPackages(args, binaries, family) {
  const built = [];
  for (const platform of PLATFORMS) {
    const { asset, binary } = platformBinary(family.command, platform);
    const source = join(binaries, asset);
    if (!existsSync(source)) {
      fail(`${source} is missing; run the release build first`);
    }
    const digest = verify(source);
    const name = family.name(platform);
    const directory = join(args.out, `${family.command}-${platform.os}-${platform.cpu}`);
    mkdirSync(join(directory, "bin"), { recursive: true });
    const destination = join(directory, "bin", binary);
    copyFileSync(source, destination);
    chmodSync(destination, 0o755);
    for (const extra of family.extras) {
      if (!existsSync(extra.source)) {
        fail(`${extra.source} is missing; every ${family.command} package carries it`);
      }
      const target = join(directory, extra.destination);
      mkdirSync(dirname(target), { recursive: true });
      copyFileSync(extra.source, target);
    }
    const files = ["bin", ...new Set(family.extras.map((extra) => extra.destination.split("/")[0])), "README.md"];

    writeManifest(directory, {
      name,
      version: args.version,
      description: `${family.description} for ${platform.os}-${platform.cpu}`,
      license: "Apache-2.0",
      repository: {
        type: "git",
        url: "git+https://github.com/Open-MBEE/OpenSysML.git",
        directory: family.directory,
      },
      os: [platform.os],
      cpu: [platform.cpu],
      files,
    });
    writeFileSync(
      join(directory, "README.md"),
      `# ${name}\n\n` +
        `${family.description} for ${platform.os}-${platform.cpu}. Installed as an\n` +
        `optional dependency of [\`${family.parent}\`](https://www.npmjs.com/package/${family.parent});\n` +
        "there is nothing to import here.\n\n" +
        `SHA-256 of \`bin/${binary}\`: \`${digest}\`\n`,
    );
    built.push({ name, directory, digest });
  }
  return built;
}

/**
 * Generates @openmbee/opensysml-mcp from packaging/npm/sysml-mcp: the same
 * bin/, lib/ and README, the manifest renamed, with `opensysml-mcp` as a second
 * command beside `sysml-mcp` and its platform packages pinned at the version
 * being built. The registry listing stays with @openmbee/sysml-mcp, so the
 * alias declares no mcpName. There is one source tree: nothing here is edited by hand.
 */
function buildMCPAlias(args) {
  const source = args.mcpPkg;
  const directory = join(args.out, "opensysml-mcp");
  mkdirSync(directory, { recursive: true });
  for (const part of ["bin", "lib"]) {
    cpSync(join(mcpRoot, part), join(directory, part), { recursive: true });
  }
  const shim = source.bin?.["sysml-mcp"];
  if (shim === undefined) {
    fail(`${source.name} declares no sysml-mcp command for ${MCP_ALIAS} to share`);
  }
  const platforms = Object.keys(source.optionalDependencies ?? {});
  if (platforms.length === 0) {
    fail(`${source.name} pins no platform package for ${MCP_ALIAS} to pin`);
  }
  // Everything but the name, the description's install line, the commands, the
  // pins and the registry name is the runner's own manifest.
  const manifest = { ...source };
  delete manifest.mcpName;
  delete manifest.scripts;
  manifest.name = MCP_ALIAS;
  manifest.version = args.version;
  manifest.description = source.description.replaceAll(`npx -y ${source.name}`, `npx -y ${MCP_ALIAS}`);
  manifest.bin = { "sysml-mcp": shim, "opensysml-mcp": shim };
  manifest.optionalDependencies = Object.fromEntries(platforms.map((name) => [name, args.version]));
  writeManifest(directory, manifest);

  // The runner's README under the alias's name; the platform package names
  // (@openmbee/sysml-mcp-<os>-<cpu>) and links to the runner's own page stay.
  const sourceName = source.name.replaceAll("/", String.raw`\/`);
  const readme = readFileSync(join(mcpRoot, "README.md"), "utf8")
    .replaceAll(new RegExp(String.raw`${sourceName}(?![-\w])`, "g"), MCP_ALIAS)
    .replace(
      "\n\n```sh\n",
      `\n\nThis is [\`${source.name}\`](https://www.npmjs.com/package/${source.name}) under the name the other OpenSysML npm packages share: the same command, published from the same release at the same version, with \`opensysml-mcp\` as a second name for it.\n\n\`\`\`sh\n`,
    );
  writeFileSync(join(directory, "README.md"), readme);
  return { name: MCP_ALIAS, directory, alias: source.name };
}

function buildWasmPackage(args) {
  const assets = ["sysml-wasm.wasm", "sysml-prod.wasm.gz", "wasm_exec.js"];
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
  writeManifest(directory, {
    name,
    version: args.version,
    description: "Combined sysml-wasm WebAssembly module, the browser build of the sysml REPL and matching Go runtime",
    license: "Apache-2.0",
    repository: {
      type: "git",
      url: "git+https://github.com/Open-MBEE/OpenSysML.git",
      directory: "client/node",
    },
    files: ["sysml-wasm.wasm", "sysml-prod.wasm.gz", "wasm_exec.js", "README.md"],
    exports: {
      "./sysml-wasm.wasm": "./sysml-wasm.wasm",
      "./sysml-prod.wasm.gz": "./sysml-prod.wasm.gz",
      "./wasm_exec.js": "./wasm_exec.js",
      "./package.json": "./package.json",
    },
  });
  writeFileSync(
    join(directory, "README.md"),
    `# ${name}\n\n` +
      "The combined `sysml-wasm` WebAssembly module and its matching Go runtime. " +
      `Install this package alongside [\`${args.pkg.name}\`](https://www.npmjs.com/package/${args.pkg.name}); ` +
      "Node's `connectWasm()` discovers it automatically when no module is supplied.\n\n" +
      "`sysml-prod.wasm.gz` is the browser build of the `sysml` REPL (`-tags sysml_re`), " +
      "gzip-compressed; a page inflates it with `DecompressionStream` and runs it under `wasm_exec.js` " +
      "with an emulated file system and stdin, the way the OpenSysML site's REPL page does.\n\n" +
      `SHA-256 of \`sysml-wasm.wasm\`: \`${digests["sysml-wasm.wasm"]}\`\n\n` +
      `SHA-256 of \`sysml-prod.wasm.gz\`: \`${digests["sysml-prod.wasm.gz"]}\`\n\n` +
      `SHA-256 of \`wasm_exec.js\`: \`${digests["wasm_exec.js"]}\`\n`,
  );
  return { name, directory, digest: digests["sysml-wasm.wasm"], digests };
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  rmSync(args.out, { recursive: true, force: true });
  const built = [];

  if (args.binaries !== undefined) {
    built.push(
      ...buildBinaryPackages(args, args.binaries, {
        command: "sysml-grpc",
        name: (platform) => `${args.pkg.name}-sysml-grpc-${platform.os}-${platform.cpu}`,
        directory: "client/node",
        description: "sysml-grpc service binary",
        parent: args.pkg.name,
        extras: [],
      }),
    );
  }

  if (args.mcp !== undefined) {
    // What the sysml-mcp release archive holds, so `npx` installs the same
    // thing the archive does: the binary, its LICENSE and its man page.
    built.push(
      ...buildBinaryPackages(args, args.mcp, {
        command: "sysml-mcp",
        name: (platform) => `${args.mcpPkg.name}-${platform.os}-${platform.cpu}`,
        directory: "packaging/npm/sysml-mcp",
        description: "sysml-mcp Model Context Protocol server binary",
        parent: args.mcpPkg.name,
        extras: [
          { source: join(repoRoot, "LICENSE"), destination: "LICENSE" },
          {
            source: join(repoRoot, "packaging", "man", "man1", "sysml-mcp.1"),
            destination: "share/man/man1/sysml-mcp.1",
          },
        ],
      }),
      buildMCPAlias(args),
    );
  }

  if (args.wasm !== undefined) {
    built.push(buildWasmPackage(args));
  }

  writeFileSync(
    join(args.out, "packages.json"),
    `${JSON.stringify({ version: args.version, packages: built }, null, 2)}\n`,
  );
  for (const entry of built) {
    const provenance = entry.digest ?? `alias of ${entry.alias}`;
    process.stdout.write(`${entry.name}@${args.version}  ${provenance}\n`);
  }
  process.stdout.write(`\n${built.length} packages in ${args.out}; nothing was published.\n`);
}

main();
