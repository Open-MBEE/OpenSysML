// Capability constants must be reachable from the package entry points, not only from the module that declares them.
import assert from "node:assert/strict";
import { test } from "node:test";
import * as browser from "../src/browser/index.js";
import {
  CAPABILITY_CHECK,
  CAPABILITY_EXPORT_GRAPHS,
  CAPABILITY_PARSE_SOURCES_AFFECTED,
  CAPABILITY_PARSE_SOURCES_KEEP,
} from "../src/node/index.js";

test("the node entry point publishes the capability constants", () => {
  assert.equal(CAPABILITY_PARSE_SOURCES_AFFECTED, "parse_sources_affected");
  assert.equal(CAPABILITY_PARSE_SOURCES_KEEP, "parse_sources_keep");
  assert.equal(CAPABILITY_CHECK, "check");
  assert.equal(CAPABILITY_EXPORT_GRAPHS, "export_graphs");
});

test("the browser entry point publishes the same capability constants", () => {
  assert.equal(browser.CAPABILITY_PARSE_SOURCES_AFFECTED, CAPABILITY_PARSE_SOURCES_AFFECTED);
  assert.equal(browser.CAPABILITY_PARSE_SOURCES_KEEP, CAPABILITY_PARSE_SOURCES_KEEP);
  assert.equal(browser.CAPABILITY_CHECK, CAPABILITY_CHECK);
  assert.equal(browser.CAPABILITY_EXPORT_GRAPHS, CAPABILITY_EXPORT_GRAPHS);
});
