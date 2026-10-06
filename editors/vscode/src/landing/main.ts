// The landing page's hero diagram: the stack model as the engine renders it, laid out
// by ELK, its wires rerouted by libavoid around the boxes a visitor moves, drawn by
// the same canvas code as the VS Code diagram panel.
import type { LayoutGeometry, RenderPoint, RenderResult } from "../protocol";
import { autoLayout, type AutoLayout } from "../webview/autolayout";
import { CLEARANCE, loadAvoid, portExitReach } from "../webview/avoid";
import { cssEscape, drawCanvas } from "../webview/canvas";
import {
  alignedPlacement,
  clampNodeToBounds,
  freePlacement,
  keepOrthogonalRoutes,
  layoutCanvas,
  type Box,
  type CanvasLayout,
  type Overrides,
  type PlacedNode,
  type PlacedPort,
} from "../webview/layout";
import {
  JOURNEY_EVENTS,
  debugSteps,
  journey,
  landingModel,
  readModel,
  runJourney,
  type DebugStep,
  type Diagnostic,
  type EngineClient,
  type EngineInstance,
  type JourneyRun,
  type LandingModel,
  type LandingPart,
} from "./model";
import { presented } from "./present";
import stack from "./stack.json";

const SVG_NS = "http://www.w3.org/2000/svg";
/** How far a box stays inside the hero's edges. */
const HERO_PAD = 8;
/** How far a pointer travels before a press becomes a drag. */
const DRAG_SLOP = 4;
const KEY_STEP = 10;
const LONG_PRESS = 550;
const EDIT_DELAY = 300;
const MAX_SCALE = 1.6;
const HOP = 1000;

function insetBox(box: Box, distance: number): Box {
  return {
    x: box.x + distance,
    y: box.y + distance,
    width: Math.max(0, box.width - 2 * distance),
    height: Math.max(0, box.height - 2 * distance),
  };
}

interface Mounted {
  dispose(): void;
}

interface EngineUrls {
  wasmExec: string;
  engine: string;
}

declare global {
  interface Window {
    osmlLoadEngine?(urls: EngineUrls): Promise<EngineClient>;
    osmlMountDiagram?(root: HTMLElement): void;
    __osmlDiagram?: Mounted;
    document$?: { subscribe(next: () => void): { unsubscribe(): void } };
  }
}

interface Gesture {
  id: string;
  pointer: number;
  start: RenderPoint;
  from: RenderPoint;
  at: RenderPoint;
  moved: boolean;
  longPressed: boolean;
  frame?: number;
  timer?: ReturnType<typeof setTimeout>;
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function diagnosticText(diagnostic: Diagnostic): string {
  const at = diagnostic.line === undefined ? "" : `line ${diagnostic.line}:${diagnostic.column ?? 1}: `;
  return `${at}${diagnostic.message || "error"}`;
}

function webHref(value: string | undefined): string | undefined {
  if (!value) {
    return undefined;
  }
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:" ? url.href : undefined;
  } catch {
    return undefined;
  }
}

function overlap(a: DOMRect | Box, b: DOMRect | Box): number {
  const ax = "left" in a ? a.left : a.x;
  const ay = "top" in a ? a.top : a.y;
  const bx = "left" in b ? b.left : b.x;
  const by = "top" in b ? b.top : b.y;
  const w = Math.min(ax + a.width, bx + b.width) - Math.max(ax, bx);
  const h = Math.min(ay + a.height, by + b.height) - Math.max(ay, by);
  return w > 0 && h > 0 ? w * h : 0;
}

function mount(root: HTMLElement): Mounted {
  window.__osmlDiagram?.dispose();
  const hero = root.closest<HTMLElement>(".osml-hero") ?? root;
  const stage = root.querySelector<HTMLElement>("[data-osml-stage]")!;
  const card = hero.querySelector<HTMLElement>("[data-osml-card]")!;
  const statusEl = root.querySelector<HTMLElement>("[data-osml-status]")!;
  const runBtn = root.querySelector<HTMLButtonElement>("[data-osml-run]")!;
  const srcBtn = root.querySelector<HTMLButtonElement>("[data-osml-source]")!;
  const editor = root.querySelector<HTMLElement>("[data-osml-editor]")!;
  const srcEl = root.querySelector<HTMLTextAreaElement>("[data-osml-src]")!;
  const resetBtn = root.querySelector<HTMLButtonElement>("[data-osml-reset]")!;
  const debugBtn = root.querySelector<HTMLButtonElement>("[data-osml-debug]")!;
  const debugPanel = root.querySelector<HTMLElement>("[data-osml-debugger]")!;
  const sendBtns = [...debugPanel.querySelectorAll<HTMLButtonElement>("[data-osml-send]")];
  const debugResetBtn = debugPanel.querySelector<HTMLButtonElement>("[data-osml-debug-reset]")!;
  const backBtn = debugPanel.querySelector<HTMLButtonElement>("[data-osml-step-back]")!;
  const playBtn = debugPanel.querySelector<HTMLButtonElement>("[data-osml-play]")!;
  const stepBtn = debugPanel.querySelector<HTMLButtonElement>("[data-osml-step]")!;
  const speedSel = debugPanel.querySelector<HTMLSelectElement>("[data-osml-speed]")!;
  const seedEl = debugPanel.querySelector<HTMLInputElement>("[data-osml-seed]")!;
  const reseedBtn = debugPanel.querySelector<HTMLButtonElement>("[data-osml-reseed]")!;
  const nowEl = debugPanel.querySelector<HTMLElement>("[data-osml-debug-now]")!;
  const queueEl = debugPanel.querySelector<HTMLElement>("[data-osml-debug-queue]")!;
  const traceEl = debugPanel.querySelector<HTMLOListElement>("[data-osml-debug-trace]")!;
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const ac = new AbortController();
  const { signal } = ac;
  const on = <K extends keyof HTMLElementEventMap>(
    target: HTMLElement | Document | Window | SVGElement,
    type: K,
    listener: (event: HTMLElementEventMap[K]) => void,
  ): void => target.addEventListener(type, listener as EventListener, { signal });

  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("class", "osml-diagram");
  svg.setAttribute("role", "group");
  svg.setAttribute("aria-label", "The OpenSysML stack, drawn from its SysML model");
  const content = document.createElementNS(SVG_NS, "g");
  svg.append(content);
  hero.append(svg);

  let model = landingModel(stack.hash, stack.render as unknown as RenderResult, stack.instances as unknown as EngineInstance[]);
  let result = presented(model);
  let live = false;
  let goodSource: string | undefined;
  let auto: AutoLayout | undefined;
  let view = { x: 0, y: 0, scale: 1 };
  let layout = layoutCanvas(result, { bounds: bounds() });
  let generation = 0;
  let laidOut = false;
  // Where the visitor has put a box, by part name, so a placement survives an edit.
  const placed = new Map<string, RenderPoint>();
  let gesture: Gesture | undefined;
  let cardFor: string | undefined;
  let hot: string | undefined;
  let liveNode: string | undefined;
  let liveEdge: number | undefined;
  let token: SVGCircleElement | undefined;
  let running = false;
  let editTimer: ReturnType<typeof setTimeout> | undefined;
  let editSequence = 0;
  let enginePromise: Promise<EngineClient> | undefined;
  let sourcePromise: Promise<string> | undefined;

  const partOf = (id: string): LandingPart | undefined => [...model.parts.values()].find((part) => part.id === id);
  const idOf = (feature: string): string | undefined => model.parts.get(feature)?.id;

  function status(text: string, error = false): void {
    statusEl.textContent = text;
    statusEl.classList.toggle("osml-schematic__status--err", error);
  }

  function overrides(): Overrides {
    const nodes = new Map<string, LayoutGeometry>();
    for (const [feature, at] of placed) {
      const id = idOf(feature);
      if (id) {
        nodes.set(id, { x: at.x, y: at.y });
      }
    }
    return { nodes, bounds: bounds() };
  }

  // fit centres the unmoved diagram in the stage, scaled to the stage's width up to MAX_SCALE.
  function fit(): void {
    const home = layoutCanvas(result, { bounds: bounds() }, auto);
    const heroRect = hero.getBoundingClientRect();
    const stageRect = stage.getBoundingClientRect();
    const scale = Math.min(MAX_SCALE, stageRect.width / home.width);
    const height = Math.ceil(home.height * scale);
    if (stage.style.height !== `${height}px`) {
      stage.style.height = `${height}px`;
    }
    view = {
      scale,
      x: stageRect.left - heroRect.left + (stageRect.width - home.width * scale) / 2 - home.origin.x * scale,
      y: stage.getBoundingClientRect().top - heroRect.top - home.origin.y * scale,
    };
  }

  // bounds is the hero in layout coordinates, inset by HERO_PAD.
  function bounds(): Box {
    const pad = HERO_PAD / view.scale;
    return {
      x: -view.x / view.scale + pad,
      y: -view.y / view.scale + pad,
      width: hero.clientWidth / view.scale - 2 * pad,
      height: hero.clientHeight / view.scale - 2 * pad,
    };
  }

  function placementBounds(): Box {
    return insetBox(bounds(), CLEARANCE);
  }

  function clamped(node: PlacedNode, at: RenderPoint): RenderPoint {
    return clampNodeToBounds(node, at, placementBounds(), (port) => exitReach(node, port));
  }

  function exitReach(node: PlacedNode, port: PlacedPort): number {
    const sharing = layout.edges.filter(
      ({ edge, hidden }) =>
        !hidden &&
        ((edge.from === node.node.id && edge.fromPort === port.port.id) ||
          (edge.to === node.node.id && edge.toPort === port.port.id)),
    ).length;
    return portExitReach(sharing);
  }

  function otherNodes(id: string): PlacedNode[] {
    return [...layout.nodes.values()].filter((entry) => entry.node.id !== id && !entry.hidden);
  }

  // keepInHero keeps moved boxes clear while re-clamping in model order.
  function keepInHero(): void {
    let changed = false;
    const settled = new Map<string, PlacedNode>();
    for (const entry of layout.nodes.values()) {
      if (!entry.hidden) {
        settled.set(entry.node.id, entry);
      }
    }
    for (const part of model.parts.values()) {
      const entry = layout.nodes.get(part.id);
      if (!entry || entry.hidden) {
        continue;
      }
      const moved = placed.has(part.feature);
      const bounded = clamped(entry, placed.get(part.feature) ?? entry.box);
      const at =
        (moved &&
          freePlacement(
            entry,
            bounded,
            [...settled.values()].filter((other) => other.node.id !== entry.node.id),
            placementBounds(),
            exitReach,
          )) ||
        bounded;
      if (at.x !== entry.box.x || at.y !== entry.box.y) {
        placed.set(part.feature, at);
        changed = true;
      }
      settled.set(entry.node.id, { ...entry, box: { ...entry.box, ...at } });
    }
    if (changed) {
      layout = layoutCanvas(result, overrides(), auto);
    }
  }

  function groupOf(id: string): SVGGElement | null {
    return content.querySelector<SVGGElement>(`g.opensysml-node[data-opensysml-id="${cssEscape(id)}"]`);
  }

  function edgesAt(id: string): number[] {
    return result.edges.flatMap((edge, index) => (edge.from === id || edge.to === id ? [index] : []));
  }

  function syncClasses(): void {
    const lit = cardFor ?? hot;
    const litEdges = new Set(lit ? edgesAt(lit) : []);
    hero.classList.toggle("osml-hero--focus", lit !== undefined);
    for (const group of content.querySelectorAll<SVGGElement>("g.opensysml-node")) {
      const id = group.dataset.opensysmlId;
      group.classList.toggle("is-hot", id === lit);
      group.classList.toggle("is-live", id === liveNode);
    }
    for (const group of content.querySelectorAll<SVGGElement>("g.opensysml-edge")) {
      const index = Number(group.dataset.edge);
      group.classList.toggle("is-hot", litEdges.has(index));
      group.classList.toggle("is-live", index === liveEdge);
    }
  }

  function decorate(): void {
    for (const group of content.querySelectorAll<SVGGElement>("g.opensysml-node")) {
      const part = partOf(group.dataset.opensysmlId ?? "");
      if (!part) {
        continue;
      }
      group.classList.add("osml-part");
      group.setAttribute("tabindex", "0");
      group.setAttribute("role", "button");
      group.setAttribute(
        "aria-label",
        `${part.attrs.label}, ${part.attrs.kind ?? "part"}. Enter opens the project; ` +
          "Shift+F10 shows its model; arrow keys move it.",
      );
    }
  }

  function draw(shown: CanvasLayout): void {
    const focused = (document.activeElement as Element | null)?.closest?.("g.opensysml-node");
    const focusedId = focused && content.contains(focused) ? (focused as SVGGElement).dataset.opensysmlId : undefined;
    const drawn = drawCanvas(shown);
    content.replaceChildren(...Array.from(drawn.childNodes));
    content.setAttribute("transform", `translate(${view.x} ${view.y}) scale(${view.scale})`);
    decorate();
    if (token) {
      content.append(token);
    }
    syncClasses();
    if (focusedId) {
      groupOf(focusedId)?.focus({ preventScroll: true });
    }
    placeCard();
  }

  function settle(): void {
    if (!laidOut) {
      return;
    }
    fit();
    layout = layoutCanvas(result, overrides(), auto);
    keepInHero();
    draw(layout);
  }

  function relayOut(): void {
    const mine = ++generation;
    const shown = result;
    void autoLayout(shown)
      .catch((error: unknown) => {
        console.warn("ELK could not lay out the landing diagram", error);
        return undefined;
      })
      .then((laid) => {
        if (mine !== generation || signal.aborted) {
          return;
        }
        auto = laid;
        laidOut = true;
        root.classList.add("osml-schematic--drawn");
        settle();
      });
  }

  // ---- the engine and the model text ----

  function engine(): Promise<EngineClient> {
    if (!window.osmlLoadEngine) {
      return Promise.reject(new Error("the engine loader is missing"));
    }
    enginePromise ??= window.osmlLoadEngine({
      wasmExec: root.dataset.osmlWasmExec ?? "",
      engine: root.dataset.osmlEngine ?? "",
    }).catch((error: unknown) => {
      enginePromise = undefined;
      throw error;
    });
    return enginePromise;
  }

  function publishedSource(): Promise<string> {
    sourcePromise ??= fetch(root.dataset.osmlModel ?? "")
      .then((response) => {
        if (!response.ok) {
          throw new Error(`the model did not download (HTTP ${response.status})`);
        }
        return response.text();
      })
      .catch((error: unknown) => {
        sourcePromise = undefined;
        throw error;
      });
    return sourcePromise;
  }

  function currentSource(): Promise<string> {
    return !srcEl.readOnly && srcEl.value ? Promise.resolve(srcEl.value) : publishedSource();
  }

  function adopt(next: LandingModel, source: string): void {
    model = next;
    goodSource = source;
    live = true;
    for (const feature of [...placed.keys()]) {
      if (!model.parts.has(feature)) {
        placed.delete(feature);
      }
    }
    result = presented(model);
    if (cardFor && !partOf(cardFor)) {
      closeCard();
    }
    relayOut();
    if (cardFor) {
      fillCard(cardFor);
    }
    // A debugger run belongs to the model it ran; a new model runs afresh.
    if (debugPanel.hidden) {
      run = undefined;
    } else if (rerunning === undefined) {
      void rerun(-1);
    } else {
      rerunStale = true;
    }
  }

  // read parses the source on the engine; it reports diagnostics and keeps the last good model.
  async function read(source: string, verb: string): Promise<LandingModel | undefined> {
    const started = performance.now();
    const outcome = readModel(await engine(), source);
    if (outcome.diagnostics.length > 0 || !outcome.model) {
      const first = outcome.diagnostics[0];
      srcEl.setAttribute("aria-invalid", "true");
      status(`Not ${verb}: ${first ? diagnosticText(first) : "the model has nothing to draw"}. The diagram shows the last model that read cleanly.`, true);
      return undefined;
    }
    srcEl.removeAttribute("aria-invalid");
    if (source !== goodSource) {
      adopt(outcome.model, source);
    }
    const ms = Math.max(1, Math.round(performance.now() - started));
    status(`The engine read the model in ${ms} ms: ${outcome.model.parts.size} parts, ${outcome.model.render.edges.length} interfaces.`);
    return outcome.model;
  }

  function scheduleEdit(): void {
    const mine = ++editSequence;
    clearTimeout(editTimer);
    editTimer = setTimeout(() => {
      editTimer = undefined;
      if (running || rerunning !== undefined) {
        scheduleEdit();
        return;
      }
      const source = srcEl.value;
      void read(source, "redrawn").catch((error: unknown) => {
        if (mine === editSequence) {
          status(`Not redrawn: ${message(error)}. The diagram shows the last model that read cleanly.`, true);
        }
      });
    }, EDIT_DELAY);
  }

  // ---- the card ----

  function row(into: HTMLElement, key: string, value: string): void {
    const k = document.createElement("div");
    k.className = "osml-nodecard__k";
    k.textContent = key;
    const v = document.createElement("div");
    v.className = "osml-nodecard__v";
    v.textContent = value;
    into.append(k, v);
  }

  function fillCard(id: string): void {
    const part = partOf(id);
    if (!part) {
      closeCard();
      return;
    }
    const close = document.createElement("button");
    close.type = "button";
    close.className = "osml-nodecard__close";
    close.setAttribute("aria-label", "Close");
    close.textContent = "×";
    close.addEventListener("click", () => closeCard(true));
    const stereo = document.createElement("div");
    stereo.className = "osml-nodecard__stereo";
    stereo.textContent = `«${part.attrs.kind ?? "part"}»`;
    const name = document.createElement("div");
    name.className = "osml-nodecard__name";
    name.id = "osml-nodecard-name";
    name.textContent = part.attrs.label;
    const sub = document.createElement("div");
    sub.className = "osml-nodecard__sub";
    sub.textContent = `part ${part.feature} : ${part.symbol.slice(part.symbol.lastIndexOf("::") + 2)}`;
    const attrs = document.createElement("div");
    attrs.className = "osml-nodecard__attrs";
    for (const key of Object.keys(part.attrs).sort()) {
      row(attrs, key, part.attrs[key]);
    }
    const node = model.render.nodes.find((candidate) => candidate.id === id);
    for (const port of node?.ports ?? []) {
      row(attrs, `port ${port.name}`, port.type ?? "");
    }
    const wires = document.createElement("div");
    wires.className = "osml-nodecard__attrs";
    for (const edge of model.render.edges) {
      if (edge.from !== id && edge.to !== id) {
        continue;
      }
      const peer = partOf(edge.from === id ? edge.to : edge.from);
      row(wires, edge.from === id ? "→" : "←", `${peer?.attrs.label ?? "?"} (${edge.label || "interface"})`);
    }
    const note = document.createElement("div");
    note.className = "osml-nodecard__note";
    note.textContent = live
      ? `Read by the engine from model ${model.hash.slice(0, 12)}`
      : "The engine's saved reading of the published model. Run or edit it to read it live.";
    card.replaceChildren(close, stereo, name, sub, attrs, wires, note);
    const repo = webHref(part.attrs.repo);
    if (repo) {
      const link = document.createElement("a");
      link.className = "osml-nodecard__link";
      link.href = repo;
      link.target = "_blank";
      link.rel = "noopener";
      link.textContent = "Open the project ↗";
      card.append(link);
    }
    card.setAttribute("aria-labelledby", name.id);
  }

  function openCard(id: string, focus: boolean): void {
    cardFor = id;
    fillCard(id);
    card.hidden = false;
    syncClasses();
    placeCard();
    if (focus) {
      card.querySelector<HTMLButtonElement>(".osml-nodecard__close")?.focus();
    }
  }

  function closeCard(refocus = false): void {
    const was = cardFor;
    cardFor = undefined;
    card.hidden = true;
    card.replaceChildren();
    syncClasses();
    if (refocus && was) {
      groupOf(was)?.focus({ preventScroll: true });
    }
  }

  // placeCard keeps the card visible while preferring a spot beside its box.
  function placeCard(): void {
    if (!cardFor || card.hidden) {
      return;
    }
    const shape = groupOf(cardFor)?.querySelector(".shape");
    if (!shape) {
      return;
    }
    const heroRect = hero.getBoundingClientRect();
    const headerBottom = document.querySelector<HTMLElement>(".md-header")?.getBoundingClientRect().bottom ?? 0;
    const visibleLeft = Math.max(heroRect.left, 0);
    const visibleTop = Math.max(heroRect.top, 0, headerBottom);
    const visibleRight = Math.min(heroRect.right, window.innerWidth);
    const visibleBottom = Math.min(heroRect.bottom, window.innerHeight);
    const pad = 12;
    const visibleWidth = visibleRight - visibleLeft;
    const visibleHeight = visibleBottom - visibleTop;
    const area =
      visibleWidth > 2 * pad && visibleHeight > 2 * pad
        ? {
            x: visibleLeft - heroRect.left + pad,
            y: visibleTop - heroRect.top + pad,
            width: visibleWidth - 2 * pad,
            height: visibleHeight - 2 * pad,
          }
        : {
            x: pad,
            y: pad,
            width: Math.max(0, heroRect.width - 2 * pad),
            height: Math.max(0, heroRect.height - 2 * pad),
          };
    const at = shape.getBoundingClientRect();
    const box = new DOMRect(at.left - heroRect.left, at.top - heroRect.top, at.width, at.height);
    const gap = 14;
    const style = getComputedStyle(card);
    const horizontalInsets =
      style.boxSizing === "border-box"
        ? 0
        : Number.parseFloat(style.paddingLeft) +
          Number.parseFloat(style.paddingRight) +
          Number.parseFloat(style.borderLeftWidth) +
          Number.parseFloat(style.borderRightWidth);
    const verticalInsets =
      style.boxSizing === "border-box"
        ? 0
        : Number.parseFloat(style.paddingTop) +
          Number.parseFloat(style.paddingBottom) +
          Number.parseFloat(style.borderTopWidth) +
          Number.parseFloat(style.borderBottomWidth);
    card.style.maxWidth = `${Math.max(0, area.width - horizontalInsets)}px`;
    card.style.maxHeight = `${Math.max(0, area.height - verticalInsets)}px`;
    const width = card.offsetWidth;
    const height = card.offsetHeight;
    const others = [...content.querySelectorAll<SVGGElement>("g.opensysml-node")]
      .filter((group) => group.dataset.opensysmlId !== cardFor)
      .map((group) => {
        const r = (group.querySelector(".shape") ?? group).getBoundingClientRect();
        return new DOMRect(r.left - heroRect.left, r.top - heroRect.top, r.width, r.height);
      });
    const middleY = box.y + box.height / 2 - height / 2;
    const middleX = box.x + box.width / 2 - width / 2;
    const candidates = [
      { side: "right", x: box.right + gap, y: middleY },
      { side: "left", x: box.left - gap - width, y: middleY },
      { side: "below", x: middleX, y: box.bottom + gap },
      { side: "above", x: middleX, y: box.top - gap - height },
    ];
    const placements = candidates.map((candidate) => {
      const x = Math.min(Math.max(candidate.x, area.x), Math.max(area.x, area.x + area.width - width));
      const y = Math.min(Math.max(candidate.y, area.y), Math.max(area.y, area.y + area.height - height));
      const shift = Math.abs(x - candidate.x) + Math.abs(y - candidate.y);
      return { ...candidate, x, y, shift, rect: new DOMRect(x, y, width, height) };
    });
    const uncovered = placements.filter((placement) => overlap(placement.rect, box) === 0);
    const choices = uncovered.length > 0 ? uncovered : placements;
    let best: { x: number; y: number; cost: number } | undefined;
    for (const candidate of choices) {
      // Keep clear candidates when available; otherwise the selected box weighs four times as much.
      const cost =
        overlap(candidate.rect, box) * 4 +
        others.reduce((sum, other) => sum + overlap(candidate.rect, other), 0) +
        candidate.shift;
      if (!best || cost < best.cost) {
        best = { x: candidate.x, y: candidate.y, cost };
      }
    }
    card.style.left = `${best!.x}px`;
    card.style.top = `${best!.y}px`;
  }

  // ---- moving boxes ----

  function point(event: PointerEvent): RenderPoint {
    const p = new DOMPoint(event.clientX, event.clientY).matrixTransform(content.getScreenCTM()!.inverse());
    return { x: p.x, y: p.y };
  }

  function moveTo(id: string, at: RenderPoint): void {
    const part = partOf(id);
    const entry = layout.nodes.get(id);
    if (!part || !entry) {
      return;
    }
    placed.set(part.feature, clamped(entry, at));
    const previous = layout;
    layout = keepOrthogonalRoutes(layoutCanvas(result, overrides(), auto), previous);
    draw(layout);
  }

  function openProject(id: string): void {
    const repo = webHref(partOf(id)?.attrs.repo);
    if (repo) {
      window.open(repo, "_blank", "noopener");
    }
  }

  function partGroup(target: EventTarget | null): SVGGElement | undefined {
    const group = (target as Element | null)?.closest?.<SVGGElement>("g.opensysml-node.osml-part");
    return group && content.contains(group) ? group : undefined;
  }

  function endGesture(cancelled: boolean): void {
    const ended = gesture;
    if (!ended) {
      return;
    }
    clearTimeout(ended.timer);
    if (ended.frame !== undefined) {
      cancelAnimationFrame(ended.frame);
      ended.frame = undefined;
    }
    gesture = undefined;
    hero.classList.remove("osml-hero--dragging");
    if (ended.moved) {
      const entry = layout.nodes.get(ended.id);
      const at = entry && freePlacement(entry, ended.at, otherNodes(ended.id), placementBounds(), exitReach);
      if (entry && at) {
        moveTo(ended.id, alignedPlacement(entry, at, layout, placementBounds(), exitReach));
      }
    } else if (!cancelled && !ended.longPressed) {
      openProject(ended.id);
    }
  }

  on(svg, "pointerdown", (event) => {
    const group = partGroup(event.target);
    if (!group || event.button !== 0 || !laidOut) {
      return;
    }
    const id = group.dataset.opensysmlId!;
    const entry = layout.nodes.get(id)!;
    const started: Gesture = {
      id,
      pointer: event.pointerId,
      start: point(event),
      from: { x: entry.box.x, y: entry.box.y },
      at: { x: entry.box.x, y: entry.box.y },
      moved: false,
      longPressed: false,
    };
    // Touch has no right button: a held, unmoved press opens the same card.
    if (event.pointerType !== "mouse") {
      started.timer = setTimeout(() => {
        if (gesture === started && !started.moved) {
          started.longPressed = true;
          openCard(id, false);
        }
      }, LONG_PRESS);
    }
    gesture = started;
    event.preventDefault();
  });
  on(window, "pointermove", (event) => {
    const active = gesture;
    if (!active || event.pointerId !== active.pointer) {
      return;
    }
    const p = point(event);
    const dx = p.x - active.start.x;
    const dy = p.y - active.start.y;
    if (!active.moved && Math.hypot(dx, dy) * view.scale < DRAG_SLOP) {
      return;
    }
    if (!active.moved) {
      active.moved = true;
      clearTimeout(active.timer);
      hero.classList.add("osml-hero--dragging");
    }
    active.at = { x: active.from.x + dx, y: active.from.y + dy };
    if (active.frame === undefined) {
      active.frame = requestAnimationFrame(() => {
        active.frame = undefined;
        if (!signal.aborted && gesture === active) {
          moveTo(active.id, active.at);
        }
      });
    }
  });
  on(window, "pointerup", (event) => {
    if (gesture && event.pointerId === gesture.pointer) {
      endGesture(false);
    }
  });
  on(window, "pointercancel", (event) => {
    if (gesture && event.pointerId === gesture.pointer) {
      endGesture(true);
    }
  });
  on(svg, "contextmenu", (event) => {
    const group = partGroup(event.target);
    if (!group) {
      return;
    }
    event.preventDefault();
    openCard(group.dataset.opensysmlId!, false);
  });
  on(svg, "keydown", (event) => {
    const group = partGroup(event.target);
    if (!group) {
      return;
    }
    const id = group.dataset.opensysmlId!;
    const steps: Record<string, [number, number]> = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openProject(id);
    } else if (event.key === "ContextMenu" || (event.key === "F10" && event.shiftKey)) {
      event.preventDefault();
      openCard(id, true);
    } else if (steps[event.key] && laidOut) {
      event.preventDefault();
      const entry = layout.nodes.get(id)!;
      const box = entry.box;
      const step = (event.shiftKey ? 4 : 1) * KEY_STEP;
      const direction = steps[event.key];
      const at = freePlacement(
        entry,
        { x: box.x + direction[0] * step, y: box.y + direction[1] * step },
        otherNodes(id),
        placementBounds(),
        exitReach,
        { x: direction[0], y: direction[1] },
      );
      if (at) {
        moveTo(id, at);
      }
    }
  });
  const light = (id: string | undefined): void => {
    hot = id;
    syncClasses();
  };
  on(svg, "pointerover", (event) => {
    if (!gesture) {
      light(partGroup(event.target)?.dataset.opensysmlId);
    }
  });
  on(svg, "pointerout", (event) => {
    if (!gesture && !partGroup(event.relatedTarget)) {
      light(undefined);
    }
  });
  on(svg, "focusin", (event) => light(partGroup(event.target)?.dataset.opensysmlId));
  on(svg, "focusout", () => light(undefined));
  on(document, "pointerdown", (event) => {
    if (!card.hidden && !card.contains(event.target as Node) && !partGroup(event.target)) {
      closeCard();
    }
  });
  on(document, "keydown", (event) => {
    if (event.key === "Escape" && !card.hidden) {
      closeCard(true);
    }
  });

  // ---- running the model ----

  function delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  // Each move of the token takes a new motion number; an older travel stops where it is.
  let motion = 0;

  function travel(index: number, forward: boolean, duration = HOP): Promise<void> {
    const mine = ++motion;
    return new Promise((resolve) => {
      let began: number | undefined;
      const step = (now: number): void => {
        const line = content.querySelector<SVGGeometryElement>(`g.opensysml-edge[data-edge="${index}"] .line`);
        if (signal.aborted || mine !== motion || !line || !token) {
          resolve();
          return;
        }
        began ??= now;
        const t = Math.min(1, (now - began) / duration);
        const eased = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
        const at = line.getPointAtLength(line.getTotalLength() * (forward ? eased : 1 - eased));
        token.setAttribute("cx", String(at.x));
        token.setAttribute("cy", String(at.y));
        if (t < 1) {
          requestAnimationFrame(step);
        } else {
          resolve();
        }
      };
      requestAnimationFrame(step);
    });
  }

  function showToken(): SVGCircleElement {
    if (!token) {
      token = document.createElementNS(SVG_NS, "circle");
      token.setAttribute("class", "osml-token");
      token.setAttribute("r", "7");
      content.append(token);
    }
    token.style.display = "";
    return token;
  }

  function dropToken(): void {
    motion++;
    token?.remove();
    token = undefined;
  }

  function edgeBetween(from: string | undefined, to: string | undefined): number {
    return result.edges.findIndex(
      (edge) => (edge.from === from && edge.to === to) || (edge.from === to && edge.to === from),
    );
  }

  async function animate(visited: string[]): Promise<void> {
    try {
      for (let i = 0; i < visited.length && !signal.aborted; i++) {
        liveNode = visited[i];
        liveEdge = undefined;
        syncClasses();
        const next = visited[i + 1];
        if (next === undefined) {
          await delay(1200);
          break;
        }
        const index = edgeBetween(liveNode, next);
        if (index < 0 || reduceMotion.matches) {
          await delay(800);
          continue;
        }
        liveEdge = index;
        syncClasses();
        showToken();
        await travel(index, result.edges[index].from === liveNode);
        if (token) {
          token.style.display = "none";
        }
      }
    } finally {
      dropToken();
      liveNode = undefined;
      liveEdge = undefined;
      syncClasses();
    }
  }

  // The model to run: the editor's text when it reads cleanly, else the last good model.
  async function runnable(): Promise<LandingModel | undefined> {
    const source = await currentSource();
    return source === goodSource ? model : read(source, "run");
  }

  let seed: number | undefined;
  let seedPicked = false;
  let runSeed: number | undefined;
  const randomSeed = (): number => 1 + Math.floor(Math.random() * 9999);
  const sameEvents = (a: readonly string[], b: readonly string[]): boolean =>
    a.length === b.length && a.every((event, index) => event === b[index]);

  on(runBtn, "click", () => {
    if (running) {
      return;
    }
    pause();
    running = true;
    runBtn.disabled = true;
    syncDebugControls();
    status("Loading the engine and reading the model…");
    void (async () => {
      let ran = false;
      try {
        const current = await runnable();
        if (!current) {
          return;
        }
        const picked = seed === undefined || seedPicked;
        if (picked) {
          seed = randomSeed();
          seedPicked = true;
          seedEl.value = String(seed);
        }
        const used = seed;
        const started = performance.now();
        const visited = journey(await engine(), current, used);
        ran = true;
        const ms = Math.max(1, Math.round(performance.now() - started));
        if (visited.length === 0) {
          throw new Error("ExecuteState visited none of the diagram's parts");
        }
        const names = visited.map((id) => partOf(id)?.attrs.label ?? id);
        status(
          `ExecuteState ran ModelJourney on ${JOURNEY_EVENTS.join(", ")} under ${picked ? "random seed" : "seed"} ${used} in ${ms} ms: ${names.join(" → ")}`,
        );
        runBtn.textContent = "▶ Run it again";
        await animate(visited);
      } catch (error) {
        status(`The engine could not run the model (${message(error)}). The diagram still works.`, true);
      } finally {
        running = false;
        runBtn.disabled = false;
        syncDebugControls();
        const eventsStale = ran && !sameEvents(sent, JOURNEY_EVENTS);
        if (run !== undefined && (runSeed !== seed || eventsStale)) {
          if (eventsStale) {
            sent = [...JOURNEY_EVENTS];
          }
          if (debugPanel.hidden) {
            run = undefined;
          } else {
            void rerun(-1);
          }
        } else if (!debugPanel.hidden) {
          showStep(cursor, false);
        }
      }
    })();
  });

  // ---- debugging the model ----
  // Every change re-runs ModelJourney from the start on the engine with all the events
  // sent so far; the panel then steps through that run's trace record by record.

  let sent: string[] = [...JOURNEY_EVENTS];
  let run: JourneyRun | undefined;
  let steps: DebugStep[] = [];
  // The last trace record shown; -1 is before the first.
  let cursor = -1;
  let playing = false;
  let playback = 0;
  let rerunning: Promise<void> | undefined;
  let rerunStale = false;

  const stateLabel = (state: string | undefined): string =>
    state === undefined ? "nowhere yet" : model.parts.get(state)?.attrs.label ?? state;

  function speed(): number {
    const value = Number(speedSel.value);
    return Number.isFinite(value) && value > 0 ? value : 1;
  }

  function recordText(step: DebugStep): string {
    const { record } = step;
    if (step.ignored) {
      return `${record.text}: ignored, no transition out of ${stateLabel(step.state)} accepts it`;
    }
    return record.text;
  }

  function fillTrace(): void {
    const items = steps.map((step, index) => {
      const item = document.createElement("li");
      item.className = `osml-debug__record osml-debug__record--${step.record.kind}`;
      item.classList.toggle("is-ignored", step.ignored === true);
      item.dataset.index = String(index);
      item.textContent = recordText(step);
      return item;
    });
    if (run?.error) {
      const failed = document.createElement("li");
      failed.className = "osml-debug__record osml-debug__record--error";
      failed.textContent = `The run failed: ${run.error}`;
      items.push(failed);
    }
    traceEl.replaceChildren(...items);
  }

  function syncDebugControls(): void {
    const idle = !running && rerunning === undefined;
    for (const button of sendBtns) {
      button.disabled = !idle;
    }
    debugResetBtn.disabled = !idle;
    playBtn.disabled = !idle || (!playing && cursor >= steps.length - 1);
    backBtn.disabled = !idle || playing || cursor < 0;
    stepBtn.disabled = !idle || playing || cursor >= steps.length - 1;
    seedEl.disabled = !idle;
    reseedBtn.disabled = !idle;
    playBtn.textContent = playing ? "❚❚ Pause" : "▶ Play";
    playBtn.setAttribute("aria-pressed", String(playing));
  }

  function showStep(index: number, animated: boolean): Promise<void> {
    cursor = Math.max(-1, Math.min(index, steps.length - 1));
    const step = steps[cursor];
    liveNode = step?.state === undefined ? undefined : idOf(step.state);
    const edge = step?.edge ? edgeBetween(idOf(step.edge.from), idOf(step.edge.to)) : -1;
    liveEdge = edge < 0 ? undefined : edge;
    syncClasses();
    for (const item of traceEl.querySelectorAll<HTMLElement>("li[data-index]")) {
      const current = Number(item.dataset.index) === cursor;
      item.classList.toggle("is-current", current);
      if (current) {
        item.setAttribute("aria-current", "step");
        const box = traceEl.getBoundingClientRect();
        const row = item.getBoundingClientRect();
        traceEl.scrollTop += row.top - box.top - (traceEl.clientHeight - row.height) / 2;
      } else {
        item.removeAttribute("aria-current");
      }
    }
    const accepted = step?.accepted ?? 0;
    const queue = sent.slice(accepted);
    queueEl.textContent = queue.length === 0 ? "empty" : queue.join(", ");
    const position = steps.length === 0 ? "no records" : `record ${cursor + 1} of ${steps.length}`;
    const where = step === undefined ? "Not started" : `In ${stateLabel(step.state)}`;
    const choice = step?.record.kind === "choice" ? `, chose ${step.record.taken ?? "?"}` : "";
    nowEl.textContent = `${where}${choice} · ${position} · ${accepted} of ${sent.length} events taken`;
    syncDebugControls();
    if (!animated || liveEdge === undefined || reduceMotion.matches || !step?.edge) {
      dropToken();
      return Promise.resolve();
    }
    showToken();
    const forward = result.edges[liveEdge].from === idOf(step.edge.from);
    return travel(liveEdge, forward, HOP / speed()).then(() => {
      if (cursor === index) {
        dropToken();
      }
    });
  }

  // rerun runs the sent events afresh and shows the run from record `from`.
  function rerun(from: number): Promise<void> {
    pause();
    const pending = (async () => {
      const current = await runnable();
      if (!current) {
        return;
      }
      const client = await engine();
      if (current !== model) {
        rerunStale = true;
        return;
      }
      // This run uses the latest model, including one its own read adopted.
      rerunStale = false;
      const started = performance.now();
      run = runJourney(client, current, sent, seed);
      runSeed = seed;
      steps = debugSteps(run.trace);
      const ms = Math.max(1, Math.round(performance.now() - started));
      const schedule = seed === undefined ? "the default schedule" : `seed ${seed}`;
      status(
        run.error
          ? `ExecuteState failed after ${steps.length} trace records: ${run.error}`
          : `ExecuteState ran ModelJourney on ${sent.length} events under ${schedule} in ${ms} ms: ${steps.length} trace records.`,
        run.error !== undefined,
      );
      fillTrace();
    })()
      .catch((error: unknown) => {
        status(`The engine could not run the model (${message(error)}). The diagram still works.`, true);
      })
      .finally(() => {
        rerunning = undefined;
        if (rerunStale) {
          rerunStale = false;
          return rerun(-1);
        }
        return showStep(Math.min(from, steps.length - 1), false);
      });
    rerunning = pending;
    syncDebugControls();
    return pending;
  }

  function pause(): void {
    playback++;
    playing = false;
    dropToken();
    syncDebugControls();
  }

  async function play(): Promise<void> {
    if (playing || running) {
      return;
    }
    const mine = ++playback;
    playing = true;
    syncDebugControls();
    while (mine === playback && !signal.aborted && cursor < steps.length - 1) {
      await showStep(cursor + 1, true);
      if (mine !== playback) {
        break;
      }
      await delay(450 / speed());
    }
    if (mine === playback) {
      playing = false;
      syncDebugControls();
    }
  }

  function setSeed(next: number | undefined): void {
    seed = next;
    seedPicked = false;
    seedEl.value = next === undefined ? "" : String(next);
    void rerun(-1);
  }

  on(debugBtn, "click", () => {
    const open = debugPanel.hidden;
    debugPanel.hidden = !open;
    debugBtn.setAttribute("aria-expanded", String(open));
    debugBtn.textContent = open ? "Hide the debugger" : "Debug the run";
    if (!open) {
      pause();
      dropToken();
      liveNode = undefined;
      liveEdge = undefined;
      syncClasses();
      return;
    }
    if (run === undefined) {
      status("Loading the engine and running the model…");
      void rerun(-1);
    } else {
      void showStep(cursor, false);
    }
  });
  for (const button of sendBtns) {
    on(button, "click", () => {
      const event = button.dataset.osmlSend;
      if (!event) {
        return;
      }
      const end = steps.length - 1;
      sent = [...sent, event];
      void rerun(end).then(() => play());
    });
  }
  on(debugResetBtn, "click", () => {
    sent = [];
    void rerun(-1).then(() => play());
  });
  on(playBtn, "click", () => {
    if (playing) {
      pause();
    } else {
      void play();
    }
  });
  on(stepBtn, "click", () => void showStep(cursor + 1, true));
  on(backBtn, "click", () => void showStep(cursor - 1, false));
  on(seedEl, "change", () => {
    const value = seedEl.value.trim();
    const parsed = Number(value);
    setSeed(value === "" || !Number.isSafeInteger(parsed) || parsed < 0 ? undefined : parsed);
  });
  on(reseedBtn, "click", () => setSeed(randomSeed()));

  // ---- editing the model ----

  on(srcBtn, "click", () => {
    const open = editor.hidden;
    editor.hidden = !open;
    srcBtn.setAttribute("aria-expanded", String(open));
    srcBtn.textContent = open ? "Hide the model" : "Edit the model";
    if (!open) {
      return;
    }
    if (srcEl.value) {
      srcEl.focus();
      return;
    }
    srcEl.value = "Loading the model…";
    srcEl.readOnly = true;
    publishedSource().then(
      (text) => {
        srcEl.value = text;
        srcEl.readOnly = false;
        srcEl.focus();
        srcEl.setSelectionRange(0, 0);
        srcEl.scrollTop = 0;
        // The engine is fetched now, so the first edit redraws without a wait.
        void engine().catch(() => undefined);
      },
      (error: unknown) => {
        srcEl.value = "";
        srcEl.readOnly = false;
        status(`Could not load the model: ${message(error)}. Hide and reopen the editor to try again.`, true);
      },
    );
  });
  on(srcEl, "input", () => {
    if (!srcEl.readOnly) {
      status("Loading the engine and reading the model…");
      scheduleEdit();
    }
  });
  on(resetBtn, "click", () => {
    publishedSource().then(
      (text) => {
        srcEl.value = text;
        srcEl.removeAttribute("aria-invalid");
        placed.clear();
        settle();
        scheduleEdit();
      },
      (error: unknown) => status(`Could not load the model: ${message(error)}.`, true),
    );
  });

  // ---- size, routing, and leaving the page ----

  const resize = new ResizeObserver(() => {
    if (!root.isConnected) {
      dispose();
      return;
    }
    settle();
  });
  resize.observe(hero);
  resize.observe(stage);
  window.addEventListener("resize", () => {
    if (cardFor) {
      placeCard();
    }
  }, { signal });
  window.addEventListener("scroll", () => {
    if (cardFor) {
      placeCard();
    }
  }, { signal });

  void loadAvoid(root.dataset.osmlLibavoid ?? "").then(
    () => {
      if (!signal.aborted && placed.size > 0) {
        settle();
      }
    },
    (error: unknown) => console.warn("libavoid did not load; moved boxes keep straight wires", error),
  );

  // Instant navigation swaps the page out without unloading it; Material's document$ marks each swap.
  const navigation = window.document$?.subscribe(() => {
    if (!root.isConnected) {
      dispose();
    }
  });

  function dispose(): void {
    if (signal.aborted) {
      return;
    }
    pause();
    const active = gesture;
    gesture = undefined;
    if (active) {
      clearTimeout(active.timer);
      if (active.frame !== undefined) {
        cancelAnimationFrame(active.frame);
        active.frame = undefined;
      }
    }
    ac.abort();
    resize.disconnect();
    navigation?.unsubscribe();
    clearTimeout(editTimer);
    svg.remove();
    hero.classList.remove("osml-hero--focus", "osml-hero--dragging");
    if (window.__osmlDiagram === mounted) {
      window.__osmlDiagram = undefined;
    }
  }

  const mounted: Mounted = { dispose };
  relayOut();
  return mounted;
}

window.osmlMountDiagram = (root: HTMLElement): void => {
  window.__osmlDiagram = mount(root);
};
