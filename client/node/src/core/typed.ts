// Runtime support for generated, statically typed views over instances.
//
// A generated class derives from TypedObject and exposes one getter per SysML
// feature, each delegating to the underlying Tier 1 Instance through
// featureValue, optionalFeatureValue or listFeatureValue. Decoding is
// unchanged: this layer only states what type a decoded value is expected to
// have, and reports a mismatch rather than returning a wrongly typed value.

import {
  FeatureValueError,
  InstanceTypeError,
  TypeMismatchError,
} from "./errors.js";
import type { Instance } from "./model.js";
import type {
  ComplexValue,
  EnumValue,
  QuantityValue,
  SysMLValue,
} from "./values.js";

/** A quantity feature value: a magnitude and the unit it is expressed in. */
export type Quantity = { kind: "quantity" } & QuantityValue;

/** Resolves a nested feature value's instance id to its Instance, when known. */
export type InstanceResolver = (id: bigint) => Instance | undefined;

/** One decoder: a raw feature value to the typed value a getter returns. */
export type FeatureDecoder<T> = (
  featureName: string,
  value: SysMLValue,
  resolve: InstanceResolver | undefined,
) => T;

const ABSENT = Symbol("absent");

type RawValue = typeof ABSENT | SysMLValue | readonly SysMLValue[];

function readRaw(
  instance: Instance,
  featureName: string,
): RawValue {
  const feature = instance.get(featureName);
  if (feature === undefined) {
    return ABSENT;
  }
  if (feature.kind === "error") {
    throw new FeatureValueError(featureName, feature.error);
  }
  if (feature.kind === "many") {
    return feature.values;
  }
  if (feature.value.kind === "absent") {
    if (feature.materialized) {
      return [];
    }
    throw new FeatureValueError(featureName, "feature value is not materialized");
  }
  return feature.value;
}

function isMany(raw: RawValue): raw is readonly SysMLValue[] {
  return Array.isArray(raw);
}

function isUnset(value: SysMLValue): boolean {
  return value.kind === "unset";
}

function isNull(value: SysMLValue): boolean {
  return value.kind === "null";
}

function mismatch(featureName: string, expected: string, value: unknown): TypeMismatchError {
  return new TypeMismatchError(featureName, expected, value);
}

// Every sysmlId a generated class in this process answers for, so a wrong type
// is distinguishable from one this client has never heard of.
const GENERATED = new Map<string, TypedObjectClass>();

/** Register a generated class so {@link TypedObject.fromInstance} can tell a wrong type from an unknown one. */
export function registerTyped(ctor: TypedObjectClass): void {
  if (ctor.sysmlId !== "") {
    GENERATED.set(ctor.sysmlId, ctor);
  }
}

/** Base class of generated typed views over an {@link Instance}. */
export class TypedObject {
  /** FQN of the SysML definition this class was generated from. */
  static readonly sysmlId: string = "";

  readonly #instance: Instance;
  readonly #resolve: InstanceResolver | undefined;

  constructor(instance: Instance, resolve?: InstanceResolver) {
    this.#instance = instance;
    this.#resolve = resolve;
  }

  /** Return a typed view over `instance`, rejecting one of another type.
   *
   * Accepted: an instance of exactly this class's definition; an instance of a
   * definition that specializes it and has a generated class of its own; and an
   * instance whose type no generated class in this process describes — the
   * service reports the type of an instantiated *usage* as the usage's own
   * FQN, which no generated class carries, so it cannot be related to a
   * definition at all.
   *
   * Use {@link TypedObject.unchecked} to bypass this deliberately.
   */
  static fromInstance<C extends TypedObjectClass>(
    this: C,
    instance: Instance,
    resolve?: InstanceResolver,
  ): InstanceType<C> {
    const actual = instance.typeId;
    const generated = actual === "" ? undefined : GENERATED.get(actual);
    if (actual !== "" && actual !== this.sysmlId && generated !== undefined) {
      if (!(generated.prototype instanceof this)) {
        throw new InstanceTypeError(this.sysmlId || this.name, actual);
      }
    }
    return new this(instance, resolve) as InstanceType<C>;
  }

  /** Return a typed view over `instance` without checking its type. */
  static unchecked<C extends TypedObjectClass>(
    this: C,
    instance: Instance,
    resolve?: InstanceResolver,
  ): InstanceType<C> {
    return new this(instance, resolve) as InstanceType<C>;
  }

  /** The underlying Tier 1 instance. */
  get instance(): Instance {
    return this.#instance;
  }

  /** The resolver nested feature values resolve instance ids through, if any. */
  get resolver(): InstanceResolver | undefined {
    return this.#resolve;
  }

  /** Two views are equal over the same instance of the same generated class. */
  equals(other: unknown): boolean {
    return (
      other instanceof TypedObject &&
      other.constructor === this.constructor &&
      other.instance.id === this.instance.id
    );
  }

  toString(): string {
    return `${this.constructor.name}(instance=${this.#instance.id.toString()})`;
  }
}

/** Decode a Boolean feature value. */
export function asBoolean(featureName: string, value: SysMLValue): boolean {
  if (value.kind === "boolean") {
    return value.value;
  }
  throw mismatch(featureName, "bool", value);
}

/** Decode an Integer/Natural feature value. */
export function asInt(featureName: string, value: SysMLValue): bigint {
  if (value.kind === "int") {
    return value.value;
  }
  throw mismatch(featureName, "int", value);
}

/** Decode a Real/Rational feature value; an integer value widens to float. */
export function asReal(featureName: string, value: SysMLValue): number {
  if (value.kind === "real") {
    return value.value;
  }
  if (value.kind === "int") {
    return Number(value.value);
  }
  throw mismatch(featureName, "float", value);
}

/** Decode a Complex feature value; a real value widens to complex. */
export function asComplex(featureName: string, value: SysMLValue): ComplexValue {
  if (value.kind === "complex") {
    return value.value;
  }
  if (value.kind === "real") {
    return { real: value.value, imaginary: 0 };
  }
  if (value.kind === "int") {
    return { real: Number(value.value), imaginary: 0 };
  }
  throw mismatch(featureName, "complex", value);
}

/** Decode a String feature value. */
export function asString(featureName: string, value: SysMLValue): string {
  if (value.kind === "string") {
    return value.value;
  }
  throw mismatch(featureName, "str", value);
}

/** Decode a quantity feature value: a magnitude and the unit it is expressed in. */
export function asQuantity(featureName: string, value: SysMLValue): Quantity {
  if (value.kind === "quantity") {
    return value;
  }
  throw mismatch(featureName, "Quantity", value);
}

/** Decode an enumeration-typed feature, which holds a literal rather than an instance. */
export function asEnum(featureName: string, value: SysMLValue): EnumValue {
  if (value.kind === "enum") {
    return value.value;
  }
  throw mismatch(featureName, "EnumLiteral", value);
}

/** Decode a feature value whose SysML type has no sound TypeScript type. */
export function asObject(_featureName: string, value: SysMLValue): SysMLValue {
  return value;
}

/** A generated typed class: the constructor plus the sysmlId it was generated for. */
export interface TypedObjectClass<T extends TypedObject = TypedObject> {
  new (instance: Instance, resolve?: InstanceResolver): T;
  readonly sysmlId: string;
}

/** Return a decoder wrapping a nested instance in the generated class `cls`. */
export function asTyped<T extends TypedObject>(
  cls: TypedObjectClass<T>,
): FeatureDecoder<T> {
  return (featureName, value, resolve) => {
    if (value.kind === "instance") {
      const nested = resolve?.(value.id);
      if (nested !== undefined) {
        return TypedObject.fromInstance.call(cls, nested, resolve) as T;
      }
    }
    throw mismatch(featureName, cls.name, value);
  };
}

/** The decoded value of a required single-valued feature. */
export function featureValue<T>(
  obj: TypedObject,
  featureName: string,
  decode: FeatureDecoder<T>,
): T {
  const raw = readRaw(obj.instance, featureName);
  if (raw === ABSENT) {
    throw mismatch(featureName, "a value", undefined);
  }
  if (isMany(raw)) {
    throw mismatch(featureName, "a single value", raw);
  }
  if (isNull(raw)) {
    throw mismatch(featureName, "a value", null);
  }
  return decode(featureName, raw, obj.resolver);
}

/** The decoded value of a `0..1` feature, or undefined when it holds no value. */
export function optionalFeatureValue<T>(
  obj: TypedObject,
  featureName: string,
  decode: FeatureDecoder<T>,
): T | undefined {
  const raw = readRaw(obj.instance, featureName);
  if (raw === ABSENT) {
    return undefined;
  }
  if (isMany(raw)) {
    if (raw.length === 0) {
      return undefined;
    }
    if (raw.length > 1) {
      throw mismatch(featureName, "at most one value", raw);
    }
    const element = raw[0];
    return isNull(element) || isUnset(element)
      ? undefined
      : decode(featureName, element, obj.resolver);
  }
  if (isNull(raw) || isUnset(raw)) {
    return undefined;
  }
  return decode(featureName, raw, obj.resolver);
}

/** The decoded values of a multi-valued feature; one holding no value is empty. */
export function listFeatureValue<T>(
  obj: TypedObject,
  featureName: string,
  decode: FeatureDecoder<T>,
): T[] {
  const raw = readRaw(obj.instance, featureName);
  if (raw === ABSENT) {
    return [];
  }
  if (isMany(raw)) {
    return raw.map((element) => decode(featureName, element, obj.resolver));
  }
  if (isNull(raw) || isUnset(raw)) {
    return [];
  }
  return [decode(featureName, raw, obj.resolver)];
}
