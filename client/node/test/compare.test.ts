// The comparison rules conformance/README.md states, on their own.

import assert from "node:assert/strict";
import { test } from "node:test";
import { check, isDefault, lookup, render } from "../conformance/compare.js";
import { Integer } from "../conformance/normalize.js";
import { Literal } from "../conformance/scenarios.js";

const number = (text: string): Literal => new Literal(text);

test("a response expectation is a subset: unmentioned fields are ignored", () => {
  const actual = { version: "${version}", capabilities: ["query"], extra: "ignored" };
  assert.deepEqual(check({ response: { version: "${version}" } }, actual), []);
});

test("an integer is compared exactly, with no tolerance", () => {
  const actual = { id: new Integer("9007199254740993") };
  assert.deepEqual(check({ response: { id: number("9007199254740993") } }, actual), []);
  assert.deepEqual(check({ response: { id: number("9007199254740992") } }, actual), [
    "id: 9007199254740993, want 9007199254740992",
  ]);
});

test("a real is compared within the relative tolerance", () => {
  const actual = { value: 0.30000000000000004 };
  assert.deepEqual(check({ response: { value: number("0.3") } }, actual), []);
  assert.equal(check({ response: { value: number("0.3000001") } }, actual).length, 1);
});

test("an unset field equals its default, and a non-default absence fails", () => {
  assert.deepEqual(check({ response: { has_errors: false, name: "" } }, {}), []);
  assert.deepEqual(check({ response: { has_errors: true } }, {}), ["has_errors: not set, want true"]);
});

test("a list must have exactly the expected length", () => {
  const actual = { diagnostics: [{ message: "one" }, { message: "two" }] };
  assert.equal(check({ response: { diagnostics: [{ message: "one" }] } }, actual).length, 1);
  assert.deepEqual(
    check({ response: { diagnostics: [{ message: "one" }, { message: "two" }] } }, actual),
    [],
  );
});

test("counts are exact and min_counts are a floor", () => {
  const actual = { symbols: [{ id: "a" }, { id: "b" }] };
  assert.deepEqual(check({ counts: { symbols: number("2") } }, actual), []);
  assert.equal(check({ counts: { symbols: number("3") } }, actual).length, 1);
  assert.deepEqual(check({ min_counts: { symbols: number("1") } }, actual), []);
  assert.equal(check({ min_counts: { symbols: number("3") } }, actual).length, 1);
});

test("contains, contains_all, non_empty and absent read paths", () => {
  const actual = { error: "symbol not found: Sample::Nope", capabilities: ["query", "convert"] };
  assert.deepEqual(check({ contains: { error: "not found" } }, actual), []);
  assert.deepEqual(check({ contains_all: { capabilities: ["convert", "query"] } }, actual), []);
  assert.equal(check({ contains_all: { capabilities: ["missing"] } }, actual).length, 1);
  assert.deepEqual(check({ non_empty: ["error"] }, actual), []);
  assert.deepEqual(check({ absent: ["instance"] }, actual), []);
  assert.equal(check({ absent: ["error"] }, actual).length, 1);
  assert.equal(check({ non_empty: ["instance"] }, actual).length, 1);
});

test("a path walks fields, list indices, map keys and a wildcard", () => {
  const tree = {
    instance: { feature_values: { wheels: { values: [{ instance_id: "@1" }, { instance_id: "@2" }] } } },
  };
  assert.deepEqual(lookup(tree, "instance.feature_values.wheels.values.1.instance_id"), {
    ok: true,
    value: "@2",
  });
  assert.deepEqual(lookup(tree, "instance.feature_values.missing"), { ok: false });
  assert.deepEqual(
    check({ contains_all: { "instance.feature_values.*.values.*.instance_id": ["@1", "@2"] } }, tree),
    [],
  );
});

test("what counts as a default value", () => {
  assert.ok(isDefault(""));
  assert.ok(isDefault(0));
  assert.ok(isDefault(false));
  assert.ok(isDefault([]));
  assert.ok(isDefault({}));
  assert.ok(isDefault(new Integer("0")));
  assert.ok(isDefault(new Literal("0")));
  assert.ok(!isDefault("x"));
  assert.ok(!isDefault([1]));
});

test("failures render the value they saw", () => {
  assert.equal(render({ a: [1, new Integer("2")] }), "{a: [1, 2]}");
  assert.equal(render("text"), '"text"');
});
