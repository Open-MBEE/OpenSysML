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

  it("refuses a dangling symlinked file whose target would be created outside the checkout", () => {
    const missing = path.join(scratch, "missing.json");
    mkdirSync(path.join(root, "parity"));
    symlinkSync(missing, path.join(root, "parity", "golden.json"));
    assert.throws(() => confinedPath(path.join(root, "parity", "golden.json"), root), new RegExp(`resolves to ${missing}`));
    assert.ok(!existsSync(missing));
  });

  it("follows a dangling symlink whose target stays inside the checkout", () => {
    symlinkSync(path.join(root, "later"), path.join(root, "link"));
    assert.equal(confinedPath(path.join(root, "link", "parity"), root), path.join(realpathSync(root), "later", "parity"));
  });

  it("applies .. in a symlink target to the symlink it follows, as the filesystem does", () => {
    mkdirSync(path.join(root, "elsewhere", "nested"), { recursive: true });
    symlinkSync(path.join("elsewhere", "nested"), path.join(root, "deep"));
    // path.join would collapse the `..` lexically, which is exactly what the filesystem does not do.
    symlinkSync(["deep", "..", "goldens"].join(path.sep), path.join(root, "output"));
    assert.equal(confinedPath(path.join(root, "output"), root), path.join(realpathSync(root), "elsewhere", "goldens"));
    assert.equal(confinedPath([root, "deep", "..", "goldens"].join(path.sep), root), path.join(realpathSync(root), "elsewhere", "goldens"));
  });

  it("refuses a .. that climbs out through a symlink target", () => {
    symlinkSync(path.join(scratch, "outside"), path.join(root, "deep"));
    symlinkSync(["deep", "..", "goldens"].join(path.sep), path.join(root, "output"));
    assert.throws(() => confinedPath(path.join(root, "output"), root), new RegExp(`resolves to ${path.join(scratch, "goldens")}`));
    assert.throws(() => confinedPath([root, "deep", "..", "goldens"].join(path.sep), root), /outside the repository checkout/);
  });

  it("refuses a symlink loop", () => {
    symlinkSync(path.join(root, "b"), path.join(root, "a"));
    symlinkSync(path.join(root, "a"), path.join(root, "b"));
    assert.throws(() => confinedPath(path.join(root, "a", "parity"), root), /too many levels of symbolic links/);
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

  for (const dangling of [false, true]) {
    it(`refuses to write through a ${dangling ? "dangling " : ""}symlinked golden that points outside the repository`, () => {
      const scratch = mkdtempSync(path.join(tmpdir(), "route-goldens-"));
      // build/ is gitignored, so an output directory under it is inside the checkout but never committed.
      const buildDir = path.resolve("..", "..", "build");
      mkdirSync(buildDir, { recursive: true });
      const outDir = mkdtempSync(path.join(buildDir, "route-goldens-test-"));
      try {
        const victim = path.join(scratch, "victim.json");
        if (!dangling) {
          writeFileSync(victim, "untouched\n");
        }
        symlinkSync(victim, path.join(outDir, "random-pairs-0.json"));
        const run = spawnSync(process.execPath, ["tools/route-goldens.mjs", outDir], { encoding: "utf8" });
        assert.notEqual(run.status, 0);
        assert.match(run.stderr, /outside the repository checkout/);
        assert.equal(dangling ? existsSync(victim) : readFileSync(victim, "utf8"), dangling ? false : "untouched\n");
      } finally {
        rmSync(scratch, { recursive: true, force: true });
        rmSync(outDir, { recursive: true, force: true });
      }
    });
  }
});
