// Values, quantities and verdicts as discriminated unions: a caller switches on
// `kind` and the compiler checks the switch is exhaustive.

import { create } from "@bufbuild/protobuf";
import type {
  Array as ArrayMessage,
  EnumLiteral,
  MeasurementRef,
  Quantity,
  UnitTerm,
  Value,
  Vector,
  VectorQuantity,
  Verdict,
} from "../generated/sysml_pb.js";
import {
  ArraySchema,
  ComplexSchema,
  EnumLiteralSchema,
  FailureReason,
  MeasurementRefSchema,
  QuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
  ValueSchema,
  ValueSequenceSchema,
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
 * A multidimensional array: `dimensions` gives the extent of each dimension and
 * `elements` the elements flattened row-major, the last dimension varying
 * fastest. A rank-0 array holds one element; an element may itself be an array.
 */
export interface ArrayValue {
  dimensions: bigint[];
  elements: SysMLValue[];
}

/**
 * A value the service computed. `absent` is the case a service sent no value at
 * all for, which is distinct from `unset` — a feature that exists and has none.
 * A `vector` is one value of numeric components, never a sequence of numbers,
 * and a `vectorQuantity` carries one quantity per component, each with its own unit.
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
  | { kind: "enum"; value: EnumValue }
  | ({ kind: "array" } & ArrayValue)
  | { kind: "vector"; components: Magnitude[] }
  | { kind: "vectorQuantity"; components: QuantityValue[] }
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

/**
 * One verification's answer. `undecided` is the service reporting it could not
 * answer, which a `holds: false` alone does not distinguish.
 */
export type SysMLVerdict =
  | { kind: "holds"; subject: VerdictSubject }
  | { kind: "fails"; subject: VerdictSubject; condition: string }
  | { kind: "undecided"; subject: VerdictSubject; error: string; cause: FailureCause };

/**
 * Decodes a `sysml.Value` into the union.
 *
 * @throws {MalformedValueError} for a value that contradicts itself: an array
 *   whose elements do not fill its dimensions, a vector with a component that
 *   is not a number, a vector quantity with no components, a quantity
 *   (alone or as a component) with no magnitude, or a measurement reference
 *   naming no unit or a unit without its reduction.
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
    case "enumLiteral":
      return { kind: "enum", value: decodeEnumLiteral(kind.value) };
    case "array":
      return { kind: "array", ...decodeArray(kind.value) };
    case "vector":
      return { kind: "vector", components: decodeVector(kind.value) };
    case "vectorQuantity":
      return { kind: "vectorQuantity", components: decodeVectorQuantity(kind.value) };
    case "null":
      return { kind: "null", reason: kind.value };
    case "unset":
      return { kind: "unset" };
    case "infinity":
      // Only an asserted arm carries the unbounded value.
      if (kind.value !== true) {
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
    case "enum":
      return create(ValueSchema, {
        kind: { case: "enumLiteral", value: create(EnumLiteralSchema, value.value) },
      });
    case "array":
      checkArrayShape(value.dimensions, value.elements.length);
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
  if (verdict.error !== "") {
    return {
      kind: "undecided",
      subject,
      error: verdict.error,
      cause: failureCause(verdict.failureReason),
    };
  }
  return verdict.holds
    ? { kind: "holds", subject }
    : { kind: "fails", subject, condition: verdict.condition };
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
    case "enum":
      return value.value.name;
    case "array":
      return `Array(${value.dimensions.join(", ")})[${value.elements.map(formatValue).join(", ")}]`;
    case "vector":
      return `⟨${value.components.map(formatMagnitude).join(", ")}⟩`;
    case "vectorQuantity":
      return `⟨${value.components.map((c) => formatValue({ kind: "quantity", ...c })).join(", ")}⟩`;
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
function checkArrayShape(dimensions: bigint[], elementCount: number): void {
  let size = 1n;
  for (const extent of dimensions) {
    if (extent <= 0n) {
      throw new MalformedValueError(`an array dimension is ${extent.toString()}, not positive`);
    }
    size *= extent;
  }
  if (size !== BigInt(elementCount)) {
    throw new MalformedValueError(
      `an array of dimensions (${dimensions.join(", ")}) holds ${elementCount} element(s), want ${size.toString()}`,
    );
  }
}

function decodeArray(array: ArrayMessage): ArrayValue {
  checkArrayShape(array.dimensions, array.elements.length);
  return { dimensions: [...array.dimensions], elements: array.elements.map(decodeValue) };
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
  return {
    name: literal.name,
    literalId: literal.literalId,
    enumerationId: literal.enumerationId,
  };
}
