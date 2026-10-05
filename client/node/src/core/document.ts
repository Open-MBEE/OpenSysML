// Native document queries: a calc def specializing `DocumentQueries::Query`,
// answered as typed rows. Binding values are plain values, an ElementRef
// naming a model element, or an ObjectRef naming an object the service holds.

import { create } from "@bufbuild/protobuf";
import type {
  DocumentObject as PbDocumentObject,
  DocumentEvent as PbDocumentEvent,
  DocumentQueryBinding,
  DocumentQueryRow,
  DocumentValue as PbDocumentValue,
  Quantity,
  RunDocumentQueryResponse,
} from "../generated/sysml_pb.js";
import {
  DocumentObjectSchema,
  DocumentQueryBindingSchema,
  DocumentValueSchema,
  QuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
} from "../generated/sysml_pb.js";
import { DocumentQueryError, UnsupportedValueError } from "./errors.js";
import {
  decodeBigInteger,
  encodeQuantityMagnitude,
  fitsInt64,
  formatValue,
  type Magnitude,
  type SysMLValue,
} from "./values.js";

/** Whether a wire binding sends an Integer beyond int64, which needs `big_int_values`. */
export function bindingHoldsBigInt(binding: DocumentQueryBinding): boolean {
  return binding.values.some(
    (value) =>
      value.kind.case === "bigIntValue" ||
      (value.kind.case === "quantity" && value.kind.value.magnitude.case === "bigIntMagnitude"),
  );
}

/** A model element, named by qualified name. */
export class ElementRef {
  /** Qualified name of the element. */
  readonly id: string;
  /** Metamodel type name; empty when bound by a caller, reported when answered. */
  readonly type: string;

  constructor(id: string, type = "") {
    this.id = id;
    this.type = type;
  }

  toString(): string {
    return this.type !== "" ? `${this.id} (${this.type})` : this.id;
  }
}

/** An object the service holds for the model, created by `instantiate`. */
export class ObjectRef {
  /** The object's id, as `instantiate` answered it. */
  readonly id: bigint;
  /** The object by the label a session reaches it under. */
  readonly path: string;
  /** The usage the object is held under; reported when answered, ignored when bound. */
  readonly element: ElementRef | undefined;

  constructor(options: { id?: bigint | number; path?: string; element?: ElementRef } = {}) {
    this.id = typeof options.id === "number" ? BigInt(options.id) : (options.id ?? 0n);
    this.path = options.path ?? "";
    this.element = options.element;
  }

  toString(): string {
    return this.path !== "" ? this.path : `#${this.id.toString()}`;
  }
}

/** A row a `Verdicts` query answered: an assertion checked on one object. Answered only. */
export class DocumentVerdict {
  readonly assertion: ElementRef;
  /** "constraint", "requirement", "satisfaction" or "verification". */
  readonly kind: string;
  /** The assertion as written. */
  readonly text: string;
  /** The object checked, named from the element the query was bound to. */
  readonly path: string;
  /** "holds", "violated" or "undecided". */
  readonly status: string;
  /** The condition that evaluated to false, as written; empty otherwise. */
  readonly condition: string;
  /** Why the assertion is violated or undecided; empty when it holds. */
  readonly reason: string;
  /** Verdict kinds of the verification cases verifying the row's requirement. */
  readonly verification: readonly string[];

  constructor(init: {
    assertion: ElementRef;
    kind: string;
    text: string;
    path: string;
    status: string;
    condition?: string;
    reason?: string;
    verification?: readonly string[];
  }) {
    this.assertion = init.assertion;
    this.kind = init.kind;
    this.text = init.text;
    this.path = init.path;
    this.status = init.status;
    this.condition = init.condition ?? "";
    this.reason = init.reason ?? "";
    this.verification = init.verification ?? [];
  }

  toString(): string {
    const where = this.path !== "" ? ` on ${this.path}` : "";
    return `${this.text}${where}: ${this.status}`;
  }
}

/** A row a `States` query answered: one active leaf state of one object. Answered only. */
export class DocumentState {
  /** The object whose state machine the row reads. */
  readonly object: ObjectRef;
  /** The exhibited state usage's name, or the state def's. */
  readonly machine: string;
  /** The active leaf state's own name. */
  readonly name: string;
  /** The leaf's path from the machine's top level. */
  readonly path: string;
  /** The state usage's declaration. */
  readonly state: ElementRef | undefined;
  /** The orthogonal region declaring the leaf; empty when not in one. */
  readonly region: string;
  /** The active composite states around the leaf, outermost first. */
  readonly enclosing: readonly string[];

  constructor(init: {
    object: ObjectRef;
    machine: string;
    name: string;
    path: string;
    state?: ElementRef;
    region?: string;
    enclosing?: readonly string[];
  }) {
    this.object = init.object;
    this.machine = init.machine;
    this.name = init.name;
    this.path = init.path;
    this.state = init.state;
    this.region = init.region ?? "";
    this.enclosing = init.enclosing ?? [];
  }

  toString(): string {
    return `${this.object.toString()}.${this.machine} in ${this.path}`;
  }
}

/** A row an `Events` query answered: one record of a session's trace. Answered only. */
export class DocumentEvent {
  /** "accept", "send", "transition", "entry", "exit", "do", "choice", "guard" or "terminate". */
  readonly kind: string;
  /** The instant the record was written at: a quantity, an int or a real. */
  readonly time: DocumentValue;
  /** The object the record is about; undefined for a record of the run as a whole. */
  readonly object: ObjectRef | undefined;
  /** The state machine the record is about. */
  readonly machine: string;
  /** The state entered, exited or run (entry, exit and do records). */
  readonly state: string;
  /** The transition's source state. */
  readonly fromState: string;
  /** The transition's target state. */
  readonly toState: string;
  /** The object a send was delivered to. */
  readonly target: ObjectRef | undefined;
  /** The accepted or sent event's type name. */
  readonly event: string;
  /** The accept's payload, one `name = value` text per attribute. */
  readonly payload: readonly string[];
  /** What a choice drew from, in order. */
  readonly alternatives: readonly string[];
  /** The alternative the choice took. */
  readonly taken: string;
  /** The record as the trace prints it. */
  readonly text: string;

  constructor(init: {
    kind: string;
    time: DocumentValue;
    text: string;
    object?: ObjectRef;
    machine?: string;
    state?: string;
    fromState?: string;
    toState?: string;
    target?: ObjectRef;
    event?: string;
    payload?: readonly string[];
    alternatives?: readonly string[];
    taken?: string;
  }) {
    this.kind = init.kind;
    this.time = init.time;
    this.text = init.text;
    this.object = init.object;
    this.machine = init.machine ?? "";
    this.state = init.state ?? "";
    this.fromState = init.fromState ?? "";
    this.toState = init.toState ?? "";
    this.target = init.target;
    this.event = init.event ?? "";
    this.payload = init.payload ?? [];
    this.alternatives = init.alternatives ?? [];
    this.taken = init.taken ?? "";
  }

  toString(): string {
    const time = this.time;
    const rendered =
      time instanceof ElementRef ||
      time instanceof ObjectRef ||
      time instanceof DocumentVerdict ||
      time instanceof DocumentState ||
      time instanceof DocumentEvent
        ? time.toString()
        : renderPlainTime(time);
    return `${rendered}: ${this.text}`;
  }
}

/** What a binding value or an answered cell value may be. */
/** A time that is a plain value: a SysML value formatted, anything else as JavaScript spells it. */
function renderPlainTime(time: SysMLValue | string | bigint | number | boolean): string {
  return typeof time === "object" ? formatValue(time) : String(time);
}

export type DocumentValue =
  | ElementRef
  | ObjectRef
  | DocumentVerdict
  | DocumentState
  | DocumentEvent
  | string
  | bigint
  | number
  | boolean
  | { kind: "infinity" }
  | ({ kind: "quantity" } & Extract<SysMLValue, { kind: "quantity" }>);

/** What `bindings` accepts for one parameter: one value or several. */
export type BindingValues = DocumentValue | readonly DocumentValue[];

/** One selected element and its projected cells, one per column. */
export class DocumentRow {
  /** The selected element itself, or the usage the row's object is held under. */
  readonly element: ElementRef;
  /** One value sequence per column, in column order. */
  readonly cells: readonly (readonly DocumentValue[])[];
  /** The verdict a `Verdicts` row carries; undefined for any other row. */
  readonly verdict: DocumentVerdict | undefined;
  /** The object a row over held objects is about; undefined for any other row. */
  readonly object: ObjectRef | undefined;
  /** The state a `States` row carries; undefined for any other row. */
  readonly state: DocumentState | undefined;
  /** The event an `Events` row carries; undefined for any other row. */
  readonly event: DocumentEvent | undefined;

  constructor(init: {
    element: ElementRef;
    cells: readonly (readonly DocumentValue[])[];
    verdict?: DocumentVerdict;
    object?: ObjectRef;
    state?: DocumentState;
    event?: DocumentEvent;
  }) {
    this.element = init.element;
    this.cells = init.cells;
    this.verdict = init.verdict;
    this.object = init.object;
    this.state = init.state;
    this.event = init.event;
  }

  /** The values of the `index`-th column. */
  cell(index: number): readonly DocumentValue[] {
    if (index < 0 || index >= this.cells.length) {
      throw new RangeError(`the row has ${this.cells.length} cells, not ${index + 1}`);
    }
    return this.cells[index];
  }
}

/** A document query's answer: projected columns and typed rows, in the engine's order. */
export class DocumentQueryResult {
  /** Projected property names, in projection order. */
  readonly columns: readonly string[];
  /** The selected rows, in the engine's order. */
  readonly rows: readonly DocumentRow[];

  constructor(init: { columns: readonly string[]; rows: readonly DocumentRow[] }) {
    this.columns = init.columns;
    this.rows = init.rows;
  }

  get length(): number {
    return this.rows.length;
  }

  [Symbol.iterator](): Iterator<DocumentRow> {
    return this.rows[Symbol.iterator]();
  }
}

/** Translate a bindings mapping into the RPC's protobuf. */
export function buildBindings(
  bindings: Readonly<Record<string, BindingValues>> | undefined,
): DocumentQueryBinding[] {
  if (bindings === undefined) {
    return [];
  }
  const out: DocumentQueryBinding[] = [];
  for (const [parameter, values] of Object.entries(bindings)) {
    const list: readonly DocumentValue[] = Array.isArray(values) ? values : [values];
    out.push(
      create(DocumentQueryBindingSchema, {
        parameter,
        values: list.map((value) => boundValue(parameter, value)),
      }),
    );
  }
  return out;
}

function boundValue(parameter: string, value: DocumentValue): PbDocumentValue {
  if (value instanceof ElementRef) {
    return create(DocumentValueSchema, { kind: { case: "elementId", value: value.id } });
  }
  if (value instanceof ObjectRef) {
    if (value.id === 0n && value.path === "") {
      throw new DocumentQueryError(
        `binding ${JSON.stringify(parameter)} cannot carry ${value.toString()}: an object is ` +
          `bound by id or by path; neither was given`,
      );
    }
    return create(DocumentValueSchema, {
      kind: {
        case: "object",
        value: create(DocumentObjectSchema, { instanceId: value.id, path: value.path }),
      },
    });
  }
  if (typeof value === "boolean") {
    return create(DocumentValueSchema, { kind: { case: "boolValue", value } });
  }
  if (typeof value === "string") {
    return create(DocumentValueSchema, { kind: { case: "stringValue", value } });
  }
  if (typeof value === "bigint") {
    return fitsInt64(value)
      ? create(DocumentValueSchema, { kind: { case: "intValue", value } })
      : create(DocumentValueSchema, { kind: { case: "bigIntValue", value: value.toString() } });
  }
  if (typeof value === "number") {
    return create(DocumentValueSchema, { kind: { case: "realValue", value } });
  }
  if (value instanceof DocumentVerdict) {
    throw new DocumentQueryError(
      `binding ${JSON.stringify(parameter)} cannot carry ${value.toString()}: a verdict is ` +
        `answered by queries, not bound to them`,
    );
  }
  if (value instanceof DocumentState || value instanceof DocumentEvent) {
    const what = value instanceof DocumentState ? "a state" : "an event";
    throw new DocumentQueryError(
      `binding ${JSON.stringify(parameter)} cannot carry ${value.toString()}: ${what} row is ` +
        `answered by queries, not bound to them`,
    );
  }
  if (typeof value === "object" && "kind" in value && value.kind === "quantity") {
    return create(DocumentValueSchema, {
      kind: { case: "quantity", value: boundQuantity(value) },
    });
  }
  throw new DocumentQueryError(
    `binding ${JSON.stringify(parameter)} cannot carry ${JSON.stringify(value)}: a binding is a ` +
      `str, int, float, bool, Quantity, ElementRef or ObjectRef`,
  );
}

/** A wire quantity's magnitude as a value; one carrying none reads as a real zero. */
function decodeMagnitude(magnitude: Quantity["magnitude"]): Magnitude {
  switch (magnitude.case) {
    case "intMagnitude":
      return { kind: "int", value: magnitude.value };
    case "bigIntMagnitude":
      return { kind: "int", value: decodeBigInteger(magnitude.value) };
    case "realMagnitude":
      return { kind: "real", value: magnitude.value };
    default:
      return { kind: "real", value: 0 };
  }
}

function boundQuantity(
  value: Extract<SysMLValue, { kind: "quantity" }>,
): ReturnType<typeof create<typeof QuantitySchema>> {
  const magnitude = value.magnitude;
  return create(QuantitySchema, {
    magnitude: encodeQuantityMagnitude(magnitude),
    unit: value.unit,
    ...(value.unitTerm === undefined
      ? {}
      : {
          unitTerm: create(UnitTermSchema, {
            scaleNum: value.unitTerm.scaleNum,
            scaleDen: value.unitTerm.scaleDen,
            factors: value.unitTerm.factors.map((factor) =>
              create(UnitFactorSchema, { unitId: factor.unitId, exponent: factor.exponent }),
            ),
          }),
        }),
  });
}

/** Decode a `RunDocumentQueryResponse` into a `DocumentQueryResult`. */
export function documentResult(response: RunDocumentQueryResponse): DocumentQueryResult {
  return new DocumentQueryResult({
    columns: response.columns.map((column) => column.name),
    rows: response.rows.map(rowOf),
  });
}

function rowOf(row: DocumentQueryRow): DocumentRow {
  const cells = row.cells.map((cell) => cell.values.map(valueOf));
  const kind = row.element?.kind;
  if (kind?.case === "verdict") {
    const verdict = valueOf(row.element) as DocumentVerdict;
    return new DocumentRow({ element: verdict.assertion, cells, verdict });
  }
  if (kind?.case === "object") {
    const obj = objectOf(kind.value);
    return new DocumentRow({ element: obj.element ?? new ElementRef(""), cells, object: obj });
  }
  if (kind?.case === "state") {
    const state = stateOf(kind.value);
    return new DocumentRow({
      element: state.object.element ?? new ElementRef(""),
      cells,
      object: state.object,
      state,
    });
  }
  if (kind?.case === "event") {
    const event = eventOf(kind.value);
    return new DocumentRow({
      element: event.object?.element ?? new ElementRef(""),
      cells,
      ...(event.object === undefined ? {} : { object: event.object }),
      event,
    });
  }
  return new DocumentRow({ element: elementOf(row.element), cells });
}

function elementOf(value: PbDocumentValue | undefined): ElementRef {
  if (value?.kind.case === "elementId") {
    return new ElementRef(value.kind.value, value.elementType);
  }
  return new ElementRef("", value?.elementType ?? "");
}

function objectOf(obj: PbDocumentObject): ObjectRef {
  return new ObjectRef({ id: obj.instanceId, path: obj.path, element: elementOf(obj.element) });
}

function stateOf(state: NonNullable<Extract<PbDocumentValue["kind"], { case: "state" }>["value"]>): DocumentState {
  return new DocumentState({
    object: state.object === undefined ? new ObjectRef() : objectOf(state.object),
    machine: state.machine,
    name: state.name,
    path: state.statePath,
    ...(state.state === undefined ? {} : { state: elementOf(state.state) }),
    region: state.region,
    enclosing: [...state.enclosing],
  });
}

function eventOf(event: NonNullable<Extract<PbDocumentValue["kind"], { case: "event" }>["value"]>): DocumentEvent {
  return new DocumentEvent({
    kind: event.kind,
    time: valueOf(event.time),
    text: event.text,
    ...(event.object === undefined ? {} : { object: objectOf(event.object) }),
    machine: event.machine,
    state: event.state,
    fromState: event.from,
    toState: event.to,
    ...(event.target === undefined ? {} : { target: objectOf(event.target) }),
    event: event.event,
    payload: [...event.payload],
    alternatives: [...event.alternatives],
    taken: event.taken,
  });
}

/** Decode a run-trace event using the document query's event conversion. */
export function documentEventOf(event: PbDocumentEvent): DocumentEvent {
  return eventOf(event);
}

function valueOf(value: PbDocumentValue | undefined): DocumentValue {
  const kind = value?.kind;
  switch (kind?.case) {
    case "elementId":
      return new ElementRef(kind.value, value?.elementType ?? "");
    case "stringValue":
      return kind.value;
    case "intValue":
      return kind.value;
    case "bigIntValue":
      return decodeBigInteger(kind.value);
    case "realValue":
      return kind.value;
    case "boolValue":
      return kind.value;
    case "infinity":
      return { kind: "infinity" };
    case "quantity": {
      const magnitude = kind.value.magnitude;
      const decoded: Extract<SysMLValue, { kind: "quantity" }> = {
        kind: "quantity",
        magnitude: decodeMagnitude(magnitude),
        unit: kind.value.unit,
      };
      return decoded;
    }
    case "object":
      return objectOf(kind.value);
    case "verdict": {
      const verdict = kind.value;
      return new DocumentVerdict({
        assertion: elementOf(verdict.assertion),
        kind: verdict.kind,
        text: verdict.text,
        path: verdict.path,
        status: verdict.verdict,
        condition: verdict.condition,
        reason: verdict.reason,
        verification: [...verdict.verification],
      });
    }
    case "state":
      return stateOf(kind.value);
    case "event":
      return eventOf(kind.value);
    default:
      throw new UnsupportedValueError(
        `the service answered a document value this client cannot read: ${JSON.stringify(kind?.case)}`,
      );
  }
}
