// The hero's model shell, shared by every form of scene: the model the stage shows and the
// controls that change it (status line, editor, variant toggles); a form draws what it adopts.
import type { Connection } from "@openmbee/opensysml/browser";
import { blocks, readModel, sceneTarget, snapshotModel, type Diagnostic, type LandingModel, type SceneTarget } from "./model";
import { programOf, requiredVariant, variantChain, type Scene, type SceneControl, type SceneProgram, type SceneVariant } from "./scene";

const EDIT_DELAY = 300;

export interface Mounted {
  dispose(): void;
}

/** What a scene's mount is given by the hero: the module connection it shares and its model's URL. */
export interface MountOptions {
  engine(): Promise<Connection>;
  modelUrl: string;
  /** Called when the stage finds its root gone from the page. */
  onLeave(): void;
}

/** What a form does for the shell: show each model it adopts and keep its own controls in step. */
export interface ShellHooks {
  /** A model the engine read cleanly, under the text it was read from; the form redraws it. */
  adopt(model: LandingModel, source: string): void;
  /** Whether the form is working on the model itself, so an edit's read waits its turn. */
  busy(): boolean;
  /** The published text is back in the editor: the form forgets what the visitor did to the stage. */
  reset(): void;
  /** A variant changed what Run executes over the same model: the form forgets the previous program's run. */
  reprogram(): void;
  /** `running` changed: the form syncs the controls it owns. */
  syncControls(): void;
}

export interface Shell {
  /** The model the stage shows: the scene's snapshot until the engine reads the text. */
  readonly model: LandingModel;
  /** The root shown: the scene's own, or a variant's while its toggle is on. */
  readonly target: SceneTarget;
  /** What Run executes: the scene's program, or a variant's while its toggle is on. */
  readonly program: SceneProgram;
  /** Whether `model` was read live rather than taken from the snapshot. */
  readonly live: boolean;
  /** Whether a Run is in progress; the controls that would change the model wait. */
  readonly running: boolean;
  readonly signal: AbortSignal;
  status(text: string, error?: boolean): void;
  engine(): Promise<Connection>;
  publishedSource(): Promise<string>;
  /** The model to run: the editor's text when it reads cleanly, else the last good model. */
  runnable(): Promise<LandingModel | undefined>;
  setRunning(running: boolean): void;
  dispose(): void;
}

export function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function diagnosticText(diagnostic: Diagnostic): string {
  const at = diagnostic.line === undefined ? "" : `line ${diagnostic.line}:${diagnostic.column ?? 1}: `;
  return `${at}${diagnostic.message || "error"}`;
}

/** shortName is the last segment of a qualified name: `saturnV` of `MassRollup::saturnV`. */
export function shortName(symbol: string): string {
  return symbol.slice(symbol.lastIndexOf("::") + 2);
}

/** readSummary counts what a read model shows: a machine's states and transitions, else parts and interfaces. */
export function readSummary(model: LandingModel): string {
  if (model.machine) {
    return `${model.machine.states.size} states, ${model.render.edges.length} transitions`;
  }
  return `${model.parts.size} parts, ${model.render.edges.length} interfaces`;
}

export const sameTarget = (a: SceneTarget, b: SceneTarget): boolean => a.rootSymbol === b.rootSymbol && a.view === b.view;

export function mountShell(root: HTMLElement, scene: Scene, options: MountOptions, hooks: ShellHooks): Shell {
  const statusEl = root.querySelector<HTMLElement>("[data-osml-status]")!;
  const runBtn = root.querySelector<HTMLButtonElement>("[data-osml-run]")!;
  const srcBtn = root.querySelector<HTMLButtonElement>("[data-osml-source]")!;
  const editor = root.querySelector<HTMLElement>("[data-osml-editor]")!;
  const srcEl = root.querySelector<HTMLTextAreaElement>("[data-osml-src]")!;
  const resetBtn = root.querySelector<HTMLButtonElement>("[data-osml-reset]")!;
  const debugBtn = root.querySelector<HTMLButtonElement>("[data-osml-debug]")!;
  const downloadBtn = root.querySelector<HTMLButtonElement>("[data-osml-download]")!;
  const variantsEl = root.querySelector<HTMLElement>("[data-osml-variants]");
  // The controls are the page's; a scene shows the ones it declares and starts them afresh.
  const offers = (control: SceneControl): boolean => scene.controls.includes(control);
  runBtn.hidden = !offers("run");
  runBtn.disabled = false;
  runBtn.textContent = "▶ Run the model";
  srcBtn.hidden = !offers("edit");
  srcBtn.setAttribute("aria-expanded", "false");
  srcBtn.textContent = "Edit the model";
  editor.hidden = true;
  srcEl.value = "";
  srcEl.readOnly = false;
  srcEl.removeAttribute("aria-invalid");
  debugBtn.hidden = !offers("debug");
  downloadBtn.hidden = !offers("download");

  const ac = new AbortController();
  const { signal } = ac;
  const on = <K extends keyof HTMLElementEventMap>(
    target: HTMLElement,
    type: K,
    listener: (event: HTMLElementEventMap[K]) => void,
  ): void => target.addEventListener(type, listener as EventListener, { signal });

  let model = snapshotModel(scene);
  let target: SceneTarget = sceneTarget(scene);
  let live = false;
  let running = false;
  let goodSource: string | undefined;
  let editTimer: ReturnType<typeof setTimeout> | undefined;
  // The revision of what is read: the editor's text and the shown root; an older read is dropped.
  let editSequence = 0;
  let sourcePromise: Promise<string> | undefined;

  function status(text: string, error = false): void {
    if (signal.aborted) {
      // A scene that has left the stage says nothing over the one that replaced it.
      return;
    }
    statusEl.textContent = text;
    statusEl.classList.toggle("osml-schematic__status--err", error);
  }

  function engine(): Promise<Connection> {
    if (signal.aborted) {
      return Promise.reject(new Error("the diagram has left the page"));
    }
    return options.engine();
  }

  function publishedSource(): Promise<string> {
    sourcePromise ??= fetch(options.modelUrl)
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
    hooks.adopt(next, source);
  }

  // read parses the source on the engine; it reports diagnostics and keeps the last good model.
  async function read(source: string, verb: string): Promise<LandingModel | undefined> {
    const started = performance.now();
    const revision = editSequence;
    const outcome = await readModel(await engine(), scene, source, target);
    if (revision !== editSequence || signal.aborted) {
      // The editor took input or the root changed while the worker read this; the newer read follows.
      return undefined;
    }
    if (!outcome.model) {
      const first = outcome.diagnostics.find(blocks) ?? outcome.diagnostics[0];
      srcEl.setAttribute("aria-invalid", "true");
      status(`Not ${verb}: ${first ? diagnosticText(first) : "the model has nothing to show"}. The stage shows the last model that read cleanly.`, true);
      return undefined;
    }
    srcEl.removeAttribute("aria-invalid");
    if (source !== goodSource || !sameTarget(outcome.model.target, model.target)) {
      adopt(outcome.model, source);
    }
    const ms = Math.max(1, Math.round(performance.now() - started));
    status(`The engine read the model in ${ms} ms: ${readSummary(outcome.model)}.`);
    return outcome.model;
  }

  async function runnable(): Promise<LandingModel | undefined> {
    const source = await currentSource();
    return source === goodSource && sameTarget(model.target, target) ? model : read(source, "run");
  }

  function scheduleEdit(): void {
    const mine = ++editSequence;
    clearTimeout(editTimer);
    editTimer = setTimeout(() => {
      editTimer = undefined;
      if (running || hooks.busy()) {
        scheduleEdit();
        return;
      }
      const source = srcEl.value;
      void read(source, "redrawn").catch((error: unknown) => {
        if (mine === editSequence) {
          status(`Not redrawn: ${message(error)}. The stage shows the last model that read cleanly.`, true);
        }
      });
    }, EDIT_DELAY);
  }

  function setRunning(next: boolean): void {
    running = next;
    runBtn.disabled = next;
    syncVariantControls();
    hooks.syncControls();
  }

  // ---- switching the root ----
  // A variant's toggle shows the scene from another root over the same text.

  let variant: SceneVariant | undefined;
  const variantBtns = new Map<SceneVariant, HTMLButtonElement>();
  if (variantsEl) {
    variantsEl.replaceChildren();
    for (const candidate of scene.variants) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "osml-btn";
      button.setAttribute("aria-pressed", "false");
      button.textContent = candidate.label;
      variantsEl.append(button);
      variantBtns.set(candidate, button);
      // Switching a variant off returns to the one it builds on; one that is on only through another goes too.
      on(button, "click", () => void switchVariant(variantChain(scene, variant).includes(candidate) ? requiredVariant(scene, candidate) : candidate));
    }
  }

  function syncVariantControls(): void {
    const chain = variantChain(scene, variant);
    for (const [candidate, button] of variantBtns) {
      const isOn = chain.includes(candidate);
      const required = requiredVariant(scene, candidate);
      button.hidden = required !== undefined && !chain.includes(required);
      button.setAttribute("aria-pressed", String(isOn));
      button.textContent = isOn ? candidate.activeLabel : candidate.label;
      button.disabled = running;
    }
  }

  async function switchVariant(next: SceneVariant | undefined): Promise<void> {
    if (running) {
      return;
    }
    const before = programOf(scene, variant);
    variant = next;
    target = next === undefined ? sceneTarget(scene) : { rootSymbol: next.rootSymbol, view: next.view };
    if (sameTarget(target, model.target) && programOf(scene, variant) !== before) {
      hooks.reprogram();
    }
    // A read of the previous root still in flight is superseded, as an edit supersedes a read.
    const revision = ++editSequence;
    syncVariantControls();
    status(`Reading ${shortName(target.rootSymbol)}${next === undefined ? " again" : ""}…`);
    try {
      const current = await runnable();
      if (revision !== editSequence || signal.aborted) {
        return;
      }
      if (current && next !== undefined) {
        status(next.hint);
      }
      if (!current) {
        revertVariant();
      }
    } catch (error) {
      if (revision === editSequence) {
        status(`The engine could not read the model (${message(error)}). The stage still works.`, true);
        revertVariant();
      }
    }
  }

  // revertVariant puts the toggle back on the root the stage still shows after a read failed.
  function revertVariant(): void {
    if (sameTarget(model.target, target)) {
      return;
    }
    variant = scene.variants.find((candidate) => candidate.rootSymbol === model.target.rootSymbol && candidate.view === model.target.view);
    target = model.target;
    syncVariantControls();
  }

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
        hooks.reset();
        scheduleEdit();
      },
      (error: unknown) => status(`Could not load the model: ${message(error)}.`, true),
    );
  });

  function dispose(): void {
    if (signal.aborted) {
      return;
    }
    ac.abort();
    clearTimeout(editTimer);
    variantsEl?.replaceChildren();
  }

  syncVariantControls();
  return {
    get model() {
      return model;
    },
    get target() {
      return target;
    },
    get program() {
      return programOf(scene, variant);
    },
    get live() {
      return live;
    },
    get running() {
      return running;
    },
    signal,
    status,
    engine,
    publishedSource,
    runnable,
    setRunning,
    dispose,
  };
}
