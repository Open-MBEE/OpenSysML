// Writing a model back out, in SysML notation or RDF Turtle. The formats are
// named as the `sysml` CLI's -from/-to name them.

import type { ModelDiagnostic } from "./errors.js";
import type { MigrationReport as PbMigrationReport } from "../generated/sysml_pb.js";

/** SysML v2 / KerML textual notation. `kerml` and `text` name it too. */
export const FORMAT_SYSML = "sysml";
/** RDF in Turtle syntax. `turtle` and `rdf` name it too. */
export const FORMAT_TURTLE = "ttl";
/** The OMG API's JSON element form, the same RDF mapping spelled differently. `json` names it too. */
export const FORMAT_API_JSON = "api-json";

const TURTLE_NAMES = new Set(["ttl", "turtle", "rdf"]);
const API_JSON_NAMES = new Set(["api-json", "json"]);
const XMI_NAMES = new Set(["xmi", "uml", "mdzip"]);

/** Whether a format names SysML v1 in any of its forms: `xmi`, `uml` or `mdzip`. */
export function isV1(format: string): boolean {
  return XMI_NAMES.has(format);
}

/** Whether a path's extension names a SysML v1 model: `.xmi`, `.uml` or `.mdzip`. */
export function pathIsV1(path: string): boolean {
  const dot = path.lastIndexOf(".");
  return dot !== -1 && XMI_NAMES.has(path.slice(dot + 1).toLowerCase());
}

/** Why a v1 model is refused by a conversion, in the words every surface uses. */
export const MIGRATED_NOT_CONVERTED =
  "is a SysML v1 model, which is migrated, not converted: every element is mapped, " +
  "approximated or left unmapped and reported element by element";

/** The fallback wording, for a service too old to send its own notice. */
export const EXPERIMENTAL_NOTICE =
  "RDF conversion — Turtle and the API's JSON element form alike — is experimental: the " +
  "mapping covers model structure and the behavior its bodies state, refuses what it " +
  "cannot write back, and its vocabulary may change without a compatibility path; see " +
  "docs/reference/rdf-mapping.md § Status";

/** The fallback wording for a migration, for a service too old to send its own notice. */
export const MIGRATION_NOTICE =
  "SysML v1 migration is experimental: the mapping covers structure, ports and connectors, " +
  "requirements, constraints, instances and allocations, reports every element it approximates " +
  "or leaves behind, and what it writes for a v1 element may change without a compatibility " +
  "path; see docs/reference/sysml-v1-migration.md § Status";

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
    const known = [...EXTENSIONS.keys()].sort().join(", ");
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

/** What a migration may be asked for beside the target format. */
export interface MigrateOptions {
  /** The v1 form the source is in: `xmi`, `uml` or `mdzip`. Required for inline content; a path's extension names it otherwise. */
  fromFormat?: string;
  /** Answer every element's verdict and the report text `-migration-report` writes. */
  report?: boolean;
  /** Answer the JSON index of result snapshots `-migration-results` writes. */
  results?: boolean;
  /** Path of an MTIP layout file the service reads, as `-layout` names it. */
  layoutPath?: string;
  /** An MTIP layout carried inline; at most one of `layoutPath` and `layoutContent`. */
  layoutContent?: string;
  /** Prefix the migrated model refers to its image files with, as `-image-base-url` sets it. */
  imageBaseUrl?: string;
  /** Refuse a model with an unmapped element rather than reporting it, as `-strict` does. */
  strict?: boolean;
}

/** One SysML v1 element's verdict in a migration. */
export interface MigrationEntry {
  /** The element's `xmi:id`. */
  readonly id: string;
  /** Its v1 metaclass, with its applied stereotypes. */
  readonly kind: string;
  /** Its qualified name in the v1 model. */
  readonly name: string;
  /** The v2 element it was written as, when it was written. */
  readonly target: string;
  /** `mapped`, `approximated`, `unmapped` or `skipped`. */
  readonly verdict: string;
  /** Why the verdict is what it is, in the migrator's words. */
  readonly note: string;
}

/**
 * The account a migration gives of itself: what became of every element. The
 * summary and the four counts always come back; `entries` and `text` when the
 * migration was asked for the report.
 */
export class MigrationReport {
  /** The v1 model migrated, as the service named it. */
  readonly source: string;
  /** The tool that exported it, as its XMI says. */
  readonly exporter: string;
  /** The one-line account: `migrated N element(s): … mapped, … approximated, … unmapped`. */
  readonly summary: string;
  /** Elements with a faithful v2 form. */
  readonly mapped: number;
  /** Elements written in a v2 form that is not quite theirs. */
  readonly approximated: number;
  /** Elements with no v2 form, left out and reported. */
  readonly unmapped: number;
  /** Elements the migration does not consider: profile, library and notation-only content, and elements nothing refers to. */
  readonly skipped: number;
  /** Every element's verdict, when the report was asked for. */
  readonly entries: readonly MigrationEntry[];
  /** The report as `-migration-report` writes it, when asked for; else empty. */
  readonly text: string;

  constructor(init: {
    source: string;
    exporter: string;
    summary: string;
    mapped: number;
    approximated: number;
    unmapped: number;
    skipped: number;
    entries: readonly MigrationEntry[];
    text: string;
  }) {
    this.source = init.source;
    this.exporter = init.exporter;
    this.summary = init.summary;
    this.mapped = init.mapped;
    this.approximated = init.approximated;
    this.unmapped = init.unmapped;
    this.skipped = init.skipped;
    this.entries = init.entries;
    this.text = init.text;
  }

  /** The entries with `verdict`: `mapped`, `approximated`, `unmapped` or `skipped`. */
  byVerdict(verdict: string): MigrationEntry[] {
    return this.entries.filter((entry) => entry.verdict === verdict);
  }

  toString(): string {
    return this.text !== "" ? this.text : this.summary;
  }
}

/** Reads the report the service sent. */
export function migrationReportOf(message: PbMigrationReport | undefined): MigrationReport {
  return new MigrationReport({
    source: message?.source ?? "",
    exporter: message?.exporter ?? "",
    summary: message?.summary ?? "",
    mapped: message?.mapped ?? 0,
    approximated: message?.approximated ?? 0,
    unmapped: message?.unmapped ?? 0,
    skipped: message?.skipped ?? 0,
    entries: (message?.entries ?? []).map((entry) => ({
      id: entry.id,
      kind: entry.kind,
      name: entry.name,
      target: entry.target,
      verdict: entry.verdict,
      note: entry.note,
    })),
    text: message?.text ?? "",
  });
}

/** A SysML v1 model migrated to one of the formats OpenSysML writes. */
export class Migration {
  /** The migrated model. */
  readonly content: string;
  /** The v1 form read, canonically `xmi`: `uml` and `mdzip` name the same reader. */
  readonly fromFormat: string;
  /** Format `content` is written in. */
  readonly toFormat: string;
  /** What became of every element; the summary and the counts come back with every migration. */
  readonly report: MigrationReport;
  /** The JSON index of the result snapshots the v1 tool stored, as `-migration-results` writes it, when asked for; else empty. */
  readonly results: string;
  /** Image files the model's diagrams embed, by the relative path `content` refers to them with. */
  readonly files: ReadonlyMap<string, Uint8Array>;
  /** Always true: the migration is experimental. */
  readonly experimental = true;
  /** What is experimental about it, in the service's own wording. */
  readonly experimentalNotice: string;

  constructor(init: {
    content: string;
    fromFormat: string;
    toFormat: string;
    report: MigrationReport;
    results: string;
    files: ReadonlyMap<string, Uint8Array>;
    experimentalNotice: string;
  }) {
    this.content = init.content;
    this.fromFormat = init.fromFormat;
    this.toFormat = init.toFormat;
    this.report = init.report;
    this.results = init.results;
    this.files = init.files;
    this.experimentalNotice = init.experimentalNotice;
  }

  toString(): string {
    return this.content;
  }

  get length(): number {
    return this.content.length;
  }
}
