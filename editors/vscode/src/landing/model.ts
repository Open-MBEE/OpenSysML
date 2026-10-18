import type {
  Connection,
  DocumentEvent,
  Instance,
  ModelDiagnostic,
  RenderedView,
} from "@openmbee/opensysml/browser";
import { ExecutionError, ParseError } from "@openmbee/opensysml/browser";
import { normalizeRender } from "../protocol";
import type { RenderEdge, RenderNode, RenderResult } from "../protocol";
import type { Scene } from "./scene";

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
  /** The features from the stack down to this part, dotted: `pilot.editors`. Keys `LandingModel.parts`. */
  path: string;
  /** The feature of the part this one is nested in; absent for a project. */
  owner?: string;
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

export interface TraceRecord {
  kind: string; // entry | exit | accept | transition | choice | send | do | guard
  state?: string;
  from?: string;
  to?: string;
  event?: string;
  alternatives?: string[];
  taken?: string;
  text: string;
}

export interface JourneyRun {
  visited: string[];
  trace: TraceRecord[];
  error?: string;
}

export interface DebugStep {
  record: TraceRecord;
  /** The state after this record: the last state entered, or undefined before the first entry. */
  state?: string;
  /** This record's transition, when it is one. */
  edge?: { from: string; to: string };
  /** How many events have been accepted up to and including this record. */
  accepted: number;
  /** An accept record that fired no transition before the next accept or the end of the trace. */
  ignored?: true;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** Flattens a typed instance to the feature values the diagram reads: strings and nested parts. */
export function instanceRecord(instance: Instance): EngineInstance {
  const featureValues: Record<string, EngineFeatureValue> = {};
  for (const [name, feature] of instance.features) {
    if (feature.kind !== "single") {
      continue;
    }
    if (feature.value.kind === "string") {
      featureValues[name] = { value: { stringValue: feature.value.value } };
    } else if (feature.value.kind === "instance") {
      featureValues[name] = { value: { instanceId: String(feature.value.id) } };
    }
  }
  return { id: String(instance.id), typeSymbolId: instance.typeId, featureValues };
}

/** Shapes a typed rendering as the extension's render protocol, which the diagram draws. */
export function renderResult(view: RenderedView): RenderResult {
  const nodes = view.nodes.map((node): RenderNode => ({
    id: node.id,
    kind: node.kind,
    name: node.name,
    type: node.type,
    detail: node.detail,
    ...(node.parent === "" ? {} : { parent: node.parent }),
    ...(node.ports.length === 0
      ? {}
      : {
          ports: node.ports.map((port) => ({
            id: port.id,
            name: port.name,
            ...(port.type === "" ? {} : { type: port.type }),
            ...(port.direction === "" ? {} : { direction: port.direction }),
          })),
        }),
    ...(node.geometry === undefined
      ? {}
      : {
          ...(node.geometry.unplaced ? {} : { x: node.geometry.x, y: node.geometry.y }),
          ...(node.geometry.hasSize ? { width: node.geometry.width, height: node.geometry.height } : {}),
          ...(node.geometry.collapsed ? { collapsed: true } : {}),
        }),
    ...(node.style === undefined ? {} : { style: node.style }),
    ...(node.style?.fill ? { fill: node.style.fill } : {}),
    ...(node.style?.line ? { border: node.style.line } : {}),
  }));
  const edges = view.edges.map((edge): RenderEdge => ({
    from: edge.from,
    to: edge.to,
    label: edge.label,
    kind: edge.kind,
    ...(edge.fromPort === "" ? {} : { fromPort: edge.fromPort }),
    ...(edge.toPort === "" ? {} : { toPort: edge.toPort }),
    ...(edge.route.length === 0 ? {} : { route: [...edge.route] }),
    ...(edge.style === undefined ? {} : { style: edge.style }),
  }));
  const result: Omit<RenderResult, "form" | "artifact" | "version"> = {
    view: view.view,
    kind: view.kind,
    stated: view.stated,
    shown: [...view.shown],
    nodes,
    edges,
    notices: [...view.notices],
    ...(view.columns.length === 0 ? {} : { columns: [...view.columns] }),
    ...(view.rows.length === 0 ? {} : { rows: view.rows.map((row) => ({ cells: [...row.cells] })) }),
    ...(view.canvas === undefined
      ? {}
      : {
          canvas: {
            unit: view.canvas.unit,
            ...(view.canvas.hasSize ? { width: view.canvas.width, height: view.canvas.height } : {}),
          },
        }),
  };
  return result as RenderResult;
}

/** An error stops the drawing; a warning or a note comes back beside the model. */
export function blocks(diagnostic: Diagnostic): boolean {
  return diagnostic.severity === undefined || diagnostic.severity === "error";
}

function diagnosticOf(diagnostic: ModelDiagnostic): Diagnostic {
  return {
    message: diagnostic.message,
    ...(diagnostic.severity === undefined ? {} : { severity: diagnostic.severity }),
    ...(diagnostic.startLine === undefined ? {} : { line: diagnostic.startLine }),
    ...(diagnostic.startColumn === undefined ? {} : { column: diagnostic.startColumn }),
  };
}

/** Shapes a trace event as the debugger's record; absent fields are omitted. */
export function traceRecord(event: DocumentEvent): TraceRecord {
  return {
    kind: event.kind,
    ...(event.state === "" ? {} : { state: event.state }),
    ...(event.fromState === "" ? {} : { from: event.fromState }),
    ...(event.toState === "" ? {} : { to: event.toState }),
    ...(event.event === "" ? {} : { event: event.event }),
    ...(event.alternatives.length === 0 ? {} : { alternatives: [...event.alternatives] }),
    ...(event.taken === "" ? {} : { taken: event.taken }),
    text: event.text,
  };
}

export function landingModel(
  rootSymbol: string,
  hash: string,
  render: RenderResult,
  instances: readonly EngineInstance[],
): LandingModel {
  const rootName = rootSymbol.slice(rootSymbol.lastIndexOf("::") + 2);
  const root = render.nodes.find(
    (node) => node.parent === undefined && (node.name === rootSymbol || node.name === rootName),
  );
  if (!root) {
    throw new Error(`rendered view has no ${rootSymbol} root`);
  }
  const rootInstance = instances.find((instance) => instance.typeSymbolId === rootSymbol);
  const instancesById = new Map<string, EngineInstance>(
    instances.flatMap((instance) => instance.id === undefined ? [] : [[instance.id, instance]]),
  );
  const nodes = render.nodes
    .filter((node) => node.id !== root.id && node.kind !== "attribute")
    .map((node): RenderNode => {
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
  const nodesById = new Map(nodes.map((node) => [node.id, node]));
  // instanceOf follows feature values down from the stack's instance to a node's.
  const instanceOf = (node: RenderNode): EngineInstance | undefined => {
    const parent = node.parent === undefined ? undefined : nodesById.get(node.parent);
    const owner = parent === undefined ? rootInstance : instanceOf(parent);
    const instanceId = owner?.featureValues?.[node.name]?.value?.instanceId;
    return instanceId === undefined ? undefined : instancesById.get(instanceId);
  };
  const pathOf = (node: RenderNode): string => {
    const parent = node.parent === undefined ? undefined : nodesById.get(node.parent);
    return parent === undefined ? node.name : `${pathOf(parent)}.${node.name}`;
  };
  const parts = new Map<string, LandingPart>();
  for (const node of nodes) {
    if (node.kind !== "part") {
      continue;
    }
    const instance = instanceOf(node);
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
      path: pathOf(node),
      symbol: instance?.typeSymbolId ?? node.type,
      attrs,
    };
    const owner = node.parent === undefined ? undefined : nodesById.get(node.parent)?.name;
    if (owner !== undefined) {
      part.owner = owner;
    }
    parts.set(part.path, part);
  }
  return { hash, render: normalized, parts };
}

/** Parses the scene's text on the connection; diagnostics come back instead of a model. */
export async function readModel(connection: Connection, scene: Scene, source: string): Promise<ModelRead> {
  let parsed;
  try {
    parsed = await connection.parseSources([[scene.documentName, source]]);
  } catch (error) {
    if (error instanceof ParseError && error.diagnostics.length > 0) {
      return { diagnostics: error.diagnostics.map(diagnosticOf) };
    }
    throw error;
  }
  const diagnostics = parsed.diagnostics.map(diagnosticOf);
  if (diagnostics.some(blocks)) {
    return { diagnostics };
  }
  const render = renderResult(await connection.renderView(parsed.hash, scene.view, { ports: "minimal", show: scene.show }));
  const tree = await parsed.instantiate(scene.rootSymbol);
  const instances = [tree.root, ...tree.all.filter((instance) => instance.id !== tree.root.id)].map(instanceRecord);
  return { diagnostics, model: landingModel(scene.rootSymbol, parsed.hash, render, instances) };
}

/** Runs the scene's state machine on all its events; the states visited name the diagram's parts. */
export async function journey(
  connection: Connection,
  scene: Scene,
  model: LandingModel,
  seed?: number,
): Promise<string[]> {
  const result = await connection.executeState(model.hash, scene.journey.symbol, {
    events: scene.journey.events,
    ...(seed === undefined ? {} : { schedule: `seed:${seed}` }),
  });
  return result.statesVisited.flatMap((state) => {
    const id = model.parts.get(state)?.id;
    return id === undefined ? [] : [id];
  });
}

/** Runs the scene's state machine on `events` with its trace; a seed picks each free choice. */
export async function runJourney(
  connection: Connection,
  scene: Scene,
  model: LandingModel,
  events: readonly string[],
  seed?: number,
): Promise<JourneyRun> {
  try {
    const result = await connection.executeState(model.hash, scene.journey.symbol, {
      events,
      trace: true,
      ...(seed === undefined ? {} : { schedule: `seed:${seed}` }),
    });
    return { visited: result.statesVisited, trace: result.trace.map(traceRecord) };
  } catch (error) {
    if (error instanceof ExecutionError) {
      return { visited: [], trace: error.trace.map(traceRecord), error: errorMessage(error) };
    }
    throw error;
  }
}

export function debugSteps(trace: readonly TraceRecord[]): DebugStep[] {
  let state: string | undefined;
  let accepted = 0;
  return trace.map((record, index) => {
    if (record.kind === "entry" && record.state !== undefined) {
      state = record.state;
    }
    if (record.kind === "accept") {
      accepted += 1;
    }
    const step: DebugStep = {
      record,
      ...(state === undefined ? {} : { state }),
      ...(record.kind === "transition" && record.from !== undefined && record.to !== undefined
        ? { edge: { from: record.from, to: record.to } }
        : {}),
      accepted,
    };
    if (record.kind === "accept") {
      let fired = false;
      for (let next = index + 1; next < trace.length && trace[next].kind !== "accept"; next += 1) {
        if (trace[next].kind === "transition") {
          fired = true;
          break;
        }
      }
      if (!fired) {
        step.ignored = true;
      }
    }
    return step;
  });
}
