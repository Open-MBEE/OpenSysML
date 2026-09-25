// Comparing a normalized response against a scenario's expectation, by the
// rules in conformance/README.md.

import { Integer, type Normalized } from "./normalize.js";
import { byCodeUnit } from "./order.js";
import { Literal, type Expect } from "./scenarios.js";

/**
 * The relative difference two Real values may have and still be the same value.
 * It applies to Reals alone: an integral field is compared by its digits.
 */
export const TOLERANCE = 1e-9;

/** Compares a normalized response against one expectation, one message per mismatch. */
export function check(expect: Expect, actual: Normalized): string[] {
  const failures: string[] = [];
  if (expect.response !== undefined) {
    failures.push(...match("", expect.response, actual));
  }
  for (const path of [...(expect.non_empty ?? [])].sort(byCodeUnit)) {
    const found = lookup(actual, path);
    if (!found.ok) {
      failures.push(`${path}: not set, want a value`);
    } else if (isDefault(found.value)) {
      failures.push(`${path}: empty, want a value`);
    }
  }
  for (const path of [...(expect.absent ?? [])].sort(byCodeUnit)) {
    const found = lookup(actual, path);
    if (found.ok && !isDefault(found.value)) {
      failures.push(`${path}: set to ${render(found.value)}, want it unset`);
    }
  }
  for (const path of sortedKeys(expect.contains)) {
    const want = expect.contains?.[path] ?? "";
    const found = lookup(actual, path);
    if (!found.ok) {
      failures.push(`${path}: not set, want it to contain ${quote(want)}`);
    } else if (typeof found.value !== "string") {
      failures.push(`${path}: ${render(found.value)} is not text, want it to contain ${quote(want)}`);
    } else if (!found.value.includes(want)) {
      failures.push(`${path}: ${quote(found.value)} does not contain ${quote(want)}`);
    }
  }
  for (const path of sortedKeys(expect.contains_all)) {
    failures.push(...containsAll(actual, path, expect.contains_all?.[path] ?? []));
  }
  for (const path of sortedKeys(expect.counts)) {
    const want = numberOf(expect.counts?.[path]);
    const got = count(actual, path);
    if (got === undefined) {
      failures.push(`${path}: not a list or map, want ${want} entries`);
    } else if (got !== want) {
      failures.push(`${path}: ${got} entries, want ${want}`);
    }
  }
  for (const path of sortedKeys(expect.min_counts)) {
    const want = numberOf(expect.min_counts?.[path]);
    const got = count(actual, path);
    if (got === undefined) {
      failures.push(`${path}: not a list or map, want at least ${want} entries`);
    } else if (got < want) {
      failures.push(`${path}: ${got} entries, want at least ${want}`);
    }
  }
  return failures;
}

/**
 * Checks that every wanted string is at `path`: a substring of the text there, or
 * a member of the values there. `*` takes one field of every entry of a list.
 */
function containsAll(actual: Normalized, path: string, wants: string[]): string[] {
  const found = lookup(actual, path);
  if (found.ok && typeof found.value === "string") {
    const text = found.value;
    return wants.filter((want) => !text.includes(want)).map((want) => `${path}: does not contain ${quote(want)}`);
  }
  const collected = values(actual, path);
  if (collected === undefined) {
    return [`${path}: neither text nor a list, want it to contain ${render(wants)}`];
  }
  return wants
    .filter((want) => !collected.includes(want))
    .map((want) => `${path}: ${render(collected)} does not contain ${quote(want)}`);
}

/** The values at a path, expanding `*` and a trailing list into its members. */
function values(tree: unknown, path: string): unknown[] | undefined {
  const found = walk(tree, path.split("."));
  if (found === undefined) {
    return undefined;
  }
  if (found.length === 1 && Array.isArray(found[0])) {
    return found[0] as unknown[];
  }
  return found;
}

function walk(current: unknown, segments: string[]): unknown[] | undefined {
  if (segments.length === 0) {
    return [current];
  }
  const [head, ...rest] = segments;
  if (head === "*") {
    let entries: unknown[];
    if (Array.isArray(current)) {
      entries = current;
    } else if (isObject(current)) {
      entries = sortedKeys(current).map((key) => current[key]);
    } else {
      return undefined;
    }
    const found: unknown[] = [];
    for (const entry of entries) {
      const reached = walk(entry, rest);
      if (reached === undefined) {
        return undefined;
      }
      found.push(...reached);
    }
    return found;
  }
  const step = lookup(current, head);
  if (!step.ok) {
    return undefined;
  }
  return walk(step.value, rest);
}

/** Compares an expected tree against the response's, reporting the paths that differ. */
function match(path: string, want: unknown, got: unknown): string[] {
  if (want instanceof Literal) {
    return matchNumber(path, want.text, got);
  }
  if (Array.isArray(want)) {
    if (!Array.isArray(got)) {
      return [`${at(path)}: ${render(got)}, want a list`];
    }
    if (got.length !== want.length) {
      return [`${at(path)}: ${got.length} entries, want ${want.length} (${render(got)})`];
    }
    return want.flatMap((item, index) => match(join(path, String(index)), item, got[index]));
  }
  if (isObject(want)) {
    if (!isObject(got)) {
      return [`${at(path)}: ${render(got)}, want an object`];
    }
    const failures: string[] = [];
    for (const key of sortedKeys(want)) {
      const child = join(path, key);
      if (!(key in got)) {
        // An unset field and a field left at its default are the same thing on
        // the wire, so a default expectation still matches.
        if (!isDefault(want[key])) {
          failures.push(`${child}: not set, want ${render(want[key])}`);
        }
        continue;
      }
      failures.push(...match(child, want[key], got[key]));
    }
    return failures;
  }
  if (typeof want === "number") {
    return matchNumber(path, String(want), got);
  }
  if (String(want) !== String(got)) {
    return [`${at(path)}: ${render(got)}, want ${render(want)}`];
  }
  return [];
}

/** Compares an expected number, as the scenario wrote it, against the response's. */
function matchNumber(path: string, want: string, got: unknown): string[] {
  if (got instanceof Integer) {
    const digits = integerLiteral(want);
    if (digits === undefined || digits !== got.digits) {
      return [`${at(path)}: ${got.digits}, want ${want}`];
    }
    return [];
  }
  if (typeof got === "number") {
    const number = Number(want);
    if (Number.isNaN(number) || !near(got, number)) {
      return [`${at(path)}: ${got}, want ${want}`];
    }
    return [];
  }
  return [`${at(path)}: ${render(got)}, want the number ${want}`];
}

/** An expected number as the digits of a whole number, also accepting "1500.0". */
function integerLiteral(text: string): string | undefined {
  if (/^[+-]?\d+$/.test(text)) {
    return BigInt(text).toString();
  }
  const number = Number(text);
  if (!Number.isFinite(number) || !Number.isInteger(number) || !Number.isSafeInteger(number)) {
    return undefined;
  }
  return BigInt(number).toString();
}

/** Whether two Real values are the same within the tolerance. */
function near(got: number, want: number): boolean {
  if (got === want) {
    return true;
  }
  const scale = Math.max(Math.abs(got), Math.abs(want));
  return Math.abs(got - want) <= TOLERANCE * scale;
}

/** Whether a value is what an unset field holds. */
export function isDefault(value: unknown): boolean {
  if (value instanceof Literal) {
    return Number(value.text) === 0;
  }
  if (value instanceof Integer) {
    return BigInt(value.digits) === 0n;
  }
  if (value === null || value === undefined) {
    return true;
  }
  switch (typeof value) {
    case "boolean":
      return !value;
    case "number":
      return value === 0;
    case "bigint":
      return value === 0n;
    case "string":
      return value === "";
    default:
      break;
  }
  if (Array.isArray(value)) {
    return value.length === 0;
  }
  if (isObject(value)) {
    return Object.keys(value).length === 0;
  }
  return false;
}

/** Walks a dotted path into a normalized response: fields, map keys, list indices. */
export function lookup(tree: unknown, path: string): { ok: true; value: unknown } | { ok: false } {
  let current = tree;
  for (const segment of path.split(".")) {
    if (Array.isArray(current)) {
      const index = Number(segment);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) {
        return { ok: false };
      }
      current = current[index];
      continue;
    }
    if (isObject(current)) {
      if (!(segment in current)) {
        return { ok: false };
      }
      current = current[segment];
      continue;
    }
    return { ok: false };
  }
  return { ok: true, value: current };
}

/** The number of entries of the list or map at `path`. */
function count(tree: Normalized, path: string): number | undefined {
  const found = lookup(tree, path);
  if (!found.ok) {
    // An empty list or map is an unset field, so nothing there is zero entries.
    return 0;
  }
  if (Array.isArray(found.value)) {
    return found.value.length;
  }
  if (isObject(found.value)) {
    return Object.keys(found.value).length;
  }
  return undefined;
}

function numberOf(value: Literal | number | undefined): number {
  if (value instanceof Literal) {
    return Number(value.text);
  }
  return value ?? 0;
}

function at(path: string): string {
  return path === "" ? "response" : path;
}

function join(path: string, segment: string): string {
  return path === "" ? segment : `${path}.${segment}`;
}

function sortedKeys(value: Record<string, unknown> | undefined): string[] {
  return value === undefined ? [] : Object.keys(value).sort(byCodeUnit);
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value) && !(value instanceof Integer) && !(value instanceof Literal);
}

function quote(text: string): string {
  return JSON.stringify(text);
}

/** A value as a scenario would write it. */
export function render(value: unknown): string {
  if (typeof value === "string") {
    return quote(value);
  }
  if (value === null || value === undefined) {
    return "nothing";
  }
  if (value instanceof Integer) {
    return value.digits;
  }
  if (value instanceof Literal) {
    return value.text;
  }
  if (Array.isArray(value)) {
    return `[${value.map(render).join(", ")}]`;
  }
  if (isObject(value)) {
    return `{${sortedKeys(value)
      .map((key) => `${key}: ${render(value[key])}`)
      .join(", ")}}`;
  }
  return typeof value === "bigint" ? value.toString() : JSON.stringify(value);
}
