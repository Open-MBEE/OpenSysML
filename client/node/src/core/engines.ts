// The analysis engines the service answers with, and the standing of an answer.
// The standing a verdict or response carries is the `VerdictStanding` of
// values.ts; the helpers here read it.

import type { EngineInfo as PbEngineInfo } from "../generated/sysml_pb.js";
import type { Bound as PbBound } from "../generated/sysml_pb.js";
import { decodeStanding, type VerdictBound, type VerdictStanding } from "./values.js";

/** Strengths, weakest first, as the service spells them. */
export const STRENGTH_NOT_COVERED = "not covered";
export const STRENGTH_OBSERVED = "observed";
export const STRENGTH_WITNESSED = "witnessed";
export const STRENGTH_BOUNDED = "bounded";
export const STRENGTH_PROVED = "proved";

/** Engine selections that name no single engine. */
export const ENGINE_AUTO = "auto";
export const ENGINE_ALL = "all";

/** How far an answer can be trusted: who answered, how strongly, within what. */
export type Standing = VerdictStanding;

/** One limit an engine ran under, and whether the run stopped at it. */
export type EngineBound = VerdictBound;

/** Reads the standing fields a response or verdict carries. */
export function standingOf(pb: { engine: string; strength: string; bounds: PbBound[] }): Standing {
  return decodeStanding(pb);
}

/** Whether the service reported a standing at all. */
export function standingReported(standing: Standing): boolean {
  return standing.strength !== "";
}

/** The bounds the engine stopped at. */
export function standingReached(standing: Standing): EngineBound[] {
  return standing.bounds.filter((bound) => bound.reached);
}

/** One phrase: `observed by run`, `bounded by explore (runs 64 reached)`. */
export function explainStanding(standing: Standing): string {
  if (!standingReported(standing)) {
    return "";
  }
  let line = standing.strength;
  if (standing.engine !== "") {
    line += ` by ${standing.engine}`;
  }
  const reached = standingReached(standing);
  if (reached.length > 0) {
    line += ` (${reached.map((b) => `${b.name} ${b.limit.toString()} reached`).join(", ")})`;
  }
  return line;
}

/** One engine the service registers, as `listEngines` reports it. */
export interface EngineInfo {
  /** The engine's name, the spelling `engine` selects it by. */
  name: string;
  /** The strongest evidence it may claim, one of `STRENGTH_*`. */
  authority: string;
  /** The question kinds it answers. */
  answers: readonly string[];
  /** The bounds it runs under. */
  bounds: readonly string[];
  /** The external process it needs; empty for an in-process engine. */
  process: string;
  /** Where that process was found; empty when it was not. */
  processFound: string;
  /** Whether it can run here. */
  ready: boolean;
  /** Why it cannot, when it cannot. */
  unavailable: string;
  /** `built-in`, `tool` or `engine` (one registered from a manifest). */
  kind: string;
  /** How it is spoken to. */
  protocol: string;
  /** The manifest file an external engine was read from; empty for a built-in one. */
  source: string;
  /** The resolved command of an external engine; empty for a built-in one. */
  command: string;
  /** The version its manifest declares; empty for a built-in one. */
  version: string;
  /** Whether this service runs it; an external engine is listed unserved until `-serve-external-engines`. */
  served: boolean;
}

/** An engine as the wire describes it. */
export function engineInfoOf(pb: PbEngineInfo): EngineInfo {
  return {
    name: pb.name,
    authority: pb.authority,
    answers: [...pb.answers],
    bounds: [...pb.bounds],
    process: pb.process,
    processFound: pb.processFound,
    ready: pb.ready,
    unavailable: pb.unavailable,
    kind: pb.kind,
    protocol: pb.protocol,
    source: pb.source,
    command: pb.command,
    version: pb.version,
    served: pb.served,
  };
}

/** One line naming the engine, its kind, its authority, its questions and its status. */
export function explainEngine(engine: EngineInfo): string {
  let status = engine.ready ? "ready" : `unavailable: ${engine.unavailable}`;
  if (engine.kind === "engine" && !engine.served) {
    status += "; not served by this service";
  }
  const kind =
    engine.kind === "engine" || engine.kind === "tool"
      ? ` (${engine.kind}, ${engine.protocol})`
      : "";
  return `${engine.name}${kind}: ${engine.authority}, answers ${engine.answers.join(", ")}; ${status}`;
}
