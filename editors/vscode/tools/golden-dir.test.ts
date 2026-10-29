import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, it } from "node:test";

import { canonicalPendingPath, confinedPath } from "./golden-dir";

describe("confinedPath", () => {
  let scratch: string;
  let root: string;
  beforeEach(() => {
    scratch = mkdtempSync(path.join(tmpdir(), "route-goldens-"));
    root = path.join(scratch, "checkout");
    mkdirSync(root);
  });
  afterEach(() => rmSync(scratch, { recursive: true, force: true }));

  it("accepts the checkout itself and a directory to be created under it", () => {
    const canonicalRoot = realpathSync(root);
    assert.equal(confinedPath(root, root), canonicalRoot);
    assert.equal(confinedPath(path.join(root, "a", "b"), root), path.join(canonicalRoot, "a", "b"));
    assert.ok(!existsSync(path.join(root, "a")), "the check creates nothing");
  });

  it("canonicalizes a pending tail below an existing ancestor", () => {
    assert.equal(canonicalPendingPath(path.join(root, "x", "..", "y", "z")), path.join(realpathSync(root), "y", "z"));
  });

  it("refuses a .. traversal out of the checkout", () => {
    const outside = path.join(root, "..", "elsewhere");
    assert.throws(() => confinedPath(outside, root), new RegExp(`resolves to ${path.join(realpathSync(scratch), "elsewhere")}`));
    assert.ok(!existsSync(outside));
  });

  it("refuses a symlink inside the checkout that points outside it", () => {
    symlinkSync(scratch, path.join(root, "link"));
    assert.throws(() => confinedPath(path.join(root, "link", "parity"), root), /outside the repository checkout/);
    assert.ok(!existsSync(path.join(scratch, "parity")));
  });

  it("refuses a symlinked file inside the checkout whose target is outside it", () => {
    const victim = path.join(scratch, "victim.json");
    writeFileSync(victim, "{}\n");
    mkdirSync(path.join(root, "parity"));
    symlinkSync(victim, path.join(root, "parity", "golden.json"));
    assert.throws(() => confinedPath(path.join(root, "parity", "golden.json"), root), new RegExp(`resolves to ${realpathSync(victim)}`));
  });

  it("refuses a sibling whose name extends the checkout's", () => {
    mkdirSync(`${root}malicious`);
    assert.throws(() => confinedPath(path.join(`${root}malicious`, "parity"), root), /outside the repository checkout/);
  });
});

describe("route-goldens.mjs", () => {
  it("refuses an output directory outside the repository and creates nothing", () => {
    const scratch = mkdtempSync(path.join(tmpdir(), "route-goldens-"));
    try {
      const target = path.join(scratch, "parity");
      const run = spawnSync(process.execPath, ["tools/route-goldens.mjs", target], { encoding: "utf8" });
      assert.notEqual(run.status, 0);
      assert.match(run.stderr, /outside the repository checkout/);
      assert.ok(!existsSync(target));
    } finally {
      rmSync(scratch, { recursive: true, force: true });
    }
  });

  it("refuses to write through a symlinked golden that points outside the repository", () => {
    const scratch = mkdtempSync(path.join(tmpdir(), "route-goldens-"));
    // build/ is gitignored, so an output directory under it is inside the checkout but never committed.
    const outDir = mkdtempSync(path.resolve("..", "..", "build", "route-goldens-test-"));
    try {
      const victim = path.join(scratch, "victim.json");
      writeFileSync(victim, "untouched\n");
      symlinkSync(victim, path.join(outDir, "random-pairs-0.json"));
      const run = spawnSync(process.execPath, ["tools/route-goldens.mjs", outDir], { encoding: "utf8" });
      assert.notEqual(run.status, 0);
      assert.match(run.stderr, /outside the repository checkout/);
      assert.equal(readFileSync(victim, "utf8"), "untouched\n");
    } finally {
      rmSync(scratch, { recursive: true, force: true });
      rmSync(outDir, { recursive: true, force: true });
    }
  });
});
