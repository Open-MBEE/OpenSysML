import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, realpathSync, rmSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, it } from "node:test";

import { canonicalPendingPath, confinedDirectory } from "./golden-dir";

describe("confinedDirectory", () => {
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
    assert.equal(confinedDirectory(root, root), canonicalRoot);
    assert.equal(confinedDirectory(path.join(root, "a", "b"), root), path.join(canonicalRoot, "a", "b"));
    assert.ok(!existsSync(path.join(root, "a")), "the check creates nothing");
  });

  it("canonicalizes a pending tail below an existing ancestor", () => {
    assert.equal(canonicalPendingPath(path.join(root, "x", "..", "y", "z")), path.join(realpathSync(root), "y", "z"));
  });

  it("refuses a .. traversal out of the checkout", () => {
    const outside = path.join(root, "..", "elsewhere");
    assert.throws(() => confinedDirectory(outside, root), new RegExp(`resolves to ${path.join(realpathSync(scratch), "elsewhere")}`));
    assert.ok(!existsSync(outside));
  });

  it("refuses a symlink inside the checkout that points outside it", () => {
    symlinkSync(scratch, path.join(root, "link"));
    assert.throws(() => confinedDirectory(path.join(root, "link", "parity"), root), /outside the repository checkout/);
    assert.ok(!existsSync(path.join(scratch, "parity")));
  });

  it("refuses a sibling whose name extends the checkout's", () => {
    mkdirSync(`${root}malicious`);
    assert.throws(() => confinedDirectory(path.join(`${root}malicious`, "parity"), root), /outside the repository checkout/);
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
});
