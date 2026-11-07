/**
 * Reading an element's metamodel properties by their specification names.
 *
 * The `GetElementProperties` RPC answers the KerML 1.1 / SysML 2.1 abstract
 * syntax of one element — `inheritedFeature`, `connectorEnd`,
 * `directedFeature`, `payloadType`, ... — as the model's API element form
 * (`Convert` to `api-json`) spells it, without converting the whole model. An
 * element is named by qualified name, by the positional id of an unnamed
 * member (`Owner::@2`) or by the element form's own id, which every element
 * reference in an answer carries, so a client walks the graph with one vocabulary.
 */

import type {
  ElementPropertyValue as ElementPropertyValueMessage,
  ElementRef,
  GetElementPropertiesResponse,
} from "../generated/sysml_pb.js";

/** An element the model's element form knows: one of the model, or a standard-library element it references. */
export class ElementIdentity {
  /** The element form's `@id`, accepted back as the element to read. */
  readonly id: string;
  /** Empty for an unnamed element; an unnamed standard-library member carries its library path (`owner/Metaclass[i]`). */
  readonly qualifiedName: string;
  /** `ConnectionUsage`, `PartDefinition`, ...; empty when the id is known to nothing. */
  readonly metaclass: string;

  constructor(fields: { id: string; qualifiedName?: string; metaclass?: string }) {
    this.id = fields.id;
    this.qualifiedName = fields.qualifiedName ?? "";
    this.metaclass = fields.metaclass ?? "";
  }

  /** The qualified name, or the id for an unnamed element. */
  toString(): string {
    return this.qualifiedName === "" ? this.id : this.qualifiedName;
  }
}

/**
 * One value of a metamodel property, as the element form spells it: an element
 * reference, a string (an enumeration literal included), a boolean, a number,
 * or a `bigint` for an integer beyond the safe range. A number beyond `int64`
 * and `float64` arrives as its decimal text.
 */
export type PropertyValue = ElementIdentity | string | boolean | number | bigint;

/** One metamodel property of an element. */
export class ElementProperty {
  /** The specification name (`connectorEnd`). */
  readonly name: string;
  /** The metaclass declaring the property (`Connector`); empty when the element's metaclass declares no such property. */
  readonly definingClass: string;
  /** Whether the property holds a collection. */
  readonly many: boolean;
  /** Whether the specification derives the property. */
  readonly derived: boolean;
  /** The values in the collection's order; empty for an empty or null property. */
  readonly values: readonly PropertyValue[];
  /**
   * Why there are no values when the property cannot be answered: the metaclass
   * declares no such property, or OpenSysML does not derive it yet. Empty for a
   * property that is answered.
   */
  readonly unsupported: string;

  constructor(fields: {
    name: string;
    definingClass?: string;
    many?: boolean;
    derived?: boolean;
    values?: readonly PropertyValue[];
    unsupported?: string;
  }) {
    this.name = fields.name;
    this.definingClass = fields.definingClass ?? "";
    this.many = fields.many ?? false;
    this.derived = fields.derived ?? false;
    this.values = fields.values ?? [];
    this.unsupported = fields.unsupported ?? "";
  }

  /** Whether the property was answered, with or without values. */
  get supported(): boolean {
    return this.unsupported === "";
  }

  /** The one value of a single-valued property, `undefined` when it has none. */
  get value(): PropertyValue | undefined {
    return this.values[0];
  }

  /** The element references among the values, in order. */
  get elements(): ElementIdentity[] {
    return this.values.filter((value): value is ElementIdentity => value instanceof ElementIdentity);
  }
}

/** What one read answered: the element and its properties, in the order asked for, or the element form's order when none was named. */
export class ElementProperties {
  readonly element: ElementIdentity;
  readonly properties: readonly ElementProperty[];
  private readonly byName: Map<string, ElementProperty>;

  constructor(element: ElementIdentity, properties: readonly ElementProperty[]) {
    this.element = element;
    this.properties = properties;
    this.byName = new Map(properties.map((property) => [property.name, property]));
  }

  /** The entry for the named property, `undefined` when the read did not ask for it. */
  get(name: string): ElementProperty | undefined {
    return this.byName.get(name);
  }

  /** Whether the read answered the named property. */
  has(name: string): boolean {
    return this.byName.has(name);
  }

  [Symbol.iterator](): Iterator<ElementProperty> {
    return this.properties[Symbol.iterator]();
  }
}

export function elementIdentityOf(reference: ElementRef | undefined): ElementIdentity {
  return new ElementIdentity({
    id: reference?.id ?? "",
    qualifiedName: reference?.qualifiedName ?? "",
    metaclass: reference?.metaclass ?? "",
  });
}

export function propertyValueOf(message: ElementPropertyValueMessage): PropertyValue {
  const value = message.value;
  switch (value.case) {
    case "element":
      return elementIdentityOf(value.value);
    case "stringValue":
    case "number":
      return value.value;
    case "boolValue":
      return value.value;
    case "intValue":
      return Number.isSafeInteger(Number(value.value)) ? Number(value.value) : value.value;
    case "realValue":
      return value.value;
    default:
      throw new Error("an ElementPropertyValue with no value");
  }
}

/** Decodes a `GetElementPropertiesResponse` the service answered without error. */
export function elementPropertiesOf(response: GetElementPropertiesResponse): ElementProperties {
  return new ElementProperties(
    elementIdentityOf(response.element),
    response.properties.map(
      (property) =>
        new ElementProperty({
          name: property.name,
          definingClass: property.definingClass,
          many: property.many,
          derived: property.derived,
          values: property.values.map(propertyValueOf),
          unsupported: property.unsupported,
        }),
    ),
  );
}
