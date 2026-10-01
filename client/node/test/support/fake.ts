// A Transport standing in for a service: a canned handshake, and answers the
// test supplies per method. Lets capability gates and decoders run without a
// process to spawn.

import type { Transport } from "@connectrpc/connect";
import { Connection } from "../../src/core/connection.js";

/** The ServerInfo the fake service reports. */
export interface FakeInfo {
  version: string;
  capabilities: string[];
}

export const ALL_CAPABILITIES = [
  "parse_sources",
  "inline_language",
  "strict_conformance",
  "convert",
  "migrate",
  "query",
  "document_query",
  "render_document",
  "render_document_html",
  "verification",
  "verification_questions",
  "engines",
  "schedule",
  "schedule_explore",
  "performer",
  "feature_values",
  "complex_values",
  "structured_values",
  "measurement_refs",
  "function_values",
  "metaobject_values",
  "infinity_value",
  "set_values",
  "tensor_values",
  "enum_values",
  "unset_value",
  "undetermined_value",
  "apply_edits",
  "edit_documents",
  "authoring",
  "connection_authoring",
  "satisfy_authoring",
  "requirement_constraint_authoring",
  "transition_authoring",
  "verification_objective_authoring",
  "metadata_authoring",
  "metadata_prefix_authoring",
  "sequence_authoring",
  "action_body_statement_authoring",
  "import_authoring",
  "documentation_authoring",
  "comment_authoring",
  "member_modifiers",
  "implicit_parameters",
  "constraint_body_authoring",
  "state_action_authoring",
];

/**
 * A transport whose `unary` answers GetServerInfo with `info` and every other
 * method with whatever `answer` returns (or throws). Messages pass through
 * untouched: decoders under test read the same wire objects the service sends.
 */
export function fakeTransport(
  info: FakeInfo,
  answer: (method: string, input: unknown) => unknown,
): Transport {
  return {
    unary(method: { name: string }, _signal: unknown, _timeoutMs: unknown, _header: unknown, input: unknown) {
      const message =
        method.name === "GetServerInfo"
          ? { version: info.version, capabilities: info.capabilities }
          : answer(method.name, input);
      return Promise.resolve({
        stream: false,
        method,
        header: new Headers(),
        message,
        trailer: new Headers(),
      });
    },
    stream() {
      return Promise.reject(new Error("the fake transport serves no streaming calls"));
    },
  } as unknown as Transport;
}

/** A connection over `fakeTransport`, for gate and decoder tests. */
export async function fakeConnection(
  capabilities: readonly string[],
  answer: (method: string, input: unknown) => unknown = () => {
    throw new Error("the fake service answers nothing for this method");
  },
): Promise<Connection> {
  return Connection.open({
    transport: fakeTransport({ version: "test", capabilities: [...capabilities] }, answer),
    encoding: "protobuf",
    backend: {
      origin: "the fake transport",
      release: () => Promise.resolve(),
    },
  });
}
