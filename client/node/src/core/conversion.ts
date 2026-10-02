// Writing a model back out, in SysML notation or RDF Turtle. The formats are
// named as the `sysml` CLI's -from/-to name them.

import { byCodeUnit } from "./capabilities.js";
import type { ModelDiagnostic } from "./errors.js";

/** SysML v2 / KerML textual notation. `kerml` and `text` name it too. */
export const FORMAT_SYSML = "sysml";
/** RDF in Turtle syntax. `turtle` and `rdf` name it too. */
export const FORMAT_TURTLE = "ttl";
/** The OMG API's JSON element form, the same RDF mapping spelled differently. `json` names it too. */
export const FORMAT_API_JSON = "api-json";

const TURTLE_NAMES = new Set(["ttl", "turtle", "rdf"]);
const API_JSON_NAMES = new Set(["api-json", "json"]);
const XMI_NAMES = new Set(["xmi", "uml", "mdzip"]);

/** The fallback wording, for a service too old to send its own notice. */
export const EXPERIMENTAL_NOTICE =
  "RDF conversion — Turtle and the API's JSON element form alike — is experimental: the " +
  "mapping covers model structure and the behavior its bodies state, refuses what it " +
  "cannot write back, and its vocabulary may change without a compatibility path; see " +
  "docs/reference/rdf-mapping.md § Status";

/** Whether a conversion between these formats uses an experimental mapping. */
export function isExperimental(fromFormat: string, toFormat: string): boolean {
  return (
    TURTLE_NAMES.has(fromFormat) ||
    TURTLE_NAMES.has(toFormat) ||
    API_JSON_NAMES.has(fromFormat) ||
    API_JSON_NAMES.has(toFormat) ||
    XMI_NAMES.has(fromFormat)
  );
}

const EXTENSIONS = new Map<string, string>([
  [".sysml", FORMAT_SYSML],
  [".kerml", FORMAT_SYSML],
  [".ttl", FORMAT_TURTLE],
  [".turtle", FORMAT_TURTLE],
  [".json", FORMAT_API_JSON],
]);

/** The format to write `path` as, from its extension. */
export function formatOfPath(path: string): string {
  const dot = path.lastIndexOf(".");
  const ext = dot === -1 ? "" : path.slice(dot).toLowerCase();
  const format = EXTENSIONS.get(ext);
  if (format === undefined) {
    const known = [...EXTENSIONS.keys()].sort(byCodeUnit).join(", ");
    throw new RangeError(
      `cannot tell the format to write ${JSON.stringify(path)} as: expected one of ${known}, ` +
        `or pass toFormat explicitly`,
    );
  }
  return format;
}

/** A model written out in one of the formats OpenSysML writes. */
export class Conversion {
  /** The converted model. */
  readonly content: string;
  /** Format the source was read as, reported even when inferred. */
  readonly fromFormat: string;
  /** Format `content` is written in. */
  readonly toFormat: string;
  /** Syntax errors the service tolerated under `tolerateSyntaxErrors`. */
  readonly diagnostics: readonly ModelDiagnostic[];
  /** True when the conversion went through the RDF mapping, which is experimental. */
  readonly experimental: boolean;
  /** What is experimental about it, in the service's own wording; empty when stable. */
  readonly experimentalNotice: string;

  constructor(init: {
    content: string;
    fromFormat: string;
    toFormat: string;
    diagnostics: readonly ModelDiagnostic[];
    experimental: boolean;
    experimentalNotice: string;
  }) {
    this.content = init.content;
    this.fromFormat = init.fromFormat;
    this.toFormat = init.toFormat;
    this.diagnostics = init.diagnostics;
    this.experimental = init.experimental;
    this.experimentalNotice = init.experimentalNotice;
  }

  toString(): string {
    return this.content;
  }

  get length(): number {
    return this.content.length;
  }
}
