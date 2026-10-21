import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { test } from "node:test";
import { PLATFORMS, ShimError, binaryPath, platformPackageName, resolveBinary } from "../lib/platform.js";

const VERSION = "1.2.3";

// An installation: a directory whose node_modules holds the named platform
// packages at the given versions, and a file inside it to resolve from.
function installation(packages) {
  const root = mkdtempSync(join(tmpdir(), "sysml-mcp-shim-"));
  for (const [name, version] of Object.entries(packages)) {
    const dir = join(root, "node_modules", ...name.split("/"));
    mkdirSync(join(dir, "bin"), { recursive: true });
    writeFileSync(join(dir, "package.json"), JSON.stringify({ name, version }));
  }
  const from = join(root, "node_modules", "@openmbee", "sysml-mcp", "lib", "main.js");
  mkdirSync(join(from, ".."), { recursive: true });
  writeFileSync(from, "");
  return { root, from: pathToFileURL(from).href };
}

test("every release platform maps to a platform package of this package's name", () => {
  assert.deepEqual(
    PLATFORMS.map(({ os, cpu }) => platformPackageName(os, cpu)),
    [
      "@openmbee/sysml-mcp-linux-x64",
      "@openmbee/sysml-mcp-linux-arm64",
      "@openmbee/sysml-mcp-darwin-x64",
      "@openmbee/sysml-mcp-darwin-arm64",
      "@openmbee/sysml-mcp-win32-x64",
    ],
  );
  assert.equal(platformPackageName("freebsd", "x64"), undefined);
  assert.equal(platformPackageName("win32", "arm64"), undefined);
});

test("the binary is bin/sysml-mcp, with .exe on Windows", () => {
  assert.equal(binaryPath("linux"), join("bin", "sysml-mcp"));
  assert.equal(binaryPath("darwin"), join("bin", "sysml-mcp"));
  assert.equal(binaryPath("win32"), join("bin", "sysml-mcp.exe"));
});

test("the binary resolves from the installed platform package at the same version", () => {
  const { root, from } = installation({ "@openmbee/sysml-mcp-linux-arm64": VERSION });
  assert.equal(
    resolveBinary({ os: "linux", cpu: "arm64", version: VERSION, from }),
    join(root, "node_modules", "@openmbee", "sysml-mcp-linux-arm64", "bin", "sysml-mcp"),
  );
});

test("an unsupported platform is refused by name with the supported ones listed", () => {
  const { from } = installation({});
  assert.throws(
    () => resolveBinary({ os: "freebsd", cpu: "x64", version: VERSION, from }),
    (error) =>
      error instanceof ShimError &&
      /no release of sysml-mcp is built for freebsd-x64/.test(error.message) &&
      /linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64/.test(error.message) &&
      /go install/.test(error.message),
  );
});

test("a missing platform package is named, with the install that leaves it out", () => {
  const { from } = installation({ "@openmbee/sysml-mcp-linux-x64": VERSION });
  assert.throws(
    () => resolveBinary({ os: "darwin", cpu: "arm64", version: VERSION, from }),
    (error) =>
      error instanceof ShimError &&
      /@openmbee\/sysml-mcp-darwin-arm64@1\.2\.3 is not installed/.test(error.message) &&
      /--no-optional/.test(error.message) &&
      /npm install @openmbee\/sysml-mcp-darwin-arm64@1\.2\.3/.test(error.message),
  );
});

test("a platform package of another version is refused, naming both versions", () => {
  const { from } = installation({ "@openmbee/sysml-mcp-win32-x64": "1.2.2" });
  assert.throws(
    () => resolveBinary({ os: "win32", cpu: "x64", version: VERSION, from }),
    (error) =>
      error instanceof ShimError &&
      /@openmbee\/sysml-mcp@1\.2\.3: the installed @openmbee\/sysml-mcp-win32-x64 is 1\.2\.2/.test(error.message),
  );
});
