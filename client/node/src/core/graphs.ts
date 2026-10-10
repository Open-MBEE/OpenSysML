import type { ExportGraphsResponse } from "../generated/sysml_pb.js";

/**
 * The lowered graph of an action or state machine and of every behavior it
 * performs, in the canonical `graphs:<version>` JSON form an external analysis
 * engine is sent.
 */
export interface Graphs {
  /** The form as canonical JSON, ending in one newline. */
  content: string;
  /** The version of the form, the `version` field of the JSON. */
  version: number;
  /** The qualified name of the behavior as resolved. */
  subject: string;
}

export function graphsOf(response: ExportGraphsResponse): Graphs {
  return { content: response.content, version: response.version, subject: response.subject };
}
