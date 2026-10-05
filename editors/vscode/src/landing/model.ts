import { normalizeRender } from "../protocol";
import type { RenderResult } from "../protocol";

export interface EngineClient {
  call(method: string, params: string): string;
}

export interface Diagnostic {
  message: string;
  severity?: string;
  line?: number;
  column?: number;
}

export interface LandingPart {
  id: string;
  feature: string;
  symbol: string;
  attrs: Record<string, string>;
}

export interface LandingModel {
  hash: string;
  render: RenderResult;
  parts: Map<string, LandingPart>;
}

export type ModelRead = { diagnostics: Diagnostic[]; model?: LandingModel };

export interface EngineValue {
  stringValue?: string;
  instanceId?: string;
}

export interface EngineFeatureValue {
  value?: EngineValue;
}

export interface EngineInstance {
  id?: string;
  typeSymbolId?: string;
  featureValues?: Record<string, EngineFeatureValue>;
}

export const STACK_SYMBOL = "OpenSysMLStack::stack";
export const JOURNEY_SYMBOL = "OpenSysMLStack::ModelJourney";
export const JOURNEY_EVENTS = ["Commit", "Pull", "Push", "Check"] as const;

type EngineRender = Omit<RenderResult, "form" | "artifact" | "version">;

interface EngineDiagnostic extends Diagnostic {
  span?: {
    startLine?: number;
    startCol?: number;
  };
}

interface ParseSourcesResult {
  modelHash?: string;
  diagnostics?: EngineDiagnostic[];
}

interface InstantiateResult {
  instances?: EngineInstance[];
}

interface ExecuteStateResult {
  statesVisited?: string[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function errorMessage(error: unknown): string {
  if (typeof error === "string") {
    return error;
  }
  if (isRecord(error) && typeof error.message === "string") {
    return error.message;
  }
  return JSON.stringify(error) ?? String(error);
}

export function rpc<T>(engine: EngineClient, method: string, params: object): T {
  const envelope: unknown = JSON.parse(engine.call(method, JSON.stringify(params)));
  if (!isRecord(envelope)) {
    throw new Error(`${method} returned an invalid response`);
  }
  if (envelope.error !== undefined && envelope.error !== null) {
    throw new Error(errorMessage(envelope.error));
  }
  if (!("result" in envelope)) {
    throw new Error(`${method} returned no result`);
  }
  const result = envelope.result;
  if (isRecord(result) && result.error !== undefined && result.error !== null) {
    throw new Error(errorMessage(result.error));
  }
  return result as T;
}

export function landingModel(hash: string, render: RenderResult, instances: EngineInstance[]): LandingModel {
  const rootName = STACK_SYMBOL.slice(STACK_SYMBOL.lastIndexOf("::") + 2);
  const root = render.nodes.find(
    (node) => node.parent === undefined && (node.name === STACK_SYMBOL || node.name === rootName),
  );
  if (!root) {
    throw new Error(`rendered view has no ${STACK_SYMBOL} root`);
  }
  const rootInstance = instances.find((instance) => instance.typeSymbolId === STACK_SYMBOL);
  const instancesById = new Map<string, EngineInstance>(
    instances.flatMap((instance) => instance.id === undefined ? [] : [[instance.id, instance]]),
  );
  const nodes = render.nodes
    .filter((node) => node.id !== root.id && node.kind !== "attribute")
    .map((node) => {
      if (node.parent !== root.id) {
        return node;
      }
      const { parent: _parent, ...lifted } = node;
      return lifted;
    });
  const retained = new Set(nodes.map((node) => node.id));
  const normalized = normalizeRender({
    ...render,
    nodes,
    edges: render.edges.filter((edge) => retained.has(edge.from) && retained.has(edge.to)),
  });
  const parts = new Map<string, LandingPart>();
  for (const node of nodes) {
    if (node.kind !== "part") {
      continue;
    }
    const instanceId = rootInstance?.featureValues?.[node.name]?.value?.instanceId;
    const instance = instanceId === undefined ? undefined : instancesById.get(instanceId);
    const attrs: Record<string, string> = {};
    for (const [name, featureValue] of Object.entries(instance?.featureValues ?? {})) {
      const value = featureValue.value?.stringValue;
      if (value !== undefined) {
        attrs[name] = value;
      }
    }
    attrs.label ??= node.name;
    const part: LandingPart = {
      id: node.id,
      feature: node.name,
      symbol: instance?.typeSymbolId ?? node.type,
      attrs,
    };
    parts.set(part.feature, part);
  }
  return { hash, render: normalized, parts };
}

export function readModel(engine: EngineClient, source: string): ModelRead {
  const parsed = rpc<ParseSourcesResult>(engine, "ParseSources", {
    documents: [{ name: "opensysml-stack.sysml", content: source }],
  });
  const diagnostics = (parsed.diagnostics ?? []).map((diagnostic) => ({
    message: diagnostic.message,
    ...(diagnostic.severity === undefined ? {} : { severity: diagnostic.severity }),
    ...(diagnostic.span?.startLine === undefined ? {} : { line: diagnostic.span.startLine }),
    ...(diagnostic.span?.startCol === undefined ? {} : { column: diagnostic.span.startCol }),
  }));
  if (diagnostics.length > 0) {
    return { diagnostics };
  }
  if (!parsed.modelHash) {
    throw new Error("ParseSources returned no modelHash");
  }
  const render = rpc<EngineRender>(engine, "RenderView", {
    modelHash: parsed.modelHash,
    view: `#interconnection:${STACK_SYMBOL}`,
    ports: "minimal",
  });
  const built = rpc<InstantiateResult>(engine, "Instantiate", {
    modelHash: parsed.modelHash,
    symbolId: STACK_SYMBOL,
  });
  return {
    diagnostics,
    model: landingModel(parsed.modelHash, render as RenderResult, built.instances ?? []),
  };
}

export function journey(engine: EngineClient, model: LandingModel): string[] {
  const result = rpc<ExecuteStateResult>(engine, "ExecuteState", {
    modelHash: model.hash,
    stateMachineSymbolId: JOURNEY_SYMBOL,
    events: JOURNEY_EVENTS,
  });
  const idsByFeature = new Map([...model.parts.values()].map((part) => [part.feature, part.id]));
  return (result.statesVisited ?? []).flatMap((state) => {
    const id = idsByFeature.get(state);
    return id === undefined ? [] : [id];
  });
}
