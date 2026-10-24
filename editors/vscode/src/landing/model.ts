import type {
  Connection,
  DocumentEvent,
  Instance,
  Magnitude,
  ModelDiagnostic,
  RenderedView,
  SysMLValue,
} from "@openmbee/opensysml/browser";
import { ExecutionError, Model, ParseError, instanceFromJson, renderedViewFromJson } from "@openmbee/opensysml/browser";
import { normalizeRender } from "../protocol";
import type { RenderEdge, RenderNode, RenderResult } from "../protocol";
import type { Scene, SceneJourney } from "./scene";

export interface Diagnostic {
  message: string;
  severity?: string;
  line?: number;
  column?: number;
}

export interface LandingPart {
  id: string;
  feature: string;
  /** The instance's type, else the type the drawn usage declares. */
  symbol: string;
  /** The type the drawn usage declares, as RenderView names it: `'S-IC'`; empty when it declares none. */
  type: string;
  /** The instance's string-valued features, by name; `label` falls back to the feature name. */
  attrs: Record<string, string>;
  /** The instance's quantities, by feature: `totalMass`. */
  quantities: Record<string, EngineQuantity>;
  /** The features the instantiation could not value, by feature, with the engine's reason. */
  unvalued: Record<string, string>;
  /** The features from the root down to this part, dotted: `pilot.editors`. Keys `LandingModel.parts`. */
  path: string;
  /** The feature of the part this one is nested in; absent for a top-level part. */
  owner?: string;
}

/** What the root is drawn as: its symbol and the view RenderView draws of it. */
export interface SceneTarget {
  rootSymbol: string;
  view: string;
}

export interface LandingModel {
  hash: string;
  /** The root the model was instantiated from and drawn as. */
  target: SceneTarget;
  render: RenderResult;
  parts: Map<string, LandingPart>;
}

export type ModelRead = { diagnostics: Diagnostic[]; model?: LandingModel };

/** A quantity as the diagram keeps it: the engine's magnitude in digits, and its unit. */
export interface EngineQuantity {
  magnitude: string;
  unit: string;
}

export interface EngineValue {
  stringValue?: string;
  instanceId?: string;
  quantity?: EngineQuantity;
}

export interface EngineFeatureValue {
  value?: EngineValue;
  /** The objects a feature lists when it holds several. */
  values?: EngineValue[];
  /** Why the instantiation could not value the feature. */
  error?: string;
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

function magnitudeText(magnitude: Magnitude): string {
  switch (magnitude.kind) {
    case "int":
      return magnitude.value.toString();
    case "real":
      return String(magnitude.value);
    case "rational":
      return `${magnitude.numerator}/${magnitude.denominator}`;
  }
}

function engineValue(value: SysMLValue): EngineValue | undefined {
  switch (value.kind) {
    case "string":
      return { stringValue: value.value };
    case "instance":
      return { instanceId: String(value.id) };
    case "quantity":
      return { quantity: { magnitude: magnitudeText(value.magnitude), unit: value.unit } };
    default:
      return undefined;
  }
}

/**
 * Flattens a typed instance to the feature values the diagram reads: strings, quantities,
 * nested parts singly or listed, and the error of a feature the instantiation could not value.
 */
export function instanceRecord(instance: Instance): EngineInstance {
  const featureValues: Record<string, EngineFeatureValue> = {};
  for (const [name, feature] of instance.features) {
    if (feature.kind === "error") {
      featureValues[name] = { error: feature.error };
      continue;
    }
    if (feature.kind === "many") {
      const values = feature.values.flatMap((value) => {
        const kept = engineValue(value);
        return kept === undefined ? [] : [kept];
      });
      if (values.length > 0) {
        featureValues[name] = { values };
      }
      continue;
    }
    const value = engineValue(feature.value);
    if (value !== undefined) {
      featureValues[name] = { value };
    }
  }
  return { id: String(instance.id), typeSymbolId: instance.typeId, featureValues };
}

/** The model a scene paints before the module loads, read from its committed snapshot. */
export function snapshotModel(scene: Scene): LandingModel {
  return landingModel(
    { rootSymbol: scene.rootSymbol, view: scene.view },
    scene.snapshot.hash,
    renderResult(renderedViewFromJson(scene.snapshot.render)),
    scene.snapshot.instances.map((instance) => instanceRecord(instanceFromJson(instance))),
  );
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

/**
 * landingModel is the diagram as the engine instantiated it: the root's box is dropped and
 * its parts drawn at the top level, each box standing for one object. A feature holding
 * one object draws it once; a feature listing several draws each the no sibling holds
 * singly, as `engines[1]`; a part feature holding no object draws nothing, as do attributes.
 * A copy's subtree takes the copy's id suffix, so two listed objects' parts never alias; a wire
 * declared to a copied feature is left out, as it cannot be attributed to one copy.
 */
export function landingModel(
  target: SceneTarget,
  hash: string,
  render: RenderResult,
  instances: readonly EngineInstance[],
): LandingModel {
  const { rootSymbol } = target;
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
  const childrenOf = new Map<string, RenderNode[]>();
  for (const node of render.nodes) {
    if (node.parent !== undefined) {
      childrenOf.set(node.parent, [...(childrenOf.get(node.parent) ?? []), node]);
    }
  }
  const nodes: RenderNode[] = [];
  const parts = new Map<string, LandingPart>();

  const partOf = (node: RenderNode, instance: EngineInstance | undefined, path: string, owner: string | undefined): LandingPart => {
    const attrs: Record<string, string> = {};
    const quantities: Record<string, EngineQuantity> = {};
    const unvalued: Record<string, string> = {};
    for (const [name, featureValue] of Object.entries(instance?.featureValues ?? {})) {
      if (featureValue.value?.stringValue !== undefined) {
        attrs[name] = featureValue.value.stringValue;
      } else if (featureValue.value?.quantity !== undefined) {
        quantities[name] = featureValue.value.quantity;
      } else if (featureValue.error !== undefined) {
        unvalued[name] = featureValue.error;
      }
    }
    attrs.label ??= node.name;
    return {
      id: node.id,
      feature: node.name,
      path,
      symbol: instance?.typeSymbolId ?? node.type,
      type: node.type,
      attrs,
      quantities,
      unvalued,
      ...(owner === undefined ? {} : { owner }),
    };
  };

  // place draws `drawn`, a copy of `source` standing for `instance`, then the boxes below it;
  // `suffix` is what the copies above it added to their ids, so each copy's subtree has its own.
  const place = (
    source: RenderNode,
    drawn: RenderNode,
    instance: EngineInstance | undefined,
    path: string,
    owner: string | undefined,
    suffix: string,
  ): void => {
    nodes.push(drawn);
    if (drawn.kind === "part") {
      parts.set(path, partOf(drawn, instance, path, owner));
    }
    expand(drawn, instance, childrenOf.get(source.id) ?? [], path, suffix);
  };

  // expand draws the children of `parent`, which stands for `instance`; at the top level
  // the root's parts are lifted out of its box, so they carry no parent.
  const expand = (
    parent: RenderNode | undefined,
    instance: EngineInstance | undefined,
    children: readonly RenderNode[],
    path: string | undefined,
    suffix: string,
  ): void => {
    const copy = (child: RenderNode, id: string, name: string): RenderNode => {
      const { parent: _parent, ...lifted } = child;
      return { ...lifted, id, name, ...(parent === undefined ? {} : { parent: parent.id }) };
    };
    const pathTo = (name: string): string => (path === undefined ? name : `${path}.${name}`);
    const owner = parent?.name;
    const single = (child: RenderNode): string | undefined => instance?.featureValues?.[child.name]?.value?.instanceId;
    const boxed = new Set<string>();
    for (const child of children) {
      if (child.kind === "attribute") {
        continue;
      }
      if (child.kind !== "part") {
        place(child, copy(child, child.id + suffix, child.name), undefined, pathTo(child.name), owner, suffix);
        continue;
      }
      const id = single(child);
      if (id !== undefined) {
        boxed.add(id);
        place(child, copy(child, child.id + suffix, child.name), instancesById.get(id), pathTo(child.name), owner, suffix);
      }
    }
    for (const child of children) {
      if (child.kind !== "part" || single(child) !== undefined) {
        continue;
      }
      let index = 0;
      for (const value of instance?.featureValues?.[child.name]?.values ?? []) {
        if (value.instanceId === undefined || boxed.has(value.instanceId)) {
          continue;
        }
        boxed.add(value.instanceId);
        index += 1;
        const name = `${child.name}[${index}]`;
        const below = `${suffix}[${index}]`;
        place(child, copy(child, child.id + below, name), instancesById.get(value.instanceId), pathTo(name), owner, below);
      }
    }
  };

  expand(undefined, rootInstance, childrenOf.get(root.id) ?? [], undefined, "");
  // Anything drawn beside the root stands for no object of its own and is kept as drawn.
  for (const node of render.nodes) {
    if (node.parent === undefined && node.id !== root.id && node.kind !== "attribute") {
      place(node, { ...node }, undefined, node.name, undefined, "");
    }
  }
  const retained = new Set(nodes.map((node) => node.id));
  const normalized = normalizeRender({
    ...render,
    nodes,
    edges: render.edges.filter((edge) => retained.has(edge.from) && retained.has(edge.to)),
  });
  return { hash, target, render: normalized, parts };
}

/** sceneTarget is the root a scene draws by default, before any variant is switched on. */
export function sceneTarget(scene: Scene): SceneTarget {
  return { rootSymbol: scene.rootSymbol, view: scene.view };
}

/**
 * explained asks the engine why each feature the instantiation could not value is so,
 * by evaluating it on its path; an undetermined answer's reason replaces the error text.
 */
async function explained(parsed: Model, model: LandingModel): Promise<LandingModel> {
  const parts = new Map(model.parts);
  await Promise.all(
    [...model.parts].map(async ([path, part]) => {
      const unvalued = { ...part.unvalued };
      await Promise.all(
        Object.keys(unvalued).map(async (feature) => {
          const value = await parsed.eval(`${model.target.rootSymbol}.${path}.${feature}`).catch(() => undefined);
          if (value?.kind === "undetermined" && value.reason !== "") {
            unvalued[feature] = value.reason;
          }
        }),
      );
      parts.set(path, { ...part, unvalued });
    }),
  );
  return { ...model, parts };
}

/** Parses the scene's text on the connection; diagnostics come back instead of a model. */
export async function readModel(
  connection: Connection,
  scene: Scene,
  source: string,
  target: SceneTarget = sceneTarget(scene),
): Promise<ModelRead> {
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
  const render = renderResult(
    await connection.renderView(parsed.hash, target.view, {
      ports: "minimal",
      show: scene.show,
      ...(scene.depth === undefined ? {} : { depth: scene.depth }),
    }),
  );
  const tree = await parsed.instantiate(target.rootSymbol);
  const instances = [tree.root, ...tree.all.filter((instance) => instance.id !== tree.root.id)].map(instanceRecord);
  return { diagnostics, model: await explained(parsed, landingModel(target, parsed.hash, render, instances)) };
}

/** valueText is a value as the status line prints it: a quantity's digits and unit, or why it is undetermined. */
export function valueText(value: SysMLValue): string {
  switch (value.kind) {
    case "quantity":
      return `${magnitudeText(value.magnitude)} ${value.unit}`;
    case "int":
    case "real":
      return String(value.value);
    case "rational":
      return `${value.numerator}/${value.denominator}`;
    case "undetermined":
      return value.reason === "" ? "undetermined" : `undetermined (${value.reason})`;
    case "unset":
      return "unset";
    case "null":
      return "null";
    default:
      return value.kind;
  }
}

/** Evaluates the root's rolled-up feature on the engine: `MassRollup::saturnV.totalMass`. */
export function rollup(connection: Connection, model: LandingModel, feature: string): Promise<SysMLValue> {
  return Model.adopt(connection, model.hash).eval(`${model.target.rootSymbol}.${feature}`);
}

/** rollupLevels is the diagram's boxes by depth, the deepest first: the order a roll-up is animated in. */
export function rollupLevels(model: LandingModel): string[][] {
  const byDepth = new Map<number, string[]>();
  for (const part of model.parts.values()) {
    const depth = part.path.split(".").length;
    byDepth.set(depth, [...(byDepth.get(depth) ?? []), part.id]);
  }
  return [...byDepth.entries()].sort(([a], [b]) => b - a).map(([, ids]) => ids);
}

/** journeyOf is the state machine a scene runs; a scene with another program has none to run. */
export function journeyOf(scene: Scene): SceneJourney {
  if (scene.program.kind !== "journey") {
    throw new Error(`the ${scene.id} scene runs no state machine`);
  }
  return scene.program;
}

/** Runs the scene's state machine on all its events; the states visited name the diagram's parts. */
export async function journey(
  connection: Connection,
  scene: Scene,
  model: LandingModel,
  seed?: number,
): Promise<string[]> {
  const program = journeyOf(scene);
  const result = await connection.executeState(model.hash, program.symbol, {
    events: program.events,
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
    const result = await connection.executeState(model.hash, journeyOf(scene).symbol, {
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
