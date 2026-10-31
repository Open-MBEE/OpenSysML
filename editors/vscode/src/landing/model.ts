import type {
  Connection,
  DocumentEvent,
  DocumentValue,
  Instance,
  Magnitude,
  ModelDiagnostic,
  RenderedView,
  SysMLValue,
} from "@openmbee/opensysml/browser";
import { ExecutionError, Model, ParseError, instanceFromJson, renderedViewFromJson } from "@openmbee/opensysml/browser";
import { normalizeRender } from "../protocol";
import type { RenderEdge, RenderNode, RenderResult } from "../protocol";
import type { Scene, SceneAction, SceneClock, SceneJourney, SceneProgram } from "./scene";

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

/** A state of the machine a scene draws: its box, and the name the trace records it under. */
export interface LandingState {
  id: string;
  /** The state's name as the trace records carry it: `prelaunch`; the start pseudostate has none. */
  name: string;
  /** The names from the machine down to this state, dotted: `onSurface.landing`; the name itself at the top. */
  path: string;
  /** What RenderView drew it as: `start`, `state` or `final`. */
  kind: string;
  /** Whether the machine enters this state first, as RenderView details it. */
  initial: boolean;
}

/** The state machine a scene draws: its states by path and, where that is unambiguous, by name. */
export interface LandingMachine {
  /** The state definition drawn: `MissionSequence::MissionPhases`. */
  symbol: string;
  /** Keyed by `LandingState.path`; a nested state is also keyed by its bare name when no other state has it. */
  states: Map<string, LandingState>;
}

/** A node of the action a scene draws: its box, and the name the trace records it under. */
export interface LandingActionNode {
  id: string;
  /** The node's name as the trace records carry it: `awaitGo`. */
  name: string;
  /** What RenderView drew it as: `initial`, `action`, `decision`, `final`. */
  kind: string;
}

/** The action a scene draws: its nodes by name. */
export interface LandingAction {
  /** The action definition drawn: `MissionSequence::LunarMission`. */
  symbol: string;
  nodes: Map<string, LandingActionNode>;
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
  /** The root itself, which the parts compose; it is not drawn as a box, so it is not among `parts`. */
  root: LandingPart;
  /** The state machine the view draws, when it draws one rather than the root's composition. */
  machine?: LandingMachine;
  /** The action the view draws, when it draws one. */
  action?: LandingAction;
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
  kind: string; // entry | exit | accept | transition | choice | send | do | guard | step | wait | decide | assign | complete
  state?: string;
  from?: string;
  to?: string;
  event?: string;
  alternatives?: string[];
  taken?: string;
  /** The simulated instant the record was written at, when the engine timed it. */
  time?: EngineQuantity;
  /** The action token the record is about, as the executor numbers them. */
  token?: string;
  /** The executor step the record was written at. */
  step?: number;
  /** The feature an `assign` record wrote, and the value the engine wrote to it, printed. */
  feature?: string;
  value?: string;
  text: string;
}

/** A value an action run assigned, as its `assign` record reports it: at which node, to which feature. */
export interface Assignment {
  at: string;
  feature: string;
  value: string;
}

export interface JourneyRun {
  visited: string[];
  trace: TraceRecord[];
  /** The simulated instant the clock stopped at; absent when the run failed. */
  finalTime?: number;
  /** An action's outputs as the engine returned them, printed; absent when the run failed. */
  outputs?: Map<string, string>;
  error?: string;
}

export interface DebugStep {
  record: TraceRecord;
  /** The state after this record: the last state entered, or undefined before the first entry. */
  state?: string;
  /** This record's transition, when it is one. */
  edge?: { from: string; to: string };
  /** The values an action run has assigned up to this record, in order. */
  assigned: Assignment[];
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

/** recordTime is the instant a trace record carries, as the page prints it; undefined when untimed. */
function recordTime(time: DocumentValue): EngineQuantity | undefined {
  if (typeof time === "number" || typeof time === "bigint") {
    return { magnitude: time.toString(), unit: "" };
  }
  if (typeof time === "object" && "magnitude" in time && "unit" in time) {
    return { magnitude: magnitudeText(time.magnitude), unit: time.unit };
  }
  return undefined;
}

/** Shapes a trace event as the debugger's record; absent fields are omitted. */
export function traceRecord(event: DocumentEvent): TraceRecord {
  const time = recordTime(event.time);
  return {
    kind: event.kind,
    ...(time === undefined ? {} : { time }),
    ...(event.state === "" ? {} : { state: event.state }),
    ...(event.fromState === "" ? {} : { from: event.fromState }),
    ...(event.toState === "" ? {} : { to: event.toState }),
    ...(event.event === "" ? {} : { event: event.event }),
    ...(event.alternatives.length === 0 ? {} : { alternatives: [...event.alternatives] }),
    ...(event.taken === "" ? {} : { taken: event.taken }),
    ...(event.token === 0n ? {} : { token: String(event.token) }),
    ...(event.step === 0 ? {} : { step: event.step }),
    ...(event.feature === "" ? {} : { feature: event.feature }),
    ...(event.value === undefined ? {} : { value: valueText(event.value) }),
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
  if (render.kind === "state") {
    return machineModel(target, hash, render, instances);
  }
  if (render.kind === "action") {
    return actionModel(target, hash, render, instances);
  }
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

  const partOf = (node: RenderNode, instance: EngineInstance | undefined, path: string, owner: string | undefined): LandingPart =>
    partRecord(node.id, node.name, node.type, instance, path, owner);

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
  return { hash, target, render: normalized, parts, root: { ...partOf(root, rootInstance, rootName, undefined), feature: rootName } };
}

/** partRecord is one object as the page keeps it: the instance's values under the box drawn for it. */
function partRecord(
  id: string,
  feature: string,
  type: string,
  instance: EngineInstance | undefined,
  path: string,
  owner: string | undefined,
): LandingPart {
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
  attrs.label ??= feature;
  return {
    id,
    feature,
    path,
    symbol: instance?.typeSymbolId ?? type,
    type,
    attrs,
    quantities,
    unvalued,
    ...(owner === undefined ? {} : { owner }),
  };
}

/**
 * machineModel is a state view as the page keeps it: the machine's own states lifted out of
 * its box, the states and regions nested in them kept where they are, the transitions kept,
 * and the root the object the machine runs on, as Instantiate materialized it.
 */
function machineModel(
  target: SceneTarget,
  hash: string,
  render: RenderResult,
  instances: readonly EngineInstance[],
): LandingModel {
  const { rootSymbol } = target;
  const rootName = rootSymbol.slice(rootSymbol.lastIndexOf("::") + 2);
  const machines = render.nodes.filter((node) => node.parent === undefined);
  if (machines.length !== 1) {
    throw new Error(`rendered state view draws ${machines.length} machines, want one`);
  }
  const [machine] = machines;
  const states = new Map<string, LandingState>();
  const byName = new Map<string, LandingState[]>();
  const nodes: RenderNode[] = [];
  // keep walks below `parent`: its children lifted when it is the machine, nested ones left in place.
  const keep = (parent: RenderNode, above: string | undefined): void => {
    for (const node of render.nodes) {
      if (node.parent !== parent.id) {
        continue;
      }
      const { parent: _parent, ...lifted } = node;
      nodes.push(parent === machine ? lifted : node);
      let path = above;
      if (node.name !== "") {
        path = above === undefined ? node.name : `${above}.${node.name}`;
      }
      if (node.name !== "" && path !== undefined && node.kind !== "region") {
        const state: LandingState = { id: node.id, name: node.name, path, kind: node.kind, initial: node.detail === "initial" };
        states.set(path, state);
        byName.set(node.name, [...(byName.get(node.name) ?? []), state]);
      }
      keep(node, path);
    }
  };
  keep(machine, undefined);
  for (const [name, named] of byName) {
    if (named.length === 1 && !states.has(name)) {
      states.set(name, named[0]);
    }
  }
  const retained = new Set(nodes.map((node) => node.id));
  const normalized = normalizeRender({
    ...render,
    nodes,
    edges: render.edges.filter((edge) => retained.has(edge.from) && retained.has(edge.to)),
  });
  const rootInstance = instances.find((instance) => instance.typeSymbolId === rootSymbol);
  return {
    hash,
    target,
    render: normalized,
    parts: new Map(),
    root: partRecord(machine.id, rootName, "", rootInstance, rootName, undefined),
    machine: { symbol: machine.name, states },
  };
}

/**
 * actionModel is an action view as the page keeps it: the action's own nodes lifted out of its
 * box and keyed by the name the trace records carry, the successions kept, and the root the
 * object performing the action, as Instantiate materialized it.
 */
function actionModel(
  target: SceneTarget,
  hash: string,
  render: RenderResult,
  instances: readonly EngineInstance[],
): LandingModel {
  const { rootSymbol } = target;
  const rootName = rootSymbol.slice(rootSymbol.lastIndexOf("::") + 2);
  const actions = render.nodes.filter((node) => node.parent === undefined);
  if (actions.length !== 1) {
    throw new Error(`rendered action view draws ${actions.length} actions, want one`);
  }
  const [action] = actions;
  const nodes = new Map<string, LandingActionNode>();
  const kept: RenderNode[] = [];
  for (const node of render.nodes) {
    if (node.parent !== action.id) {
      continue;
    }
    const { parent: _parent, ...lifted } = node;
    kept.push(lifted);
    if (node.name !== "") {
      nodes.set(node.name, { id: node.id, name: node.name, kind: node.kind });
    }
  }
  const retained = new Set(kept.map((node) => node.id));
  const normalized = normalizeRender({
    ...render,
    nodes: kept,
    edges: render.edges.filter((edge) => retained.has(edge.from) && retained.has(edge.to)),
  });
  const rootInstance = instances.find((instance) => instance.typeSymbolId === rootSymbol);
  return {
    hash,
    target,
    render: normalized,
    parts: new Map(),
    root: partRecord(action.id, rootName, "", rootInstance, rootName, undefined),
    action: { symbol: action.name, nodes },
  };
}

/** stateIdOf is the box a state the engine named is drawn as: a machine's state (by path or unambiguous name), an action's node, or the part a journey's state stands for. */
export function stateIdOf(model: LandingModel, state: string): string | undefined {
  return model.machine?.states.get(state)?.id ?? model.action?.nodes.get(state)?.id ?? model.parts.get(state)?.id;
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
  const { rootSymbol } = model.target;
  const explain = async (part: LandingPart, path: string): Promise<LandingPart> => {
    const unvalued = { ...part.unvalued };
    await Promise.all(
      Object.keys(unvalued).map(async (feature) => {
        const value = await parsed.eval(`${path}.${feature}`).catch(() => undefined);
        if (value?.kind === "undetermined" && value.reason !== "") {
          unvalued[feature] = value.reason;
        }
      }),
    );
    return { ...part, unvalued };
  };
  const [root, parts] = await Promise.all([
    explain(model.root, rootSymbol),
    Promise.all([...model.parts].map(async ([path, part]) => [path, await explain(part, `${rootSymbol}.${path}`)] as const)),
  ]);
  return { ...model, root, parts: new Map(parts) };
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
    case "boolean":
      return String(value.value);
    case "string":
      return value.value;
    case "instance":
      return `instance #${value.id}`;
    case "sequence":
      return value.elements.map(valueText).join(", ");
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
    const id = stateIdOf(model, state);
    return id === undefined ? [] : [id];
  });
}

/** clockOf is the clocked run a scene's program is; another program has none. */
export function clockOf(program: SceneProgram): SceneClock {
  if (program.kind !== "clock") {
    throw new Error(`a ${program.kind} program runs no clock`);
  }
  return program;
}

/** Runs the program's machine, performed by its object, for its simulated duration, with the trace. */
export async function runClock(connection: Connection, program: SceneClock, model: LandingModel): Promise<JourneyRun> {
  try {
    const result = await connection.executeState(model.hash, program.symbol, {
      performer: program.performer,
      advance: program.advance,
      trace: true,
    });
    return { visited: result.statesVisited, trace: result.trace.map(traceRecord), finalTime: result.finalTime };
  } catch (error) {
    if (error instanceof ExecutionError) {
      return { visited: [], trace: error.trace.map(traceRecord), error: errorMessage(error) };
    }
    throw error;
  }
}

/** Performs the program's action on its object, posting its events as the run waits, with the trace. */
export async function runAction(connection: Connection, program: SceneAction, model: LandingModel): Promise<JourneyRun> {
  try {
    const result = await connection.executeAction(model.hash, program.symbol, {
      performer: program.performer,
      events: program.events,
      trace: true,
    });
    const outputs = new Map<string, string>();
    for (const [name, value] of result.outputs) {
      outputs.set(name, value instanceof Error ? value.message : valueText(value));
    }
    const trace = result.trace.map(traceRecord);
    return { visited: trace.flatMap((record) => (record.kind === "step" && record.to !== undefined ? [record.to] : [])), trace, outputs };
  } catch (error) {
    if (error instanceof ExecutionError) {
      return { visited: [], trace: error.trace.map(traceRecord), error: errorMessage(error) };
    }
    throw error;
  }
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
  let assigned: Assignment[] = [];
  return trace.map((record, index) => {
    if (record.kind === "entry" && record.state !== undefined) {
      state = record.state;
    }
    // An action's token is at the node a `step` arrived at; a wait, decide or assign names the node it is at.
    if (record.kind === "step" && record.to !== undefined) {
      state = record.to;
    } else if ((record.kind === "wait" || record.kind === "decide" || record.kind === "assign") && record.state !== undefined) {
      state = record.state;
    }
    if (record.kind === "assign" && record.state !== undefined && record.feature !== undefined && record.value !== undefined) {
      assigned = [...assigned, { at: record.state, feature: record.feature, value: record.value }];
    }
    if (record.kind === "accept") {
      accepted += 1;
    }
    const moved = (record.kind === "transition" || record.kind === "step") && record.from !== undefined && record.to !== undefined;
    const step: DebugStep = {
      record,
      ...(state === undefined ? {} : { state }),
      ...(moved ? { edge: { from: record.from!, to: record.to! } } : {}),
      assigned,
      accepted,
    };
    if (record.kind === "accept") {
      let fired = false;
      // An accept fired when a transition or an action step moved on before the next accept.
      for (let next = index + 1; next < trace.length && trace[next].kind !== "accept"; next += 1) {
        if (trace[next].kind === "transition" || trace[next].kind === "step") {
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
