// The landing page's hero diagram: the stack model as the engine renders it, laid out
// by ELK, its wires rerouted by libavoid around the boxes a visitor moves, drawn by
// the same canvas code as the VS Code diagram panel.
import type { LayoutGeometry, RenderPoint, RenderResult } from "../protocol";
import { autoLayout, type AutoLayout } from "../webview/autolayout";
import { loadAvoid } from "../webview/avoid";
import { cssEscape, drawCanvas } from "../webview/canvas";
import { layoutCanvas, type Box, type CanvasLayout, type Overrides } from "../webview/layout";
import {
  JOURNEY_EVENTS,
  journey,
  landingModel,
  readModel,
  type Diagnostic,
  type EngineClient,
  type EngineInstance,
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
  held: CanvasLayout;
  moved: boolean;
  longPressed: boolean;
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
  let layout = layoutCanvas(result);
  let view = { x: 0, y: 0, scale: 1 };
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
    return { nodes };
  }

  // fit centres the unmoved diagram in the stage, scaled to the stage's width up to MAX_SCALE.
  function fit(): void {
    const home = layoutCanvas(result, {}, auto);
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

  function clamped(box: Box, at: RenderPoint): RenderPoint {
    const b = bounds();
    return {
      x: Math.min(Math.max(at.x, b.x), Math.max(b.x, b.x + b.width - box.width)),
      y: Math.min(Math.max(at.y, b.y), Math.max(b.y, b.y + b.height - box.height)),
    };
  }

  // keepInHero moves any box the hero no longer holds back inside it.
  function keepInHero(): void {
    let changed = false;
    for (const part of model.parts.values()) {
      const entry = layout.nodes.get(part.id);
      if (!entry) {
        continue;
      }
      const at = clamped(entry.box, entry.box);
      if (at.x !== entry.box.x || at.y !== entry.box.y) {
        placed.set(part.feature, at);
        changed = true;
      }
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
    return !editor.hidden && !srcEl.readOnly && srcEl.value ? Promise.resolve(srcEl.value) : publishedSource();
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
      if (running) {
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

  // placeCard puts the card beside its box, on whichever side covers the fewest other boxes.
  function placeCard(): void {
    if (!cardFor || card.hidden) {
      return;
    }
    const shape = groupOf(cardFor)?.querySelector(".shape");
    if (!shape) {
      return;
    }
    const heroRect = hero.getBoundingClientRect();
    const at = shape.getBoundingClientRect();
    const box = new DOMRect(at.left - heroRect.left, at.top - heroRect.top, at.width, at.height);
    const pad = 12;
    const gap = 14;
    card.style.maxHeight = `${Math.max(120, heroRect.height - 2 * pad)}px`;
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
      { x: box.right + gap, y: middleY },
      { x: box.left - gap - width, y: middleY },
      { x: middleX, y: box.bottom + gap },
      { x: middleX, y: box.top - gap - height },
    ];
    let best: { x: number; y: number; cost: number } | undefined;
    for (const candidate of candidates) {
      const x = Math.min(Math.max(candidate.x, pad), Math.max(pad, heroRect.width - width - pad));
      const y = Math.min(Math.max(candidate.y, pad), Math.max(pad, heroRect.height - height - pad));
      const rect = new DOMRect(x, y, width, height);
      // Covering its own box is worst; covering another is next; being pushed off its side counts a little.
      const cost =
        overlap(rect, box) * 4 +
        others.reduce((sum, other) => sum + overlap(rect, other), 0) +
        Math.abs(x - candidate.x) + Math.abs(y - candidate.y);
      if (!best || cost < best.cost) {
        best = { x, y, cost };
      }
    }
    card.style.left = `${Math.round(best!.x)}px`;
    card.style.top = `${Math.round(best!.y)}px`;
  }

  // ---- moving boxes ----

  function point(event: PointerEvent): RenderPoint {
    const p = new DOMPoint(event.clientX, event.clientY).matrixTransform(content.getScreenCTM()!.inverse());
    return { x: p.x, y: p.y };
  }

  function moveTo(id: string, at: RenderPoint, held?: CanvasLayout): void {
    const part = partOf(id);
    const entry = layout.nodes.get(id);
    if (!part || !entry) {
      return;
    }
    placed.set(part.feature, clamped(entry.box, at));
    const next = layoutCanvas(result, held ? { ...overrides(), held } : overrides(), auto);
    if (!held) {
      layout = next;
    }
    draw(next);
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
    gesture = undefined;
    hero.classList.remove("osml-hero--dragging");
    if (ended.moved) {
      layout = layoutCanvas(result, overrides(), auto);
      draw(layout);
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
      held: layout,
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
    if (!gesture || event.pointerId !== gesture.pointer) {
      return;
    }
    const p = point(event);
    const dx = p.x - gesture.start.x;
    const dy = p.y - gesture.start.y;
    if (!gesture.moved && Math.hypot(dx, dy) * view.scale < DRAG_SLOP) {
      return;
    }
    if (!gesture.moved) {
      gesture.moved = true;
      clearTimeout(gesture.timer);
      hero.classList.add("osml-hero--dragging");
    }
    moveTo(gesture.id, { x: gesture.from.x + dx, y: gesture.from.y + dy }, gesture.held);
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
      const box = layout.nodes.get(id)!.box;
      const step = (event.shiftKey ? 4 : 1) * KEY_STEP;
      moveTo(id, { x: box.x + steps[event.key][0] * step, y: box.y + steps[event.key][1] * step });
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

  function travel(index: number, forward: boolean): Promise<void> {
    return new Promise((resolve) => {
      let began: number | undefined;
      const step = (now: number): void => {
        const line = content.querySelector<SVGGeometryElement>(`g.opensysml-edge[data-edge="${index}"] .line`);
        if (signal.aborted || !line || !token) {
          resolve();
          return;
        }
        began ??= now;
        const t = Math.min(1, (now - began) / HOP);
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

  async function animate(visited: string[]): Promise<void> {
    token = document.createElementNS(SVG_NS, "circle");
    token.setAttribute("class", "osml-token");
    token.setAttribute("r", "7");
    token.style.display = "none";
    content.append(token);
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
        const index = result.edges.findIndex(
          (edge) => (edge.from === liveNode && edge.to === next) || (edge.from === next && edge.to === liveNode),
        );
        if (index < 0 || reduceMotion.matches) {
          await delay(800);
          continue;
        }
        liveEdge = index;
        syncClasses();
        token.style.display = "";
        await travel(index, result.edges[index].from === liveNode);
        token.style.display = "none";
      }
    } finally {
      token?.remove();
      token = undefined;
      liveNode = undefined;
      liveEdge = undefined;
      syncClasses();
    }
  }

  on(runBtn, "click", () => {
    if (running) {
      return;
    }
    running = true;
    runBtn.disabled = true;
    status("Loading the engine and reading the model…");
    void (async () => {
      try {
        const source = await currentSource();
        const current = source === goodSource ? model : await read(source, "run");
        if (!current) {
          return;
        }
        const started = performance.now();
        const visited = journey(await engine(), current);
        const ms = Math.max(1, Math.round(performance.now() - started));
        if (visited.length === 0) {
          throw new Error("ExecuteState visited none of the diagram's parts");
        }
        const names = visited.map((id) => partOf(id)?.attrs.label ?? id);
        status(`ExecuteState ran ModelJourney on ${JOURNEY_EVENTS.join(", ")} in ${ms} ms: ${names.join(" → ")}`);
        runBtn.textContent = "▶ Run it again";
        await animate(visited);
      } catch (error) {
        status(`The engine could not run the model (${message(error)}). The diagram still works.`, true);
      } finally {
        running = false;
        runBtn.disabled = false;
      }
    })();
  });

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
    ac.abort();
    resize.disconnect();
    navigation?.unsubscribe();
    clearTimeout(editTimer);
    clearTimeout(gesture?.timer);
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
