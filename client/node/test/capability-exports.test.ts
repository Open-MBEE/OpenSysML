// Parse-sources capabilities must be reachable from the package, not only its capabilities module.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  CAPABILITY_PARSE_SOURCES_AFFECTED,
  CAPABILITY_PARSE_SOURCES_KEEP,
} from "../src/node/index.js";

test("parse-sources capabilities are public exports", () => {
  assert.equal(CAPABILITY_PARSE_SOURCES_AFFECTED, "parse_sources_affected");
  assert.equal(CAPABILITY_PARSE_SOURCES_KEEP, "parse_sources_keep");
});
