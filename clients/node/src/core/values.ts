// Values, quantities and verdicts as discriminated unions: a caller switches on
// `kind` and the compiler checks the switch is exhaustive.

import { create } from "@bufbuild/protobuf";
import type {
  Array as ArrayMessage,
  Bound,
  EnumLiteral,
  Function as FunctionMessage,
  MeasurementRef,
  Quantity,
  TensorQuantity,
  UnitTerm,
  Value,
  ValueSet,
  Vector,
  VectorQuantity,
  Verdict,
} from "../generated/sysml_pb.js";
import {
  ArraySchema,
  ComplexSchema,
  EnumLiteralSchema,
  FailureReason,
  FunctionSchema,
  MeasurementRefSchema,
  QuantitySchema,
  TensorQuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
  ValueSchema,
  ValueSequenceSchema,
  ValueSetSchema,
  VectorQuantitySchema,
  VectorSchema,
} from "../generated/sysml_pb.js";
import { MalformedValueError, type FailureCause } from "./errors.js";

/** A quantity's magnitude: an integer or a real, never both. */
export type Magnitude = { kind: "int"; value: bigint } | { kind: "real"; value: number };

/** One unit raised to an exponent, as the service factorises a derived unit. */
export interface UnitFactor {
  unitId: string;
  exponent: number;
}

/** A unit as a scale factor over base-unit powers, when the service reports one. */
export interface UnitFactorization {
  scaleNum: number;
  scaleDen: number;
  factors: UnitFactor[];
}

/** A complex number in rectangular form: one value, never a sequence of two reals. */
export interface ComplexValue {
  real: number;
  imaginary: number;
}

/** An enumeration literal, identified by the enumeration that declares it. */
export interface EnumValue {
  name: string;
  literalId: string;
  enumerationId: string;
  /** The scalar the literal equals when its enumeration specializes a scalar type (`high = 3`). */
  value?: SysMLValue;
}

/** A magnitude in a unit as written, with the unit's reduction when the service reports one. */
export interface QuantityValue {
  magnitude: Magnitude;
  unit: string;
  unitTerm?: UnitFactorization;
}

/**
 * A measurement unit held as a value by itself, with no magnitude: `SI::m`, or
 * `m / s` as an operation composed it. `unitTerm` is its reduction, which the
 * service always sends and requires; `unitId` is the FQN of the one declaration
 * it names (`SI::kilometre`), absent for a unit an operation composed.
 */
export interface MeasurementRefValue {
  unit: string;
  unitTerm: UnitFactorization;
  unitId?: string;
}

/**
 * A calc held as a value — a calc definition, or a calc usage with an input no
 * read could supply — named by the FQN of its declaration, which is its identity.
 * `selfId` is the object its feature names resolve against, for a calc usage
 * read off a part (`holder.scale`); absent for a function closing over no object.
 */
export interface FunctionValue {
  calcId: string;
  selfId?: bigint;
}

/**
 * A multidimensional array: `dimensions` gives the extent of each dimension and
 * `elements` the elements flattened row-major, the last dimension varying
 * fastest. A rank-0 array holds one element; an element may itself be an array.
 */
export interface ArrayValue {
  dimensions: bigint[];
  elements: SysMLValue[];
}

/**
 * A tensor quantity of any rank: `dimensions` gives the extent of each dimension
 * and `components` one quantity per component, flattened row-major as an
 * array's elements are. A tensor of rank one is not a `vectorQuantity`.
 */
export interface TensorQuantityValue {
  dimensions: bigint[];
  components: QuantityValue[];
}

/**
 * A value the service computed. `absent` is the case a service sent no value at
 * all for, which is distinct from `unset` — a feature that exists and has none.
 * A `vector` is one value of numeric components, never a sequence of numbers,
 * and a `vectorQuantity` carries one quantity per component, each with its own unit.
 * A `set` is a unique, unordered collection — a `Collections::Set`'s elements —
 * distinct from a `sequence`, whose order is part of its value: the service
 * sends its elements in canonical order, so equal sets arrive alike, and reads
 * one sent in any order, refusing one that lists an element twice.
 */
export type SysMLValue =
  | { kind: "int"; value: bigint }
  | { kind: "real"; value: number }
  | { kind: "complex"; value: ComplexValue }
  | { kind: "boolean"; value: boolean }
  | { kind: "string"; value: string }
  | { kind: "instance"; id: bigint }
  | { kind: "sequence"; elements: SysMLValue[] }
  | ({ kind: "quantity" } & QuantityValue)
  | ({ kind: "measurementRef" } & MeasurementRefValue)
  | ({ kind: "function" } & FunctionValue)
  | { kind: "enum"; value: EnumValue }
  | ({ kind: "array" } & ArrayValue)
  | { kind: "vector"; components: Magnitude[] }
  | { kind: "vectorQuantity"; components: QuantityValue[] }
  | { kind: "set"; elements: SysMLValue[] }
  | ({ kind: "tensorQuantity" } & TensorQuantityValue)
  | { kind: "null"; reason: string }
  | { kind: "unset" }
  | { kind: "infinity" }
  | { kind: "absent" };

/** What a verification answered about. Kept for the verification RPCs of a later version. */
export interface VerdictSubject {
  /** "constraint", "requirement" or "satisfy". */
  kind: string;
  /** FQN of the verified element; empty for an anonymous satisfy assertion. */
  elementId: string;
  /** The element as a reader names it. */
  element: string;
  /** The instance verified against, when there was one. */
  instanceId?: bigint;
  instanceTypeId?: string;
}

/** One limit an engine ran under, and whether the run stopped at it. */
export interface VerdictBound {
  /** The bound's name as the budget spells it: "runs", "depth", "steps", "solver". */
  name: string;
  limit: bigint;
  /** True when the run stopped at the limit, which lowers the verdict's strength. */
  reached: boolean;
}

/**
 * What a verdict rests on: the engine that answered, the strength of its
 * evidence and the bounds it ran under. Empty from a service without the
 * `engines` capability, or for a verdict decided before any engine was asked.
 */
export interface VerdictStanding {
  /** The engine as `ListEngines` names it. */
  engine: string;
  /** "not covered", "observed", "witnessed", "bounded" or "proved". */
  strength: string;
  bounds: VerdictBound[];
}

/**
 * One verification's answer. `undecided` is the service reporting it could not
 * answer, which a `holds: false` alone does not distinguish. Every arm carries
 * the `standing` its evidence rests on.
 */
export type SysMLVerdict =
  | { kind: "holds"; subject: VerdictSubject; standing: VerdictStanding }
  | { kind: "fails"; subject: VerdictSubject; condition: string; standing: VerdictStanding }
  | {
      kind: "undecided";
      subject: VerdictSubject;
      error: string;
      cause: FailureCause;
      standing: VerdictStanding;
    };

/**
 * Decodes a `sysml.Value` into the union.
 *
 * @throws {MalformedValueError} for a value that contradicts itself: an array
 *   whose elements do not fill its dimensions, a vector with a component that
 *   is not a number, a vector quantity with no components, a set listing a
 *   member twice, a tensor quantity whose components do not fill its
 *   dimensions, a quantity (alone or as a component) with no magnitude, or a
 *   measurement reference naming no unit or a unit without its reduction.
 */
export function decodeValue(value: Value | undefined): SysMLValue {
  if (value === undefined) {
    return { kind: "absent" };
  }
  const kind = value.kind;
  switch (kind.case) {
    case "intValue":
      return { kind: "int", value: kind.value };
    case "realValue":
      return { kind: "real", value: kind.value };
    case "complex":
      return { kind: "complex", value: { real: kind.value.real, imaginary: kind.value.imaginary } };
    case "boolValue":
      return { kind: "boolean", value: kind.value };
    case "stringValue":
      return { kind: "string", value: kind.value };
    case "instanceId":
      return { kind: "instance", id: kind.value };
    case "sequence":
      return { kind: "sequence", elements: kind.value.elements.map(decodeValue) };
    case "quantity":
      return { kind: "quantity", ...decodeQuantity(kind.value) };
    case "measurementRef":
      return { kind: "measurementRef", ...decodeMeasurementRef(kind.value) };
    case "function":
      return { kind: "function", ...decodeFunction(kind.value) };
    case "enumLiteral":
      return { kind: "enum", value: decodeEnumLiteral(kind.value) };
    case "array":
      return { kind: "array", ...decodeArray(kind.value) };
    case "vector":
      return { kind: "vector", components: decodeVector(kind.value) };
    case "vectorQuantity":
      return { kind: "vectorQuantity", components: decodeVectorQuantity(kind.value) };
    case "set":
      return { kind: "set", elements: decodeSet(kind.value) };
    case "tensorQuantity":
      return { kind: "tensorQuantity", ...decodeTensorQuantity(kind.value) };
    case "null":
      return { kind: "null", reason: kind.value };
    case "unset":
      return { kind: "unset" };
    case "infinity":
      // Only an asserted arm carries the unbounded value.
      if (!kind.value) {
        throw new MalformedValueError("the infinity arm states no value unless it is true");
      }
      return { kind: "infinity" };
    case undefined:
      return { kind: "absent" };
  }
}

/**
 * Encodes the union as the `sysml.Value` the service decodes: the inverse of
 * {@link decodeValue} for every kind but `absent`, which is no value at all.
 *
 * @throws {MalformedValueError} for an `absent` value, or a structured value
 *   the service would refuse (see {@link decodeValue}).
 */
export function encodeValue(value: SysMLValue): Value {
  switch (value.kind) {
    case "int":
      return create(ValueSchema, { kind: { case: "intValue", value: value.value } });
    case "real":
      return create(ValueSchema, { kind: { case: "realValue", value: value.value } });
    case "complex":
      return create(ValueSchema, {
        kind: { case: "complex", value: create(ComplexSchema, value.value) },
      });
    case "boolean":
      return create(ValueSchema, { kind: { case: "boolValue", value: value.value } });
    case "string":
      return create(ValueSchema, { kind: { case: "stringValue", value: value.value } });
    case "instance":
      return create(ValueSchema, { kind: { case: "instanceId", value: value.id } });
    case "sequence":
      return create(ValueSchema, {
        kind: {
          case: "sequence",
          value: create(ValueSequenceSchema, { elements: value.elements.map(encodeValue) }),
        },
      });
    case "quantity":
      return create(ValueSchema, { kind: { case: "quantity", value: encodeQuantity(value) } });
    case "measurementRef":
      return create(ValueSchema, {
        kind: { case: "measurementRef", value: encodeMeasurementRef(value) },
      });
    case "function":
      return create(ValueSchema, { kind: { case: "function", value: encodeFunction(value) } });
    case "enum":
      return create(ValueSchema, {
        kind: { case: "enumLiteral", value: encodeEnumLiteral(value.value) },
      });
    case "array":
      checkShape("an array", value.dimensions, value.elements.length);
      return create(ValueSchema, {
        kind: {
          case: "array",
          value: create(ArraySchema, {
            dimensions: value.dimensions,
            elements: value.elements.map(encodeValue),
          }),
        },
      });
    case "vector":
      return create(ValueSchema, {
        kind: {
          case: "vector",
          value: create(VectorSchema, { components: value.components.map(encodeMagnitude) }),
        },
      });
    case "vectorQuantity":
      if (value.components.length === 0) {
        throw new MalformedValueError("a vector quantity has no components");
      }
      return create(ValueSchema, {
        kind: {
          case: "vectorQuantity",
          value: create(VectorQuantitySchema, { components: value.components.map(encodeQuantity) }),
        },
      });
    case "set":
      return create(ValueSchema, {
        kind: {
          case: "set",
          value: create(ValueSetSchema, {
            elements: uniqueMembers(value.elements).map(encodeValue),
          }),
        },
      });
    case "tensorQuantity":
      checkShape("a tensor quantity", value.dimensions, value.components.length);
      return create(ValueSchema, {
        kind: {
          case: "tensorQuantity",
          value: create(TensorQuantitySchema, {
            dimensions: value.dimensions,
            components: value.components.map(encodeQuantity),
          }),
        },
      });
    case "null":
      return create(ValueSchema, { kind: { case: "null", value: value.reason } });
    case "unset":
      return create(ValueSchema, { kind: { case: "unset", value: true } });
    case "infinity":
      return create(ValueSchema, { kind: { case: "infinity", value: true } });
    case "absent":
      throw new MalformedValueError("an absent value is no value at all, so it cannot be sent");
  }
}

/** Decodes a `sysml.Verdict` into the union. */
export function decodeVerdict(verdict: Verdict): SysMLVerdict {
  const subject: VerdictSubject = {
    kind: verdict.kind,
    elementId: verdict.elementId,
    element: verdict.element,
    ...(verdict.instanceId === 0n ? {} : { instanceId: verdict.instanceId }),
    ...(verdict.instanceTypeId === "" ? {} : { instanceTypeId: verdict.instanceTypeId }),
  };
  const standing = decodeStanding(verdict);
  if (verdict.error !== "") {
    return {
      kind: "undecided",
      subject,
      error: verdict.error,
      cause: failureCause(verdict.failureReason),
      standing,
    };
  }
  return verdict.holds
    ? { kind: "holds", subject, standing }
    : { kind: "fails", subject, condition: verdict.condition, standing };
}

/** Reads the standing fields the `engines` capability adds to a verdict. */
export function decodeStanding(verdict: {
  engine: string;
  strength: string;
  bounds: Bound[];
}): VerdictStanding {
  return {
    engine: verdict.engine,
    strength: verdict.strength,
    bounds: verdict.bounds.map((b) => ({ name: b.name, limit: b.limit, reached: b.reached })),
  };
}

/** Names the enum the service reports for a failure it could not answer through. */
export function failureCause(reason: FailureReason): FailureCause {
  switch (reason) {
    case FailureReason.EVALUATION:
      return "evaluation";
    case FailureReason.WRONG_KIND:
      return "wrong_kind";
    case FailureReason.AMBIGUOUS_SUBJECT:
      return "ambiguous_subject";
    case FailureReason.UNSPECIFIED:
      return "unspecified";
  }
}

/** Renders a value the way the REPL prints one, for logs and error messages. */
export function formatValue(value: SysMLValue): string {
  switch (value.kind) {
    case "int":
      return value.value.toString();
    case "real":
      return formatReal(value.value);
    case "complex":
      return formatComplex(value.value);
    case "boolean":
      return value.value ? "true" : "false";
    case "string":
      return JSON.stringify(value.value);
    case "instance":
      return `<instance ${value.id.toString()}>`;
    case "sequence":
      return `(${value.elements.map(formatValue).join(", ")})`;
    case "quantity": {
      const magnitude = formatMagnitude(value.magnitude);
      return value.unit === "" ? magnitude : `${magnitude}[${value.unit}]`;
    }
    case "measurementRef":
      return value.unit === "" ? formatUnitTerm(value.unitTerm) : value.unit;
    case "function":
      return value.calcId;
    case "enum":
      return value.value.name;
    case "array":
      return `Array(${value.dimensions.join(", ")})[${value.elements.map(formatValue).join(", ")}]`;
    case "vector":
      return `⟨${value.components.map(formatMagnitude).join(", ")}⟩`;
    case "vectorQuantity":
      return `⟨${value.components.map((c) => formatValue({ kind: "quantity", ...c })).join(", ")}⟩`;
    case "set":
      return `{${value.elements.map(formatValue).join(", ")}}`;
    case "tensorQuantity":
      return formatTensorQuantity(value);
    case "null":
      return value.reason === "" ? "null" : `null (${value.reason})`;
    case "unset":
      return "unset";
    case "infinity":
      return "*";
    case "absent":
      return "absent";
  }
}

function formatReal(value: number): string {
  return Number.isInteger(value) ? value.toFixed(1) : value.toString();
}

function formatMagnitude(magnitude: Magnitude): string {
  return magnitude.kind === "int" ? magnitude.value.toString() : formatReal(magnitude.value);
}

/** `Tensor(2, 2, 2)[1.0, …, 8.0][Pa]` when every component shares a unit; else each with its own. */
function formatTensorQuantity(tensor: TensorQuantityValue): string {
  const dims = tensor.dimensions.join(", ");
  const units = new Set(tensor.components.map((c) => c.unit));
  if (units.size === 1 && tensor.components[0]?.unit !== "") {
    const body = tensor.components.map((c) => formatMagnitude(c.magnitude)).join(", ");
    return `Tensor(${dims})[${body}][${tensor.components[0]?.unit ?? ""}]`;
  }
  const body = tensor.components.map((c) => formatValue({ kind: "quantity", ...c })).join(", ");
  return `Tensor(${dims})[${body}]`;
}

/** `1.5 - 2.0i`, as the REPL prints a Complex; the sign is the imaginary part's. */
function formatComplex(value: ComplexValue): string {
  const sign = value.imaginary < 0 || Object.is(value.imaginary, -0) ? "-" : "+";
  return `${formatReal(value.real)} ${sign} ${formatReal(Math.abs(value.imaginary))}i`;
}

function decodeQuantity(quantity: Quantity): QuantityValue {
  let magnitude: Magnitude;
  switch (quantity.magnitude.case) {
    case "intMagnitude":
      magnitude = { kind: "int", value: quantity.magnitude.value };
      break;
    case "realMagnitude":
      magnitude = { kind: "real", value: quantity.magnitude.value };
      break;
    default:
      throw new MalformedValueError(`a quantity in [${quantity.unit}] has no magnitude`);
  }
  const unitTerm = quantity.unitTerm === undefined ? undefined : decodeUnitTerm(quantity.unitTerm);
  return {
    magnitude,
    unit: quantity.unit,
    ...(unitTerm === undefined ? {} : { unitTerm }),
  };
}

function encodeQuantity(quantity: QuantityValue): Quantity {
  return create(QuantitySchema, {
    magnitude:
      quantity.magnitude.kind === "int"
        ? { case: "intMagnitude", value: quantity.magnitude.value }
        : { case: "realMagnitude", value: quantity.magnitude.value },
    unit: quantity.unit,
    ...(quantity.unitTerm === undefined ? {} : { unitTerm: encodeUnitTerm(quantity.unitTerm) }),
  });
}

function decodeMeasurementRef(ref: MeasurementRef): MeasurementRefValue {
  if (ref.unit === "" && ref.unitId === "" && ref.unitTerm === undefined) {
    throw new MalformedValueError("a measurement reference names no unit");
  }
  if (ref.unitTerm === undefined) {
    throw new MalformedValueError(
      `a measurement reference ${ref.unit || ref.unitId} has no reduction to base units`,
    );
  }
  return {
    unit: ref.unit,
    unitTerm: decodeUnitTerm(ref.unitTerm),
    ...(ref.unitId === "" ? {} : { unitId: ref.unitId }),
  };
}

function encodeMeasurementRef(ref: MeasurementRefValue): MeasurementRef {
  return create(MeasurementRefSchema, {
    unit: ref.unit,
    unitTerm: encodeUnitTerm(ref.unitTerm),
    unitId: ref.unitId ?? "",
  });
}

function decodeFunction(fn: FunctionMessage): FunctionValue {
  if (fn.calcId === "") {
    throw new MalformedValueError("a function names no calc");
  }
  return { calcId: fn.calcId, ...(fn.selfId === 0n ? {} : { selfId: fn.selfId }) };
}

function encodeFunction(fn: FunctionValue): FunctionMessage {
  if (fn.calcId === "") {
    throw new MalformedValueError("a function names no calc");
  }
  return create(FunctionSchema, { calcId: fn.calcId, selfId: fn.selfId ?? 0n });
}

function encodeUnitTerm(term: UnitFactorization): UnitTerm {
  return create(UnitTermSchema, {
    scaleNum: term.scaleNum,
    scaleDen: term.scaleDen,
    factors: term.factors.map((factor) => create(UnitFactorSchema, factor)),
  });
}

/** `1000·SI::metre·SI::second^-1`, for a unit that was never written down. */
function formatUnitTerm(term: UnitFactorization): string {
  const parts: string[] = [];
  if (term.scaleNum !== 1 || term.scaleDen !== 1) {
    parts.push(term.scaleDen === 1 ? `${term.scaleNum}` : `${term.scaleNum}/${term.scaleDen}`);
  }
  for (const factor of term.factors) {
    parts.push(factor.exponent === 1 ? factor.unitId : `${factor.unitId}^${factor.exponent}`);
  }
  return parts.length === 0 ? "1" : parts.join("·");
}

function encodeMagnitude(magnitude: Magnitude): Value {
  return magnitude.kind === "int"
    ? create(ValueSchema, { kind: { case: "intValue", value: magnitude.value } })
    : create(ValueSchema, { kind: { case: "realValue", value: magnitude.value } });
}

/** The flattened size the dimensions demand, refusing a dimension that is not positive. */
function checkShape(what: string, dimensions: bigint[], elementCount: number): void {
  let size = 1n;
  for (const extent of dimensions) {
    if (extent <= 0n) {
      throw new MalformedValueError(`${what} dimension is ${extent.toString()}, not positive`);
    }
    size *= extent;
  }
  if (size !== BigInt(elementCount)) {
    throw new MalformedValueError(
      `${what} of dimensions (${dimensions.join(", ")}) holds ${elementCount} element(s), want ${size.toString()}`,
    );
  }
}

function decodeArray(array: ArrayMessage): ArrayValue {
  checkShape("an array", array.dimensions, array.elements.length);
  return { dimensions: [...array.dimensions], elements: array.elements.map(decodeValue) };
}

function decodeSet(set: ValueSet): SysMLValue[] {
  return uniqueMembers(set.elements.map(decodeValue));
}

/** Returns the members as given, refusing one listed twice by {@link valuesEqual}. */
function uniqueMembers(members: SysMLValue[]): SysMLValue[] {
  members.forEach((member, i) => {
    if (members.slice(0, i).some((held) => valuesEqual(held, member))) {
      throw new MalformedValueError(
        `a set lists a member twice: ${formatValue(member)}`,
      );
    }
  });
  return members;
}

/**
 * Whether two values are the same value to the model, as the service judges a
 * set's membership: numbers by value, so a whole `real` is the `int` of its
 * value and a `complex` on the real axis is its real part, exactly across the
 * whole `int` range; a `sequence`'s order counts and a `set`'s does not, nor
 * does a member it lists twice; a quantity is the same over its base units; a
 * `measurementRef` is one reduction at one scale however spelt, except that a
 * named unit of dimension one is only its own declaration (`rad` is not `sr`);
 * an `enum` is its `literalId`, whatever else describes it; a `function` is
 * its `calcId` read against its `selfId`; a `null` is the same whatever its
 * reason, and the same as an empty `sequence` or `set` — the model's absent
 * value however spelt.
 */
export function valuesEqual(a: SysMLValue, b: SysMLValue): boolean {
  if (isEmpty(a) || isEmpty(b)) {
    return isEmpty(a) && isEmpty(b);
  }
  switch (a.kind) {
    case "int":
    case "real":
    case "complex":
      return numbersEqual(a, b);
    case "boolean":
      return b.kind === "boolean" && a.value === b.value;
    case "string":
      return b.kind === "string" && a.value === b.value;
    case "instance":
      return b.kind === "instance" && a.id === b.id;
    case "sequence":
      return b.kind === "sequence" && elementsEqual(a.elements, b.elements);
    case "quantity":
      return b.kind === "quantity" && quantitiesEqual(a, b);
    case "measurementRef":
      return b.kind === "measurementRef" && measurementRefsEqual(a, b);
    case "enum":
      return b.kind === "enum" && a.value.literalId === b.value.literalId;
    case "function":
      return (
        b.kind === "function" &&
        a.calcId === b.calcId &&
        a.selfId === b.selfId
      );
    case "array":
      return (
        b.kind === "array" &&
        dimensionsEqual(a.dimensions, b.dimensions) &&
        elementsEqual(a.elements, b.elements)
      );
    case "vector":
      return b.kind === "vector" && magnitudesEqual(a.components, b.components);
    case "vectorQuantity":
      return (
        b.kind === "vectorQuantity" &&
        componentsEqual(a.components, b.components)
      );
    case "set":
      return (
        b.kind === "set" &&
        subsetOf(a.elements, b.elements) &&
        subsetOf(b.elements, a.elements)
      );
    case "tensorQuantity":
      return (
        b.kind === "tensorQuantity" &&
        dimensionsEqual(a.dimensions, b.dimensions) &&
        componentsEqual(a.components, b.components)
      );
    case "infinity":
    case "null":
    case "unset":
    case "absent":
      return b.kind === a.kind;
  }
}

/** The model's absent value: a `null`, or a collection with no members. */
function isEmpty(value: SysMLValue): boolean {
  switch (value.kind) {
    case "null":
      return true;
    case "sequence":
    case "set":
      return value.elements.length === 0;
    default:
      return false;
  }
}

function elementsEqual(a: SysMLValue[], b: SysMLValue[]): boolean {
  return (
    a.length === b.length &&
    a.every((e, i) => valuesEqual(e, b[i]))
  );
}

function subsetOf(members: SysMLValue[], of: SysMLValue[]): boolean {
  return members.every((e) => of.some((o) => valuesEqual(e, o)));
}

function dimensionsEqual(a: bigint[], b: bigint[]): boolean {
  return a.length === b.length && a.every((d, i) => d === b[i]);
}

type NumberValue = Magnitude | { kind: "complex"; value: ComplexValue };

function numbersEqual(a: NumberValue, b: SysMLValue): boolean {
  if (a.kind === "complex") {
    if (a.value.imaginary !== 0) {
      return (
        b.kind === "complex" &&
        a.value.real === b.value.real &&
        a.value.imaginary === b.value.imaginary
      );
    }
    a = { kind: "real", value: a.value.real };
  }
  if (b.kind === "complex") {
    if (b.value.imaginary !== 0) {
      return false;
    }
    b = { kind: "real", value: b.value.real };
  }
  if (a.kind === "int") {
    if (b.kind === "int") return a.value === b.value;
    return b.kind === "real" && realIsInt(b.value, a.value);
  }
  if (b.kind === "int") return realIsInt(a.value, b.value);
  return b.kind === "real" && a.value === b.value;
}

// Whether r is exactly the integer n, never rounding n.
function realIsInt(r: number, n: bigint): boolean {
  return Number.isInteger(r) && r >= -(2 ** 63) && r < 2 ** 63 && BigInt(r) === n;
}

function magnitudesEqual(a: Magnitude[], b: Magnitude[]): boolean {
  return a.length === b.length && a.every((m, i) => numbersEqual(m, b[i]));
}

function componentsEqual(a: QuantityValue[], b: QuantityValue[]): boolean {
  return (
    a.length === b.length &&
    a.every((q, i) => quantitiesEqual(q, b[i]))
  );
}

// Commensurable quantities are equal over their base units, as the service
// judges them; one carrying no reduction is compared in its unit as written.
function quantitiesEqual(a: QuantityValue, b: QuantityValue): boolean {
  if (a.unitTerm === undefined || b.unitTerm === undefined) {
    return (
      numbersEqual(a.magnitude, b.magnitude) &&
      a.unit === b.unit &&
      unitTermsEqual(a.unitTerm, b.unitTerm)
    );
  }
  if (
    !commensurable(a.unitTerm, b.unitTerm) ||
    zeroScale(a.unitTerm) ||
    zeroScale(b.unitTerm)
  ) {
    return false;
  }
  if (
    a.magnitude.kind === "int" &&
    b.magnitude.kind === "int" &&
    wholeScale(a.unitTerm) &&
    wholeScale(b.unitTerm)
  ) {
    // m₁·n₁/d₁ = m₂·n₂/d₂ exactly, cross-multiplied in bigint.
    return (
      a.magnitude.value * BigInt(a.unitTerm.scaleNum) * BigInt(b.unitTerm.scaleDen) ===
      b.magnitude.value * BigInt(b.unitTerm.scaleNum) * BigInt(a.unitTerm.scaleDen)
    );
  }
  return baseMagnitude(a.magnitude, a.unitTerm) === baseMagnitude(b.magnitude, b.unitTerm);
}

function baseMagnitude(magnitude: Magnitude, term: UnitFactorization): number {
  return (Number(magnitude.value) * term.scaleNum) / term.scaleDen;
}

// The reduction as base unit → exponent, repeated base units summed and
// cancelled ones dropped, so two reductions compare however they are listed.
function exponents(term: UnitFactorization): Map<string, number> {
  const totals = new Map<string, number>();
  for (const factor of term.factors) {
    totals.set(factor.unitId, (totals.get(factor.unitId) ?? 0) + factor.exponent);
  }
  for (const [unitId, exponent] of totals) {
    if (exponent === 0) {
      totals.delete(unitId);
    }
  }
  return totals;
}

function commensurable(a: UnitFactorization, b: UnitFactorization): boolean {
  const x = exponents(a);
  const y = exponents(b);
  return x.size === y.size && [...x].every(([unitId, exponent]) => y.get(unitId) === exponent);
}

function zeroScale(term: UnitFactorization): boolean {
  return term.scaleNum === 0 || term.scaleDen === 0;
}

/** One reduction: commensurable at one scale, however the ratio is written. */
function sameReduction(a: UnitFactorization, b: UnitFactorization): boolean {
  return (
    commensurable(a, b) &&
    !zeroScale(a) &&
    !zeroScale(b) &&
    a.scaleNum * b.scaleDen === b.scaleNum * a.scaleDen
  );
}

// One reduction at one scale (`SI::'m/s'` is `m/s`, `km/m` is `m/mm`); a named
// unit reducing to nothing is only the declaration it names.
function measurementRefsEqual(a: MeasurementRefValue, b: MeasurementRefValue): boolean {
  if (!sameReduction(a.unitTerm, b.unitTerm)) {
    return false;
  }
  if (exponents(a.unitTerm).size === 0 && (a.unitId !== undefined || b.unitId !== undefined)) {
    return a.unitId === b.unitId;
  }
  return true;
}

function wholeScale(term: UnitFactorization): boolean {
  return Number.isInteger(term.scaleNum) && Number.isInteger(term.scaleDen);
}

function unitTermsEqual(
  a: UnitFactorization | undefined,
  b: UnitFactorization | undefined,
): boolean {
  if (a === undefined || b === undefined) {
    return a === b;
  }
  return (
    a.scaleNum === b.scaleNum &&
    a.scaleDen === b.scaleDen &&
    a.factors.length === b.factors.length &&
    a.factors.every(
      (f, i) =>
        f.unitId === b.factors[i]?.unitId &&
        f.exponent === b.factors[i]?.exponent,
    )
  );
}

function decodeTensorQuantity(tensor: TensorQuantity): TensorQuantityValue {
  checkShape("a tensor quantity", tensor.dimensions, tensor.components.length);
  return { dimensions: [...tensor.dimensions], components: tensor.components.map(decodeQuantity) };
}

function decodeVector(vector: Vector): Magnitude[] {
  return vector.components.map((component) => {
    switch (component.kind.case) {
      case "intValue":
        return { kind: "int", value: component.kind.value };
      case "realValue":
        return { kind: "real", value: component.kind.value };
      default:
        throw new MalformedValueError(
          `a vector component is ${component.kind.case ?? "empty"}, not a number`,
        );
    }
  });
}

function decodeVectorQuantity(vector: VectorQuantity): QuantityValue[] {
  if (vector.components.length === 0) {
    throw new MalformedValueError("a vector quantity has no components");
  }
  return vector.components.map(decodeQuantity);
}

function decodeUnitTerm(term: UnitTerm): UnitFactorization {
  return {
    scaleNum: term.scaleNum,
    scaleDen: term.scaleDen,
    factors: term.factors.map((factor) => ({
      unitId: factor.unitId,
      exponent: factor.exponent,
    })),
  };
}

function decodeEnumLiteral(literal: EnumLiteral): EnumValue {
  const decoded: EnumValue = {
    name: literal.name,
    literalId: literal.literalId,
    enumerationId: literal.enumerationId,
  };
  if (literal.value !== undefined) {
    decoded.value = decodeValue(literal.value);
  }
  return decoded;
}

function encodeEnumLiteral(literal: EnumValue): EnumLiteral {
  return create(EnumLiteralSchema, {
    name: literal.name,
    literalId: literal.literalId,
    enumerationId: literal.enumerationId,
    value: literal.value === undefined ? undefined : encodeValue(literal.value),
  });
}
