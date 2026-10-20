import assert from "node:assert/strict";
import { test } from "node:test";

import { serverBuildOf, serverReport } from "./serverinfo";

test("a release's extension is packaged with the release's server", () => {
  assert.equal(serverBuildOf("0.10.1"), "v0.10.1");
});

test("a release candidate's extension is packaged with the candidate's server, pre-release part and all", () => {
  assert.equal(serverBuildOf("0.10.2-rc.1"), "v0.10.2-rc.1");
  assert.equal(serverReport("0.10.2-rc.1", { name: "sysml-lsp", version: "v0.10.2-rc.1" }), "Connected to sysml-lsp v0.10.2-rc.1.");
});

test("a snapshot's extension is packaged with the snapshot's server, named by the pre-release part", () => {
  assert.equal(serverBuildOf("0.10.2-nightly-20261019-e8607989d"), "nightly-20261019-e8607989d");
});

test("the server of the same build is reported as reached", () => {
  assert.equal(serverReport("0.10.1", { name: "sysml-lsp", version: "v0.10.1" }), "Connected to sysml-lsp v0.10.1.");
  assert.equal(
    serverReport("0.10.2-nightly-20261019-e8607989d", { name: "sysml-lsp", version: "nightly-20261019-e8607989d" }),
    "Connected to sysml-lsp nightly-20261019-e8607989d.",
  );
});

test("a server of another build is reported with the build the extension was packaged with", () => {
  assert.equal(
    serverReport("0.10.2-nightly-20261019-e8607989d", { name: "sysml-lsp", version: "v0.10.0" }),
    "Connected to sysml-lsp v0.10.0; this extension (0.10.2-nightly-20261019-e8607989d) was packaged with nightly-20261019-e8607989d.",
  );
  assert.equal(
    serverReport("0.10.1", { name: "sysml-lsp", version: "v0.10.1-60-g11b5b0b60" }),
    "Connected to sysml-lsp v0.10.1-60-g11b5b0b60; this extension (0.10.1) was packaged with v0.10.1.",
  );
});

test("a server that gives no version, or no serverInfo at all, is said so", () => {
  assert.equal(
    serverReport("0.10.1", { name: "sysml-lsp" }),
    "Connected to sysml-lsp (no version); this extension (0.10.1) was packaged with v0.10.1.",
  );
  assert.equal(serverReport("0.10.1", undefined), "Connected to a server that does not report its name or version.");
});
