// Values, quantities and verdicts as discriminated unions: a caller switches on
// `kind` and the compiler checks the switch is exhaustive.

import { create } from "@bufbuild/protobuf";
import type {
  Array as ArrayMessage,
  Bound,
  EnumLiteral,
  Function as FunctionMessage,
  MeasurementRef,
  Metaobject as MetaobjectMessage,
  Quantity,
  Rational,
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
  MetaobjectSchema,
  QuantitySchema,
  RationalSchema,
  TensorQuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
  ValueSchema,
  ValueSequenceSchema,
  ValueSetSchema,
  VectorQuantitySchema,
  VectorSchema,
} from "../generated/sysml_pb.js";
import {
  CAPABILITY_BIG_INT_VALUES,
  CAPABILITY_COMPLEX_VALUES,
  CAPABILITY_FUNCTION_VALUES,
  CAPABILITY_INFINITY_VALUE,
  CAPABILITY_MEASUREMENT_REFS,
  CAPABILITY_METAOBJECT_VALUES,
  CAPABILITY_RATIONAL_VALUES,
  CAPABILITY_SET_VALUES,
  CAPABILITY_STRUCTURED_VALUES,
  CAPABILITY_TENSOR_VALUES,
  requireCapability,
  upgradeRemedy,
  type ServerInfo,
} from "./capabilities.js";
import { MalformedValueError, UnsupportedValueError, type FailureCause } from "./errors.js";

/** An exact Rational in lowest terms over a positive denominator. */
export interface RationalValue {
  numerator: bigint;
  denominator: bigint;
}

/** A quantity's magnitude: an Integer, an exact Rational or a Real, kept apart. */
export type Magnitude =
  | { kind: "int"; value: bigint }
  | ({ kind: "rational" } & RationalValue)
  | { kind: "real"; value: number };

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
 * An element of the model held as an instance of its reflective metaclass: what
 * `x meta KerML::Feature`, or the last element of `x.metadata`, evaluates to.
 * `elementId` is the FQN of the element reflected on, which is its identity;
 * `metaclassId` is the FQN of the element's own metaclass
 * (`SysML::Systems::PartUsage`), not the type it was cast to. The service always
 * sends it; one sent to the service may leave it empty to have the model's used,
 * but one naming a metaclass that is not the element's is refused. The
 * metaobject's features (`declaredName`, `ownedFeature`, ...) are read in the
 * model, not carried.
 */
export interface MetaobjectValue {
  elementId: string;
  metaclassId: string;
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
 * A model-level result the model leaves open (an unbound feature, an unfixed count):
 * `reason` says why, `countLower`/`countUpper` bound its count as `MultiplicityInfo` spells them.
 */
export interface UndeterminedValue {
  reason: string;
  countLower: string;
  countUpper: string;
}

/**
 * A value the service computed. `absent` is the case a service sent no value at
 * all for, which is distinct from `unset` — a feature that exists and has none —
 * and from `undetermined`, a model-level answer the model leaves open.
 * A `vector` is one value of numeric components, never a sequence of numbers,
 * and a `vectorQuantity` carries one quantity per component, each with its own unit.
 * A `set` is a unique, unordered collection — a `Collections::Set`'s elements —
 * distinct from a `sequence`, whose order is part of its value: the service
 * sends its elements in canonical order, so equal sets arrive alike, and reads
 * one sent in any order, refusing one that lists an element twice.
 */
export type SysMLValue =
  | { kind: "int"; value: bigint }
  | ({ kind: "rational" } & RationalValue)
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
  | ({ kind: "metaobject" } & MetaobjectValue)
  | { kind: "null"; reason: string }
  | { kind: "unset" }
  | ({ kind: "undetermined" } & UndeterminedValue)
  | { kind: "infinity" }
  | { kind: "absent" };

/** What a verification answered about. */
export interface VerdictSubject {
  /** "constraint", "requirement", "satisfy", "objective", "assertion" or "object". */
  kind: string;
  /** FQN of the verified element; empty for an anonymous satisfy assertion. */
  elementId: string;
  /** The element as a reader names it. */
  element: string;
  /** The instance verified against, when there was one. */
  instanceId?: bigint;
  instanceTypeId?: string;
  /** FQN of the requirement a "satisfy" verdict asserts satisfied; empty for every other kind. */
  requirementId?: string;
  /** Where the object this verdict is about sits in a validated one (`engine.injector`, `wheels[2]`). */
  instancePath?: string;
}

/** One feature's value in the assignment witnessing a verdict. */
export interface WitnessAssignment {
  /** The qualified feature name, chain steps appended with '.'. */
  feature: string;
  /** The value the evaluator replayed for the feature. */
  value: SysMLValue;
  /** The base units the magnitude is expressed in; empty for a value that has none. */
  unit: string;
  /** The solver's exact value as text. */
  exact: string;
}

/** What the body of a verification case answered when it ran. */
export interface VerificationVerdict {
  /** FQN of the verification case that ran. */
  caseId: string;
  /** The VerdictKind the body produced: "pass", "fail", "inconclusive" or "error". */
  kind: string;
  /** Why an inconclusive body decided nothing, or the error that stopped the run. */
  detail: string;
  /** Whether this is the verdict of a case performed by another. */
  subcase: boolean;
  /** FQN of the requirement this verdict was reported for; empty for a case run for itself. */
  requirementId: string;
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
  /** Whether the service reported a standing at all. */
  reported: boolean;
  /** The bounds the engine stopped at. */
  reached: VerdictBound[];
}

/**
 * One verification's answer. `undecided` is the service reporting it could not
 * answer, which a `holds: false` alone does not distinguish. Every arm carries
 * the `standing` its evidence rests on, the `question` asked and the `status`
 * the service answered with, the `witness` assignment when the answer carries
 * one, and the `verifications` the requirement's verification cases produced.
 */
export type SysMLVerdict =
  | {
      kind: "holds";
      subject: VerdictSubject;
      standing: VerdictStanding;
      question: string;
      status: string;
      witness: WitnessAssignment[];
      verifications: VerificationVerdict[];
    }
  | {
      kind: "fails";
      subject: VerdictSubject;
      condition: string;
      standing: VerdictStanding;
      question: string;
      status: string;
      witness: WitnessAssignment[];
      verifications: VerificationVerdict[];
    }
  | {
      kind: "undecided";
      subject: VerdictSubject;
      error: string;
      cause: FailureCause;
      standing: VerdictStanding;
      question: string;
      status: string;
      witness: WitnessAssignment[];
      verifications: VerificationVerdict[];
    };

/**
 * Decodes a `sysml.Value` into the union.
 *
 * @throws {MalformedValueError} for a value that contradicts itself: an array
 *   whose elements do not fill its dimensions, a vector with a component that
 *   is not a number, a vector quantity with no components, a set listing a
 *   member twice, a tensor quantity whose components do not fill its
 *   dimensions, a quantity (alone or as a component) with no magnitude, a
 *   measurement reference naming no unit or a unit without its reduction, or a
 *   metaobject naming no element.
 */
export function decodeValue(value: Value | undefined): SysMLValue {
  if (value === undefined) {
    return { kind: "absent" };
  }
  const kind = value.kind;
  switch (kind.case) {
    case "intValue":
      return { kind: "int", value: kind.value };
    case "bigIntValue":
      return { kind: "int", value: decodeBigInteger(kind.value) };
    case "rationalValue":
      return { kind: "rational", ...decodeRational(kind.value) };
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
    case "metaobject":
      return { kind: "metaobject", ...decodeMetaobject(kind.value) };
    case "null":
      return { kind: "null", reason: kind.value };
    case "unset":
      return { kind: "unset" };
    case "undetermined":
      return {
        kind: "undetermined",
        reason: kind.value.reason,
        countLower: kind.value.count?.lower ?? "",
        countUpper: kind.value.count?.upper ?? "",
      };
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
    case "rational":
      return encodeMagnitude(value);
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
    case "metaobject":
      return create(ValueSchema, { kind: { case: "metaobject", value: encodeMetaobject(value) } });
    case "null":
      return create(ValueSchema, { kind: { case: "null", value: value.reason } });
    case "unset":
      return create(ValueSchema, { kind: { case: "unset", value: true } });
    case "undetermined":
      throw new MalformedValueError("an undetermined result is something to read, not to send");
    case "infinity":
      return create(ValueSchema, { kind: { case: "infinity", value: true } });
    case "absent":
      throw new MalformedValueError("an absent value is no value at all, so it cannot be sent");
  }
}

/** Decodes a `sysml.Verdict` into the union. */
export function decodeVerdict(
  verdict: Verdict,
  verifications: readonly VerificationVerdict[] = [],
): SysMLVerdict {
  const subject: VerdictSubject = {
    kind: verdict.kind,
    elementId: verdict.elementId,
    element: verdict.element,
    ...(verdict.instanceId === 0n ? {} : { instanceId: verdict.instanceId }),
    ...(verdict.instanceTypeId === "" ? {} : { instanceTypeId: verdict.instanceTypeId }),
    ...(verdict.requirementId === "" ? {} : { requirementId: verdict.requirementId }),
    ...(verdict.instancePath === "" ? {} : { instancePath: verdict.instancePath }),
  };
  const standing = decodeStanding(verdict);
  const witness = verdict.witness.map((assignment) => ({
    feature: assignment.feature,
    value: decodeValue(assignment.value),
    unit: assignment.unit,
    exact: assignment.exact,
  }));
  if (verdict.error !== "") {
    return {
      kind: "undecided",
      subject,
      error: verdict.error,
      cause: failureCause(verdict.failureReason),
      standing,
      question: verdict.question,
      status: verdict.status,
      witness,
      verifications: [...verifications],
    };
  }
  return verdict.holds
    ? {
        kind: "holds",
        subject,
        standing,
        question: verdict.question,
        status: verdict.status,
        witness,
        verifications: [...verifications],
      }
    : {
        kind: "fails",
        subject,
        condition: verdict.condition,
        standing,
        question: verdict.question,
        status: verdict.status,
        witness,
        verifications: [...verifications],
      };
}

/** Reads the standing fields the `engines` capability adds to a verdict. */
export function decodeStanding(verdict: {
  engine: string;
  strength: string;
  bounds: Bound[];
}): VerdictStanding {
  const bounds = verdict.bounds.map((b) => ({ name: b.name, limit: b.limit, reached: b.reached }));
  return {
    engine: verdict.engine,
    strength: verdict.strength,
    bounds,
    reported: verdict.strength !== "",
    reached: bounds.filter((bound) => bound.reached),
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
    case FailureReason.UNDECIDED:
      return "undecided";
  }
}

/** Renders a value the way the REPL prints one, for logs and error messages. */
export function formatValue(value: SysMLValue): string {
  switch (value.kind) {
    case "int":
      return value.value.toString();
    case "rational":
      return formatRational(value);
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
    case "metaobject":
      return `meta(${value.elementId} : ${value.metaclassId})`;
    case "null":
      return value.reason === "" ? "null" : `null (${value.reason})`;
    case "unset":
      return "unset";
    case "undetermined":
      return "<undetermined>";
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
  switch (magnitude.kind) {
    case "int":
      return magnitude.value.toString();
    case "rational":
      return formatRational(magnitude);
    case "real":
      return formatReal(magnitude.value);
  }
}

/** A terminating Rational as its decimal, `0.1`; any other as `numerator/denominator`, `1/3`. */
export function formatRational(value: RationalValue): string {
  let twos = 0;
  let fives = 0;
  let rest = value.denominator;
  while (rest % 2n === 0n) {
    rest /= 2n;
    twos++;
  }
  while (rest % 5n === 0n) {
    rest /= 5n;
    fives++;
  }
  if (rest !== 1n) {
    return `${value.numerator.toString()}/${value.denominator.toString()}`;
  }
  const places = Math.max(twos, fives, 1);
  const scaled = (value.numerator * 10n ** BigInt(places)) / value.denominator;
  const negative = scaled < 0n;
  const digits = (negative ? -scaled : scaled).toString().padStart(places + 1, "0");
  const point = digits.length - places;
  return `${negative ? "-" : ""}${digits.slice(0, point)}.${digits.slice(point)}`;
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
    case "bigIntMagnitude":
      magnitude = { kind: "int", value: decodeBigInteger(quantity.magnitude.value) };
      break;
    case "realMagnitude":
      magnitude = { kind: "real", value: quantity.magnitude.value };
      break;
    case "rationalMagnitude":
      magnitude = { kind: "rational", ...decodeRational(quantity.magnitude.value) };
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
    magnitude: encodeQuantityMagnitude(quantity.magnitude),
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

function decodeMetaobject(meta: MetaobjectMessage): MetaobjectValue {
  if (meta.elementId === "") {
    throw new MalformedValueError("a metaobject names no element");
  }
  return { elementId: meta.elementId, metaclassId: meta.metaclassId };
}

function encodeMetaobject(meta: MetaobjectValue): MetaobjectMessage {
  if (meta.elementId === "") {
    throw new MalformedValueError("a metaobject names no element");
  }
  return create(MetaobjectSchema, { elementId: meta.elementId, metaclassId: meta.metaclassId });
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

/** A quantity magnitude as the wire's `Quantity.magnitude` oneof writes it. */
export function encodeQuantityMagnitude(magnitude: Magnitude): Quantity["magnitude"] {
  switch (magnitude.kind) {
    case "real":
      return { case: "realMagnitude", value: magnitude.value };
    case "rational": {
      return { case: "rationalMagnitude", value: encodeRational(magnitude) };
    }
    case "int":
      return fitsInt64(magnitude.value)
        ? { case: "intMagnitude", value: magnitude.value }
        : { case: "bigIntMagnitude", value: magnitude.value.toString() };
  }
}

function encodeMagnitude(magnitude: Magnitude): Value {
  if (magnitude.kind === "real") {
    return create(ValueSchema, { kind: { case: "realValue", value: magnitude.value } });
  }
  if (magnitude.kind === "rational") {
    return create(ValueSchema, { kind: { case: "rationalValue", value: encodeRational(magnitude) } });
  }
  return fitsInt64(magnitude.value)
    ? create(ValueSchema, { kind: { case: "intValue", value: magnitude.value } })
    : create(ValueSchema, { kind: { case: "bigIntValue", value: magnitude.value.toString() } });
}

const INT64_MIN = -(2n ** 63n);
const INT64_MAX = 2n ** 63n - 1n;

/** Whether an Integer travels as `int_value`; one beyond int64 travels as `big_int_value`. */
export function fitsInt64(value: bigint): boolean {
  return value >= INT64_MIN && value <= INT64_MAX;
}

/** Reads the decimal of a `big_int_value` or `big_int_magnitude`. */
export function decodeBigInteger(text: string): bigint {
  if (!/^-?[0-9]+$/.test(text)) {
    throw new MalformedValueError(`a big Integer ${JSON.stringify(text)} is not decimal`);
  }
  return BigInt(text);
}

/**
 * Rewrites in place each Rational arm of a wire value a double holds exactly as
 * that double: the form a service without `rational_values` reads. A Rational
 * no double holds is left as it is; a collection's elements are values of their own.
 */
export function rationalsAsReals(value: Value): void {
  switch (value.kind.case) {
    case "rationalValue": {
      const double = wireRationalAsDouble(value.kind.value);
      if (double !== undefined) {
        value.kind = { case: "realValue", value: double };
      }
      return;
    }
    case "quantity":
      quantityRationalAsReal(value.kind.value);
      return;
    case "vector":
      value.kind.value.components.forEach(rationalsAsReals);
      return;
    case "vectorQuantity":
      value.kind.value.components.forEach(quantityRationalAsReal);
      return;
    case "tensorQuantity":
      value.kind.value.components.forEach(quantityRationalAsReal);
      return;
    default:
      return;
  }
}

/** Rewrites a quantity's Rational magnitude a double holds exactly as `real_magnitude`. */
export function quantityRationalAsReal(quantity: Quantity): void {
  if (quantity.magnitude.case === "rationalMagnitude") {
    const double = wireRationalAsDouble(quantity.magnitude.value);
    if (double !== undefined) {
      quantity.magnitude = { case: "realMagnitude", value: double };
    }
  }
}

function wireRationalAsDouble(wire: Rational): number | undefined {
  return rationalAsDouble({ numerator: BigInt(wire.numerator), denominator: BigInt(wire.denominator) });
}

/** The `Rational` message of an exact Rational, numerator and denominator in full. */
export function encodeRational(value: RationalValue): Rational {
  return create(RationalSchema, {
    numerator: value.numerator.toString(),
    denominator: value.denominator.toString(),
  });
}

/**
 * Reads a `rational_value` or `rational_magnitude`.
 *
 * @throws {MalformedValueError} for one not in lowest terms over a positive
 *   denominator, or one a double holds, which travels as `real_value`.
 */
export function decodeRational(rational: Rational): RationalValue {
  const numerator = decodeBigInteger(rational.numerator);
  const denominator = decodeBigInteger(rational.denominator);
  const text = `${rational.numerator}/${rational.denominator}`;
  if (denominator <= 0n || gcd(numerator, denominator) !== 1n) {
    throw new MalformedValueError(`a Rational ${text} is not in lowest terms over a positive denominator`);
  }
  if (rationalAsDouble({ numerator, denominator }) !== undefined) {
    throw new MalformedValueError(`a Rational ${text} is a double, which travels as real_value`);
  }
  return { numerator, denominator };
}

/** The exact Rational `numerator/denominator`, reduced; throws a RangeError over a zero denominator. */
export function rational(numerator: bigint, denominator = 1n): { kind: "rational" } & RationalValue {
  if (denominator === 0n) {
    throw new RangeError("a Rational's denominator is not zero");
  }
  const sign = denominator < 0n ? -1n : 1n;
  const divisor = gcd(numerator, denominator);
  return { kind: "rational", numerator: (sign * numerator) / divisor, denominator: (sign * denominator) / divisor };
}

function gcd(a: bigint, b: bigint): bigint {
  a = a < 0n ? -a : a;
  b = b < 0n ? -b : b;
  while (b !== 0n) {
    [a, b] = [b, a % b];
  }
  return a;
}

function bitLength(n: bigint): number {
  return n === 0n ? 0 : (n < 0n ? -n : n).toString(2).length;
}

/** The exact Rational a finite double holds. */
export function rationalOfDouble(x: number): { kind: "rational" } & RationalValue {
  if (!Number.isFinite(x)) {
    throw new RangeError(`${x} is no Rational`);
  }
  let whole = x;
  let denominator = 1n;
  while (!Number.isInteger(whole)) {
    whole *= 2;
    denominator *= 2n;
  }
  return rational(BigInt(whole), denominator);
}

/** The double that holds a reduced Rational exactly, or undefined where none does. */
export function rationalAsDouble(value: RationalValue): number | undefined {
  const { numerator, denominator } = value;
  if ((denominator & (denominator - 1n)) !== 0n) {
    return undefined;
  }
  const shift = bitLength(denominator) - 1;
  let odd = numerator;
  let zeros = 0;
  while (odd !== 0n && odd % 2n === 0n) {
    odd /= 2n;
    zeros++;
  }
  if (bitLength(odd) > 53 || zeros - shift < -1074 || bitLength(numerator) - shift > 1024) {
    return undefined;
  }
  return scaleByPowerOfTwo(Number(odd), zeros - shift);
}

/** The double nearest the Rational: how a Real reads one. */
export function rationalToNumber(value: RationalValue): number {
  const exact = rationalAsDouble(value);
  if (exact !== undefined) {
    return exact;
  }
  const negative = value.numerator < 0n;
  const magnitude = negative ? -value.numerator : value.numerator;
  // A quotient of at least 55 bits, its last bit sticky, rounds once to 53.
  const shift = 55 - (bitLength(magnitude) - bitLength(value.denominator));
  const num = shift >= 0 ? magnitude << BigInt(shift) : magnitude;
  const den = shift >= 0 ? value.denominator : value.denominator << BigInt(-shift);
  let quotient = num / den;
  if (num % den !== 0n) {
    quotient |= 1n;
  }
  const result = scaleByPowerOfTwo(Number(quotient), -shift);
  return negative ? -result : result;
}

function scaleByPowerOfTwo(x: number, exponent: number): number {
  while (exponent > 1000) {
    x *= 2 ** 1000;
    exponent -= 1000;
  }
  while (exponent < -1000) {
    x *= 2 ** -1000;
    exponent += 1000;
  }
  return x * 2 ** exponent;
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
    case "rational":
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
    case "metaobject":
      // The element is the identity, whatever type each side was cast to.
      return b.kind === "metaobject" && a.elementId === b.elementId;
    case "undetermined":
      return (
        b.kind === "undetermined" &&
        a.reason === b.reason &&
        a.countLower === b.countLower &&
        a.countUpper === b.countUpper
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
  if (a.kind === "real" && b.kind === "real") {
    return a.value === b.value;
  }
  const x = exactOf(a);
  const y = exactOf(b);
  return x !== undefined && y !== undefined && x.numerator * y.denominator === y.numerator * x.denominator;
}

// The exact number a value holds, a finite Real's double included, never rounding.
function exactOf(value: SysMLValue | Magnitude): RationalValue | undefined {
  switch (value.kind) {
    case "int":
      return { numerator: value.value, denominator: 1n };
    case "rational":
      return value;
    case "real":
      return Number.isFinite(value.value) ? doubleAsRational(value.value) : undefined;
    default:
      return undefined;
  }
}

function doubleAsRational(x: number): RationalValue {
  let denominator = 1n;
  while (!Number.isInteger(x)) {
    x *= 2;
    denominator *= 2n;
  }
  return { numerator: BigInt(x), denominator };
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
  const x = a.magnitude.kind === "real" ? undefined : exactOf(a.magnitude);
  const y = b.magnitude.kind === "real" ? undefined : exactOf(b.magnitude);
  if (x !== undefined && y !== undefined && wholeScale(a.unitTerm) && wholeScale(b.unitTerm)) {
    // m₁·n₁/d₁ = m₂·n₂/d₂ exactly, cross-multiplied in bigint.
    return (
      x.numerator * y.denominator * BigInt(a.unitTerm.scaleNum) * BigInt(b.unitTerm.scaleDen) ===
      y.numerator * x.denominator * BigInt(b.unitTerm.scaleNum) * BigInt(a.unitTerm.scaleDen)
    );
  }
  return baseMagnitude(a.magnitude, a.unitTerm) === baseMagnitude(b.magnitude, b.unitTerm);
}

function baseMagnitude(magnitude: Magnitude, term: UnitFactorization): number {
  const value =
    magnitude.kind === "rational" ? rationalToNumber(magnitude) : Number(magnitude.value);
  return (value * term.scaleNum) / term.scaleDen;
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
      case "bigIntValue":
        return { kind: "int", value: decodeBigInteger(component.kind.value) };
      case "realValue":
        return { kind: "real", value: component.kind.value };
      case "rationalValue":
        return { kind: "rational", ...decodeRational(component.kind.value) };
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
    const scalar = decodeValue(literal.value);
    if (scalar.kind === "absent") {
      throw new MalformedValueError("an enumeration literal's value, when present, states a scalar");
    }
    decoded.value = scalar;
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

/** What a caller may pass as an argument or input to a run. A wire `Value`
 * passes through untouched, which is how a caller sends a shape the typing
 * layer would normalize away — including one the service is meant to refuse.
 * A JavaScript array encodes as a sequence and a `Set` as a set, as Python's
 * list and set do; collection elements accept every input form, not only
 * `SysMLValue`s. */
export type ValueInput =
  | SysMLValue
  | Value
  | boolean
  | string
  | bigint
  | number
  | readonly ValueInput[]
  | ReadonlySet<ValueInput>
  | { kind: "sequence"; elements: readonly ValueInput[] }
  | { kind: "set"; elements: readonly ValueInput[] }
  | ({ kind: "array"; elements: readonly ValueInput[] } & Omit<ArrayValue, "elements">);

/** Refuses an instance or `self` id outside int64, the width of the wire's id fields. */
function checkInt64(value: bigint): void {
  if (!fitsInt64(value)) {
    throw new RangeError(`value out of range: ${value.toString()}`);
  }
}

/**
 * Encodes a caller-facing value for the wire, requiring of `info` each
 * capability a value kind needs before the call is sent — exactly the checks
 * Python's `Connection._python_to_value` makes. A `SysMLValue` encodes as
 * itself; a boolean, string, bigint and number encode as the scalar arms; a
 * JavaScript array and `Set` encode as a sequence and a set.
 */
export function toValue(input: ValueInput, info: ServerInfo): Value {
  const probe: unknown = input;
  if (typeof probe === "object" && probe !== null && "$typeName" in probe) {
    return probe as Value;
  }
  return encodeInput(normalizeInput(input), info);
}

/** The `SysMLValue` one input names, collection elements converted the same way. */
function normalizeInput(input: unknown): SysMLValue {
  switch (typeof input) {
    case "boolean":
      return { kind: "boolean", value: input };
    case "bigint":
      return { kind: "int", value: input };
    case "number":
      return { kind: "real", value: input };
    case "string":
      return { kind: "string", value: input };
    case "undefined":
      throw new RangeError("unsupported input type: undefined");
    case "function":
    case "symbol":
    case "object":
      break;
  }
  if (input === null) {
    return { kind: "null", reason: "" };
  }
  if (Array.isArray(input)) {
    return { kind: "sequence", elements: input.map(normalizeInput) };
  }
  if (input instanceof Set) {
    return { kind: "set", elements: [...input].map(normalizeInput) };
  }
  const value = input as SysMLValue;
  switch (value.kind) {
    case "sequence":
      return { kind: "sequence", elements: value.elements.map(normalizeInput) };
    case "set":
      return { kind: "set", elements: value.elements.map(normalizeInput) };
    case "array":
      return {
        kind: "array",
        dimensions: value.dimensions,
        elements: value.elements.map(normalizeInput),
      };
    case "enum":
      return value.value.value === undefined
        ? value
        : { kind: "enum", value: { ...value.value, value: normalizeInput(value.value.value) } };
    case "unset":
      throw new RangeError(
        "an unset value cannot be sent as an input; it is a value the service answers, not one it takes",
      );
    default:
      if (typeof (value as { kind?: unknown }).kind === "string") {
        return value;
      }
      throw new RangeError(`unsupported input type: ${typeof input}`);
  }
}

function encodeInput(value: SysMLValue, info: ServerInfo): Value {
  requireInput(value, info);
  switch (value.kind) {
    case "sequence":
      return create(ValueSchema, {
        kind: {
          case: "sequence",
          value: create(ValueSequenceSchema, {
            elements: value.elements.map((element) => encodeInput(element, info)),
          }),
        },
      });
    case "array":
      checkShape("an array", value.dimensions, value.elements.length);
      return create(ValueSchema, {
        kind: {
          case: "array",
          value: create(ArraySchema, {
            dimensions: value.dimensions,
            elements: value.elements.map((element) => encodeInput(element, info)),
          }),
        },
      });
    case "set":
      return create(ValueSchema, {
        kind: {
          case: "set",
          value: create(ValueSetSchema, {
            elements: uniqueMembers(value.elements).map((element) => encodeInput(element, info)),
          }),
        },
      });
    default: {
      const wire = encodeValue(value);
      if (!info.has(CAPABILITY_RATIONAL_VALUES)) {
        rationalsAsReals(wire);
      }
      return wire;
    }
  }
}

/** The capabilities one input value needs of the service that will decode it. */
export function requireInput(value: SysMLValue, info: ServerInfo): void {
  const require = (capability: string): void => {
    requireCapability(info, capability, upgradeRemedy(capability));
  };
  const requireMagnitudes = (magnitudes: Magnitude[]): void => {
    if (magnitudes.some((magnitude) => magnitude.kind === "int" && !fitsInt64(magnitude.value))) {
      require(CAPABILITY_BIG_INT_VALUES);
    }
    if (magnitudes.some((magnitude) => magnitude.kind === "rational" && rationalAsDouble(magnitude) === undefined)) {
      require(CAPABILITY_RATIONAL_VALUES);
    }
  };
  switch (value.kind) {
    case "int":
    case "rational":
      requireMagnitudes([value]);
      return;
    case "complex":
      require(CAPABILITY_COMPLEX_VALUES);
      return;
    case "quantity":
      requireMagnitudes([value.magnitude]);
      checkReduction(`quantity in [${value.unit}]`, value.unit, value.unitTerm);
      return;
    case "measurementRef": {
      require(CAPABILITY_MEASUREMENT_REFS);
      const unitTerm: unknown = value.unitTerm;
      if (value.unit === "" && (value.unitId ?? "") === "" && unitTerm === undefined) {
        throw new UnsupportedValueError("measurement reference naming no unit");
      }
      // The wire needs the reduction on every reference, id-only ones too;
      // encodeMeasurementRef reads unitTerm unconditionally.
      if (unitTerm === undefined) {
        throw new UnsupportedValueError(
          `measurement reference ${value.unit || value.unitId} carries no reduction to base units, so the service cannot tell what it ` +
            "measures: build it from a unit the service sent, or from one the model declares",
        );
      }
      return;
    }
    case "instance":
      checkInt64(value.id);
      return;
    case "unset":
      throw new RangeError(
        "an unset value cannot be sent as an input; it is a value the service answers, not one it takes",
      );
    case "function":
      require(CAPABILITY_FUNCTION_VALUES);
      if (value.selfId !== undefined) {
        checkInt64(value.selfId);
      }
      return;
    case "metaobject":
      require(CAPABILITY_METAOBJECT_VALUES);
      return;
    case "array":
      require(CAPABILITY_STRUCTURED_VALUES);
      value.elements.forEach((element) => {
        requireInput(element, info);
      });
      return;
    case "vector":
      require(CAPABILITY_STRUCTURED_VALUES);
      requireMagnitudes(value.components);
      return;
    case "vectorQuantity":
      require(CAPABILITY_STRUCTURED_VALUES);
      requireMagnitudes(value.components.map((component) => component.magnitude));
      value.components.forEach((component) => {
        checkReduction(`quantity in [${component.unit}]`, component.unit, component.unitTerm);
      });
      return;
    case "set":
      require(CAPABILITY_SET_VALUES);
      value.elements.forEach((element) => {
        requireInput(element, info);
      });
      return;
    case "tensorQuantity":
      require(CAPABILITY_TENSOR_VALUES);
      requireMagnitudes(value.components.map((component) => component.magnitude));
      value.components.forEach((component) => {
        checkReduction(`quantity in [${component.unit}]`, component.unit, component.unitTerm);
      });
      return;
    case "infinity":
      require(CAPABILITY_INFINITY_VALUE);
      return;
    case "enum":
      if (value.value.value !== undefined) {
        requireInput(value.value.value, info);
      }
      return;
    case "sequence":
      value.elements.forEach((element) => {
        requireInput(element, info);
      });
      return;
    default:
      return;
  }
}

/**
 * Refuses a unit named without its reduction to base units — the service
 * decides commensurability over the reduction and rejects a unit sent without
 * one; an unnamed unit means dimension one and carries none.
 */
function checkReduction(
  what: string,
  unit: string,
  unitTerm: UnitFactorization | undefined,
): void {
  if (unit !== "" && unitTerm === undefined) {
    throw new UnsupportedValueError(
      `${what} carries no reduction to base units, so the service cannot tell what it ` +
        "measures: build it from a unit the service sent, or from one the model declares",
    );
  }
}
