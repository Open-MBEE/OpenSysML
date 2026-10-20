// Which platform package carries the sysml-mcp binary for a Node platform and
// architecture, and where the binary sits inside it.

import { createRequire } from "node:module";
import { dirname, join } from "node:path";

/** The platforms a release builds, by Node's process.platform and process.arch. */
export const PLATFORMS = [
  { os: "linux", cpu: "x64" },
  { os: "linux", cpu: "arm64" },
  { os: "darwin", cpu: "x64" },
  { os: "darwin", cpu: "arm64" },
  { os: "win32", cpu: "x64" },
];

export const PACKAGE_NAME = "@openmbee/sysml-mcp";

/** The platform package name for a platform, or undefined when no release builds it. */
export function platformPackageName(os, cpu) {
  const supported = PLATFORMS.some((platform) => platform.os === os && platform.cpu === cpu);
  return supported ? `${PACKAGE_NAME}-${os}-${cpu}` : undefined;
}

/** The binary's path inside a platform package. */
export function binaryPath(os) {
  return join("bin", os === "win32" ? "sysml-mcp.exe" : "sysml-mcp");
}

/**
 * An error the shim reports to the caller as its own: the message is complete
 * and the exit status is fixed, so the caller prints it and exits.
 */
export class ShimError extends Error {
  constructor(message) {
    super(message);
    this.name = "ShimError";
  }
}

/**
 * Resolves the binary for a platform from the platform package installed beside
 * this one, at exactly this package's version.
 *
 * @param {object} options
 * @param {string} options.os process.platform
 * @param {string} options.cpu process.arch
 * @param {string} options.version this package's version
 * @param {string} [options.from] a file inside the installation to resolve from
 * @returns {string} the absolute path of the binary
 */
export function resolveBinary({ os, cpu, version, from = import.meta.url }) {
  const name = platformPackageName(os, cpu);
  if (name === undefined) {
    const supported = PLATFORMS.map(({ os: o, cpu: c }) => `${o}-${c}`).join(", ");
    throw new ShimError(
      `${PACKAGE_NAME}: no release of sysml-mcp is built for ${os}-${cpu}; ` +
        `the platforms are ${supported}. Build it from source with ` +
        "`go install github.com/Open-MBEE/OpenSysML/cmd/sysml-mcp@latest` and run that binary instead.",
    );
  }
  const require = createRequire(from);
  let manifestPath;
  try {
    manifestPath = require.resolve(`${name}/package.json`);
  } catch (error) {
    if (error && error.code === "MODULE_NOT_FOUND") {
      throw new ShimError(
        `${PACKAGE_NAME}: the platform package ${name}@${version} is not installed. ` +
          "It is an optional dependency, so an install with --no-optional or --omit=optional " +
          `leaves it out; run \`npm install ${name}@${version}\` or reinstall ${PACKAGE_NAME} ` +
          "with optional dependencies.",
      );
    }
    throw error;
  }
  const manifest = require(manifestPath);
  if (manifest.version !== version) {
    throw new ShimError(
      `${PACKAGE_NAME}@${version}: the installed ${name} is ${manifest.version}; the binary ` +
        `must be the same release. Run \`npm install ${name}@${version}\`.`,
    );
  }
  return join(dirname(manifestPath), binaryPath(os));
}
