// Reading conformance/scenarios/*.json. The scenarios are the specification;
// this file only decodes them, and refuses a field it does not understand.

import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

/**
 * A number as the scenario wrote it. The digits are kept so a whole number too
 * large for a double is still compared, and written back, exactly.
 */
export class Literal {
  constructor(readonly text: string) {}

  /** The value as protobuf-JSON: digits for a 64-bit integer, a number otherwise. */
  toJSON(): number | string {
    const value = Number(this.text);
    return Number.isInteger(value) && !Number.isSafeInteger(value) ? this.text : value;
  }
}

/**
 * The source a scenario needs parsed before its call: one fixture, or several
 * parsed together as one model, named by their fixtures.
 */
export interface ScenarioModel {
  fixture?: string;
  fixtures?: string[];
  language?: string;
  strict_conformance?: boolean;
}

/** What a call must answer. Every field is optional; an absent status means OK. */
export interface Expect {
  status?: string;
  status_message_contains?: string;
  response?: Record<string, unknown>;
  contains?: Record<string, string>;
  contains_all?: Record<string, string[]>;
  non_empty?: string[];
  absent?: string[];
  /** Values are Literal, because a scenario's numbers are kept as written. */
  counts?: Record<string, Literal>;
  min_counts?: Record<string, Literal>;
}

/** One conformance case: a call to make and what it must answer. */
export interface Scenario {
  id: string;
  description?: string;
  rpc: string;
  requires_capabilities?: string[];
  expect_without_capability?: Expect;
  model?: ScenarioModel;
  request?: Record<string, unknown>;
  expect?: Expect;
  /** The file it came from, for error messages. */
  file: string;
}

const SCENARIO_FIELDS = new Set([
  "id",
  "description",
  "rpc",
  "requires_capabilities",
  "expect_without_capability",
  "model",
  "request",
  "expect",
]);

const EXPECT_FIELDS = new Set([
  "status",
  "status_message_contains",
  "response",
  "contains",
  "contains_all",
  "non_empty",
  "absent",
  "counts",
  "min_counts",
]);

const MODEL_FIELDS = new Set(["fixture", "fixtures", "language", "strict_conformance"]);

/** The scenario's RPC as a bare method name. */
export function methodOf(scenario: Scenario): string {
  const slash = scenario.rpc.lastIndexOf("/");
  return slash === -1 ? scenario.rpc : scenario.rpc.slice(slash + 1);
}

/** Reads every scenario file in `dir`, in file then declaration order. */
export function loadScenarios(dir: string): Scenario[] {
  const files = readdirSync(dir)
    .filter((name) => name.endsWith(".json"))
    .sort();
  if (files.length === 0) {
    throw new Error(`no scenario files in ${dir}`);
  }
  const scenarios: Scenario[] = [];
  const seen = new Map<string, string>();
  for (const name of files) {
    const path = join(dir, name);
    const suite = parseKeepingLiterals(readFileSync(path, "utf8"));
    for (const scenario of scenariosOf(suite, path)) {
      const where = seen.get(scenario.id);
      if (where !== undefined) {
        throw new Error(`${path}: scenario id ${JSON.stringify(scenario.id)} is already declared in ${where}`);
      }
      seen.set(scenario.id, path);
      scenarios.push(scenario);
    }
  }
  return scenarios;
}

/** Parses a scenario file, keeping every number as the literal it was written as. */
function parseKeepingLiterals(text: string): unknown {
  return JSON.parse(text, function (_key: string, value: unknown, context?: { source?: string }) {
    const source = context?.source;
    return typeof value === "number" && source !== undefined ? new Literal(source) : value;
  }) as unknown;
}

function scenariosOf(suite: unknown, path: string): Scenario[] {
  if (!isObject(suite) || !Array.isArray(suite["scenarios"])) {
    throw new Error(`${path}: expected an object with a "scenarios" list`);
  }
  return suite["scenarios"].map((entry: unknown) => scenarioOf(entry, path));
}

function scenarioOf(entry: unknown, path: string): Scenario {
  if (!isObject(entry)) {
    throw new Error(`${path}: a scenario must be an object`);
  }
  reject(entry, SCENARIO_FIELDS, path, "a scenario");
  const id = entry["id"];
  const rpc = entry["rpc"];
  if (typeof id !== "string" || id === "" || typeof rpc !== "string" || rpc === "") {
    throw new Error(`${path}: every scenario needs an id and an rpc`);
  }
  const model = entry["model"];
  if (model !== undefined) {
    if (!isObject(model)) {
      throw new Error(`${path}: ${id}: model must be an object`);
    }
    reject(model, MODEL_FIELDS, path, `${id}: model`);
  }
  for (const key of ["expect", "expect_without_capability"] as const) {
    const expect = entry[key];
    if (expect !== undefined) {
      if (!isObject(expect)) {
        throw new Error(`${path}: ${id}: ${key} must be an object`);
      }
      reject(expect, EXPECT_FIELDS, path, `${id}: ${key}`);
    }
  }
  return { ...(entry as unknown as Omit<Scenario, "file">), file: path };
}

function reject(node: Record<string, unknown>, known: ReadonlySet<string>, path: string, what: string): void {
  for (const key of Object.keys(node)) {
    if (!known.has(key)) {
      throw new Error(`${path}: ${what} has no field ${JSON.stringify(key)}`);
    }
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
