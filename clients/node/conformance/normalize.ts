// Turning a response into the JSON-shaped tree scenarios compare against, with
// the values that are not the same twice replaced. See conformance/README.md.

import { ScalarType, type DescField, type DescMessage, type Message } from "@bufbuild/protobuf";
import { isAbsolute } from "node:path";
import { reflect, type ReflectMessage } from "@bufbuild/protobuf/reflect";

import { byCodeUnit } from "./order.js";

export const MODEL_HASH_PLACEHOLDER = "${model_hash}";
export const VERSION_PLACEHOLDER = "${version}";
export const PATH_PLACEHOLDER = "${path}";

/** The int64 fields carrying a runtime instance id, relabelled per call. */
const NORMALIZED_IDS = new Set([
  "sysml.Instance.id",
  "sysml.Value.instance_id",
  "sysml.Verdict.instance_id",
  "sysml.Function.self_id",
]);

/**
 * An integral value, kept as its digits. Distinct from a number so an integral
 * field is compared exactly rather than within the tolerance a Real carries.
 */
export class Integer {
  constructor(readonly digits: string) {}

  toString(): string {
    return this.digits;
  }
}

/** A normalized response: maps, lists, strings, numbers, Integers and booleans. */
export type Normalized = Record<string, unknown>;

/** Replaces the values a call cannot repeat, and labels its instance ids. */
export class Normalizer {
  private readonly labels = new Map<string, string>();

  constructor(private readonly modelHash: string) {}

  /** Renders a message as a tree. Only set fields appear. */
  normalize(schema: DescMessage, message: Message): Normalized {
    return this.message(reflect(schema, message));
  }

  private message(reflected: ReflectMessage): Normalized {
    const out: Normalized = {};
    // Number order, so instance id labels are assigned reproducibly.
    const fields = [...reflected.fields].sort((left, right) => left.number - right.number);
    for (const field of fields) {
      if (!reflected.isSet(field)) {
        continue;
      }
      out[field.name] = this.field(field, reflected);
    }
    return out;
  }

  private field(field: DescField, reflected: ReflectMessage): unknown {
    switch (field.fieldKind) {
      case "list": {
        const list = reflected.get(field);
        return [...list].map((item) => this.element(field, item));
      }
      case "map": {
        const map = reflected.get(field);
        const entries: Normalized = {};
        const keys = [...map.keys()].map(String).sort(byCodeUnit);
        for (const key of keys) {
          entries[key] = this.element(field, map.get(key));
        }
        return entries;
      }
      case "message":
        return this.message(reflected.get(field));
      case "enum":
        return this.enumName(field, reflected.get(field));
      case "scalar":
        return this.scalar(field, reflected.get(field));
    }
  }

  /** Normalizes one list element or map value, whose kind the field declares. */
  private element(field: DescField, value: unknown): unknown {
    let kind: "message" | "enum" | "scalar" | undefined;
    if (field.fieldKind === "list") {
      kind = field.listKind;
    } else if (field.fieldKind === "map") {
      kind = field.mapKind;
    }
    switch (kind) {
      case "message":
        return this.message(value as ReflectMessage);
      case "enum":
        return this.enumName(field, value);
      default:
        return this.scalar(field, value);
    }
  }

  private enumName(field: DescField, value: unknown): string {
    const descriptor = field.enum;
    const number = Number(value);
    const literal = descriptor?.values.find((candidate) => candidate.number === number);
    return literal?.name ?? String(number);
  }

  private scalar(field: DescField, value: unknown): unknown {
    const scalar = field.scalar ?? (field.fieldKind === "map" ? field.scalar : undefined);
    const full = `${field.parent.typeName}.${field.name}`;
    switch (scalar) {
      case ScalarType.DOUBLE:
      case ScalarType.FLOAT:
        return Number(value);
      case ScalarType.INT64:
      case ScalarType.SINT64:
      case ScalarType.SFIXED64:
        return NORMALIZED_IDS.has(full) ? this.label(String(value)) : new Integer(String(value));
      case ScalarType.INT32:
      case ScalarType.SINT32:
      case ScalarType.SFIXED32:
      case ScalarType.UINT32:
      case ScalarType.UINT64:
      case ScalarType.FIXED32:
      case ScalarType.FIXED64:
        return new Integer(String(value));
      case ScalarType.BOOL:
        return Boolean(value);
      case ScalarType.BYTES:
        return Buffer.from(value as Uint8Array).toString("base64");
      default:
        return this.string(full, String(value));
    }
  }

  /** Replaces the model hash it was given, the build version, and absolute paths. */
  private string(field: string, text: string): string {
    if (field === "sysml.ServerInfoResponse.version") {
      return VERSION_PLACEHOLDER;
    }
    if (this.modelHash !== "" && text === this.modelHash) {
      return MODEL_HASH_PLACEHOLDER;
    }
    if (isAbsolute(text)) {
      return PATH_PLACEHOLDER;
    }
    return text;
  }

  /** The symbolic name of a runtime instance id. */
  private label(id: string): string {
    const existing = this.labels.get(id);
    if (existing !== undefined) {
      return existing;
    }
    const label = `@${this.labels.size + 1}`;
    this.labels.set(id, label);
    return label;
  }
}
