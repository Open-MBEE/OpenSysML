// Deliberate corruptions of a response, used to prove the runner is not
// vacuous: with one applied, the scenarios it touches must fail.

import type { Message } from "@bufbuild/protobuf";
import type {
  EvaluateResponse,
  InstantiateResponse,
  ParseFileResponse,
  ServerInfoResponse,
  SymbolResponse,
} from "../src/generated/sysml_pb.js";
import type { Mutation } from "./runner.js";

/** The corruptions --mutate can apply, and the scenarios each must break. */
export const MUTATIONS: Record<string, Mutation> = {
  /** Drops a capability the suite requires, which the handshake scenario must catch. */
  "hide-capability": (method, response) => {
    if (is<ServerInfoResponse>(response, "sysml.ServerInfoResponse") && method === "GetServerInfo") {
      response.capabilities = response.capabilities.filter((capability) => capability !== "evaluate_subject");
    }
  },
  /** Loses a parse's diagnostics, which the syntax-error scenario must catch. */
  "drop-diagnostics": (_method, response) => {
    if (is<ParseFileResponse>(response, "sysml.ParseFileResponse")) {
      response.diagnostics = [];
    }
  },
  /** Renames a symbol's kind, which every GetSymbol scenario must catch. */
  "blank-symbol-kind": (_method, response) => {
    if (is<SymbolResponse>(response, "sysml.SymbolResponse") && response.symbol !== undefined) {
      response.symbol.kind = "somethingElse";
    }
  },
  /** Shifts an integer result by one, which the arithmetic scenario must catch. */
  "shift-integer": (_method, response) => {
    if (is<EvaluateResponse>(response, "sysml.EvaluateResponse")) {
      const value = response.result;
      if (value?.kind.case === "intValue") {
        value.kind.value += 1n;
      }
    }
  },
  /** Loses what an object holds, which the instantiate scenarios must catch. */
  "drop-feature-values": (_method, response) => {
    if (is<InstantiateResponse>(response, "sysml.InstantiateResponse") && response.instance !== undefined) {
      response.instance.featureValues = {};
    }
  },
};

/** The name of a corruption --mutate accepts. */
export type MutationName = keyof typeof MUTATIONS;

function is<T extends Message>(response: Message, typeName: T["$typeName"]): response is T {
  return response.$typeName === typeName;
}
