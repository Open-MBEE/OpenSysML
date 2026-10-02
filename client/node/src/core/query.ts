// Querying a model the way the SysML v2 API & Services standard queries one:
// scope / select / where, where `where` is a primitive or composite constraint.
// Accepts the standard's JSON verbatim, or the same thing as options.

import { create } from "@bufbuild/protobuf";
import type {
  Constraint,
  Query,
  QueryResponse,
} from "../generated/sysml_pb.js";
import {
  CompositeOperator,
  ConstraintSchema,
  PrimitiveOperator,
  QuerySchema,
} from "../generated/sysml_pb.js";
import { byCodeUnit } from "./capabilities.js";
import { QueryError } from "./errors.js";

const TYPE_KEY = "@type";

/** `@type` of the standard's query resource. */
export const TYPE_QUERY = "Query";
/** `@type` of a single-property constraint. */
export const TYPE_PRIMITIVE_CONSTRAINT = "PrimitiveConstraint";
/** `@type` of a constraint over nested constraints. */
export const TYPE_COMPOSITE_CONSTRAINT = "CompositeConstraint";

const PRIMITIVE_OPERATORS = new Map<string, PrimitiveOperator>([
  ["=", PrimitiveOperator.EQUAL],
  [">", PrimitiveOperator.GREATER],
  ["<", PrimitiveOperator.LESS],
]);

const COMPOSITE_OPERATORS = new Map<string, CompositeOperator>([
  ["and", CompositeOperator.AND],
  ["or", CompositeOperator.OR],
]);

/** An element a query selected. */
export class QueryElement {
  /** Qualified name of the element, which is what a scope names it by. */
  readonly id: string;
  /** The element's metamodel type, e.g. `PartUsage`. */
  readonly type: string;
  /** The selected properties it has; one it does not have is absent. */
  readonly properties: Readonly<Record<string, string>>;

  constructor(init: { id: string; type: string; properties: Record<string, string> }) {
    this.id = init.id;
    this.type = init.type;
    this.properties = { ...init.properties };
  }

  /** Value of a selected property, or `fallback` if the element has none. */
  get(propertyName: string, fallback?: string): string | undefined {
    return this.properties[propertyName] ?? fallback;
  }

  /** The element as the standard's JSON names it: `@id`, `@type`, properties. */
  asDict(): Record<string, string> {
    return { "@id": this.id, "@type": this.type, ...this.properties };
  }

  toString(): string {
    return `${this.id} (${this.type})`;
  }
}

/** A scope entry: an element's qualified name or an `@id` reference. */
export type ScopeEntry = string | { "@id": string };

/** A constraint, as the standard's JSON writes it. */
export type ConstraintPayload = Record<string, unknown>;

/** The standard's `Query` object, as JSON. */
export type QueryPayload = Record<string, unknown>;

/** Keyword form of a query, exclusive with `payload`. */
export interface QueryForm {
  scope?: readonly ScopeEntry[];
  select?: readonly string[];
  where?: ConstraintPayload;
  /** An OSLC query text, evaluated instead of the structured query. */
  oslc?: string;
  /** A query already decoded for the wire; passes through as written. */
  query?: Query;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Translate a query, given as the standard's JSON or as keywords, to protobuf. */
export function buildQuery(options: { payload?: QueryPayload } & QueryForm = {}): Query {
  const { payload } = options;
  let { scope, select, where } = options;
  if (payload !== undefined) {
    if (scope !== undefined || select !== undefined || where !== undefined) {
      throw new QueryError("pass a query payload or scope/select/where keywords, not both");
    }
    if (!isRecord(payload)) {
      throw new QueryError("a query is an object, not a " + typeof payload);
    }
    const declared = payload[TYPE_KEY] ?? TYPE_QUERY;
    if (declared !== TYPE_QUERY) {
      throw new QueryError(`expected a ${JSON.stringify(TYPE_QUERY)} payload, got ${JSON.stringify(declared)}`);
    }
    const unknown = Object.keys(payload).filter(
      (key) => ![TYPE_KEY, "@id", "owningProject", "scope", "select", "where"].includes(key),
    );
    if (unknown.length > 0) {
      throw new QueryError(
        `a query has no ${unknown.sort(byCodeUnit).join(", ")}; the standard's query is scope, select and where`,
      );
    }
    scope = payload.scope as QueryForm["scope"];
    select = payload.select as QueryForm["select"];
    where = payload.where as QueryForm["where"];
  }

  const query = create(QuerySchema, {
    scope: sequence("scope", scope).map(scopeId),
    select: sequence("select", select).map(propertyName),
  });
  if (where !== undefined) {
    query.where = constraint(where);
  }
  return query;
}

function scopeId(entry: unknown): string {
  if (typeof entry === "string") {
    return entry;
  }
  if (isRecord(entry) && typeof entry["@id"] === "string") {
    return entry["@id"];
  }
  throw new QueryError(
    `a scope entry is an element's qualified name or a {'@id': ...} reference, not ${JSON.stringify(entry)}`,
  );
}

function propertyName(entry: unknown): string {
  if (typeof entry !== "string") {
    throw new QueryError(`a selected property is a name, not ${JSON.stringify(entry)}`);
  }
  return entry;
}

function sequence(field: string, value: unknown): unknown[] {
  if (value === undefined || value === null) {
    return [];
  }
  if (typeof value === "string" || isRecord(value)) {
    return [value];
  }
  if (!Array.isArray(value)) {
    throw new QueryError(`${field} is a list, not ${typeof value}`);
  }
  return value;
}

function constraint(payload: unknown): Constraint {
  if (!isRecord(payload)) {
    throw new QueryError(`a constraint is an object, not ${typeof payload}`);
  }
  let declared = payload[TYPE_KEY];
  if (declared === undefined) {
    declared = "constraint" in payload ? TYPE_COMPOSITE_CONSTRAINT : TYPE_PRIMITIVE_CONSTRAINT;
  }
  if (declared === TYPE_PRIMITIVE_CONSTRAINT) {
    return create(ConstraintSchema, {
      constraint: { case: "primitive", value: primitive(payload) },
    });
  }
  if (declared === TYPE_COMPOSITE_CONSTRAINT) {
    return create(ConstraintSchema, {
      constraint: { case: "composite", value: composite(payload) },
    });
  }
  throw new QueryError(
    `unknown constraint type ${JSON.stringify(declared)}; the standard's constraints are ` +
      `${TYPE_PRIMITIVE_CONSTRAINT} and ${TYPE_COMPOSITE_CONSTRAINT}`,
  );
}

function rejectUnknown(payload: Record<string, unknown>, known: string[], what: string): void {
  const unknown = Object.keys(payload).filter((key) => !known.includes(key));
  if (unknown.length > 0) {
    throw new QueryError(`a ${what} has no ${unknown.sort(byCodeUnit).join(", ")}`);
  }
}

function primitive(payload: Record<string, unknown>) {
  rejectUnknown(payload, [TYPE_KEY, "@id", "inverse", "property", "operator", "value"], TYPE_PRIMITIVE_CONSTRAINT);
  const operator = payload.operator;
  const mapped = typeof operator === "string" ? PRIMITIVE_OPERATORS.get(operator) : undefined;
  if (mapped === undefined) {
    throw new QueryError(
      `unknown primitive operator ${JSON.stringify(operator)}; expected one of ` +
        [...PRIMITIVE_OPERATORS.keys()].sort(byCodeUnit).join(", "),
    );
  }
  const name = payload.property;
  if (typeof name !== "string" || name === "") {
    throw new QueryError(`a primitive constraint names one property, not ${JSON.stringify(name)}`);
  }
  return {
    inverse: payload.inverse === true,
    property: name,
    operator: mapped,
    value: values(payload.value),
  };
}

function composite(payload: Record<string, unknown>) {
  rejectUnknown(payload, [TYPE_KEY, "@id", "constraint", "operator"], TYPE_COMPOSITE_CONSTRAINT);
  const operator = payload.operator;
  const mapped = typeof operator === "string" ? COMPOSITE_OPERATORS.get(operator) : undefined;
  if (mapped === undefined) {
    throw new QueryError(
      `unknown composite operator ${JSON.stringify(operator)}; expected one of ` +
        [...COMPOSITE_OPERATORS.keys()].sort(byCodeUnit).join(", "),
    );
  }
  const nested = payload.constraint;
  if (!Array.isArray(nested) || nested.length === 0) {
    throw new QueryError(
      `a composite constraint combines a non-empty list of constraints, not ${JSON.stringify(nested)}`,
    );
  }
  return {
    operator: mapped,
    constraint: nested.map(constraint),
  };
}

function values(value: unknown): string[] {
  if (value === undefined || value === null) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.map(compared);
  }
  return [compared(value)];
}

function compared(value: unknown): string {
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "bigint") {
    return String(value);
  }
  throw new QueryError(`cannot compare against ${JSON.stringify(value)}`);
}

/** The elements a `QueryResponse` reports, as `QueryElement` records. */
export function elementsOf(response: QueryResponse): QueryElement[] {
  return response.elements.map(
    (element) => new QueryElement({ id: element.id, type: element.type, properties: element.properties }),
  );
}
