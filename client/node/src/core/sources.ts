// The documents of a model parsed from several sources at once. Each is a file
// the service reads, or inline content under a name; both forms also have a
// SourceDocument the `inline`/`file` factories build.

import { create } from "@bufbuild/protobuf";
import type { SourceDocument as PbSourceDocument } from "../generated/sysml_pb.js";
import { SourceDocumentSchema } from "../generated/sysml_pb.js";

/** Languages inline content may be declared in. */
export const LANGUAGES = ["sysml", "kerml"] as const;

/** One document of a model parsed from several. Build with `file` or `inline`. */
export class SourceDocument {
  /** File the service reads; its extension says which notation it is. */
  readonly path: string | undefined;
  /** Inline model source, parsed under `name`. */
  readonly content: string | undefined;
  /** What diagnostics call inline content; empty for a file, named by its path. */
  readonly name: string;
  /** Notation of inline content; `sysml` when unset. */
  readonly language: string | undefined;

  private constructor(init: {
    path?: string;
    content?: string;
    name?: string;
    language?: string;
  }) {
    if (init.name !== undefined && typeof init.name !== "string") {
      throw new TypeError("name must be a string");
    }
    for (const [field, value] of [
      ["path", init.path],
      ["content", init.content],
      ["language", init.language],
    ] as const) {
      if (value !== undefined && typeof value !== "string") {
        throw new TypeError(`${field} must be a string`);
      }
    }
    if ((init.path === undefined) === (init.content === undefined)) {
      throw new RangeError("a SourceDocument is either a file path or inline content, not both");
    }
    if (init.path !== undefined) {
      if (init.path === "") {
        throw new RangeError("a file document needs a path");
      }
      if (init.name !== undefined && init.name !== "") {
        throw new RangeError(
          "a file document is named by its path; name applies to inline content",
        );
      }
      if (init.language !== undefined) {
        throw new RangeError(
          "a file's extension says which language it is; language applies to inline content",
        );
      }
    } else {
      if (init.name === undefined || init.name === "") {
        throw new RangeError(
          "inline content needs a name; diagnostics report it under that name",
        );
      }
      if (init.language !== undefined && !LANGUAGES.includes(init.language as "sysml" | "kerml")) {
        throw new RangeError("language must be 'sysml' or 'kerml'");
      }
    }
    this.path = init.path;
    this.content = init.content;
    this.name = init.name ?? "";
    this.language = init.language;
  }

  /** A file the service reads, named by its path. */
  static file(path: string): SourceDocument {
    return new SourceDocument({ path });
  }

  /** Inline content, reported and indexed under `name`. */
  static inline(
    name: string,
    content: string,
    options: { language?: string } = {},
  ): SourceDocument {
    return new SourceDocument({ content, name, ...(options.language === undefined ? {} : { language: options.language }) });
  }

  /** The name diagnostics report this document under: the path or name. */
  get documentName(): string {
    return this.path ?? this.name;
  }

  /** The `sysml.SourceDocument` message that sends this document. */
  toPb(): PbSourceDocument {
    if (this.path !== undefined) {
      return create(SourceDocumentSchema, { source: { case: "filePath", value: this.path } });
    }
    return create(SourceDocumentSchema, {
      source: { case: "content", value: this.content ?? "" },
      name: this.name,
      language: this.language ?? "",
    });
  }
}

/** What `parseSources` accepts for one document. */
export type Source = string | readonly [name: string, content: string] | SourceDocument;

function isNamedContent(document: unknown): document is readonly [string, string] {
  return (
    Array.isArray(document) &&
    document.length === 2 &&
    typeof document[0] === "string" &&
    typeof document[1] === "string"
  );
}

/**
 * The `SourceDocument` for each of `documents`, validated as Python's
 * `source_documents` does: a sequence of at least one, no two named alike.
 */
export function sourceDocuments(documents: readonly Source[]): SourceDocument[] {
  if (!Array.isArray(documents)) {
    throw new TypeError(
      "parseSources takes a sequence of documents; write one document as [document]",
    );
  }
  const result: SourceDocument[] = [];
  for (const [position, document] of documents.entries()) {
    if (document instanceof SourceDocument) {
      result.push(document);
    } else if (typeof document === "string") {
      result.push(SourceDocument.file(document));
    } else if (isNamedContent(document)) {
      result.push(SourceDocument.inline(document[0], document[1]));
    } else {
      throw new TypeError(
        `document ${position} is of an unexpected form; expected a path, a ` +
          `(name, content) pair or a SourceDocument`,
      );
    }
  }
  if (result.length === 0) {
    throw new RangeError("parseSources needs at least one document");
  }
  const seen = new Set<string>();
  for (const document of result) {
    const name = document.documentName;
    if (seen.has(name)) {
      throw new RangeError(`two documents are named ${JSON.stringify(name)}`);
    }
    seen.add(name);
  }
  return result;
}
