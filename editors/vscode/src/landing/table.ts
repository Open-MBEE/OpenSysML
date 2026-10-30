// The hero's table form: the composition listed one part per row, indented as it nests, under
// the quantities the scene names; the rolled-up column stays empty until Run fills it bottom-up.
import { rollup, rollupLevels, valueText, type LandingModel, type LandingPart } from "./model";
import { quantityText } from "./present";
import type { Scene, SceneTable } from "./scene";
import { message, mountShell, shortName, type Mounted, type MountOptions } from "./shell";

export interface TableCell {
  text: string;
  /** Whether the engine left the feature undetermined, so `text` is its reason. */
  undetermined: boolean;
}

export interface TableRow {
  /** The part's id in the rendered view, which the roll-up animation lights the row by. */
  id: string;
  part: LandingPart;
  /** How deep the part nests: the root is 0, its parts 1, theirs 2. */
  depth: number;
  /** One cell per column of the table, in column order; empty where the part has no such feature. */
  cells: TableCell[];
}

export interface RollupTable {
  /** The quantities listed, the rolled-up feature last. */
  columns: readonly string[];
  rows: TableRow[];
}

/** cellOf is a part's cell for one feature: its quantity, else the engine's reason it has none; empty without either. */
export function cellOf(part: LandingPart, feature: string): TableCell {
  const quantity = part.quantities[feature];
  if (quantity !== undefined) {
    return { text: quantityText(quantity), undetermined: false };
  }
  const reason = part.unvalued[feature];
  return reason === undefined ? { text: "", undetermined: false } : { text: `undetermined: ${reason}`, undetermined: true };
}

/** rollupTable lists the root, then its parts in composition order, under the `listed` quantities and then `feature`. */
export function rollupTable(model: LandingModel, listed: readonly string[], feature: string): RollupTable {
  const parts = [model.root, ...model.parts.values()];
  const columns = [...listed.filter((name) => name !== feature), feature];
  return {
    columns,
    rows: parts.map((part) => ({
      id: part.id,
      part,
      depth: part === model.root ? 0 : part.path.split(".").length,
      cells: columns.map((column) => cellOf(part, column)),
    })),
  };
}

/** Placeholder text for a rolled-up cell before the engine fills it. */
export const PENDING = "—";

/** tableElement draws a table; the last column is the roll-up's, `filled` or left pending for Run under `data-osml-cell`. */
export function tableElement(table: RollupTable, label: string, filled: boolean): HTMLTableElement {
  const element = document.createElement("table");
  element.setAttribute("aria-label", label);
  const head = element.createTHead().insertRow();
  for (const title of ["Part", "Type"]) {
    const th = document.createElement("th");
    th.scope = "col";
    th.textContent = title;
    head.append(th);
  }
  for (const column of table.columns) {
    const th = document.createElement("th");
    th.scope = "col";
    th.className = "osml-table__num";
    th.textContent = column;
    head.append(th);
  }
  const computed = table.columns.length - 1;
  const body = element.createTBody();
  for (const row of table.rows) {
    const tr = body.insertRow();
    tr.dataset.osmlId = row.id;
    tr.classList.toggle("is-root", row.depth === 0);
    const name = tr.insertCell();
    name.className = "osml-table__part";
    name.style.setProperty("--osml-depth", String(row.depth));
    const keyword = document.createElement("span");
    keyword.className = "keyword";
    keyword.textContent = "part";
    name.append(keyword, row.part.feature);
    const type = tr.insertCell();
    type.className = "osml-table__type";
    type.textContent = row.part.type;
    row.cells.forEach((cell, index) => {
      const td = tr.insertCell();
      td.className = "osml-table__num";
      if (index === computed) {
        td.dataset.osmlCell = row.id;
        if (!filled) {
          td.classList.add("is-pending");
          td.textContent = PENDING;
          td.setAttribute("aria-label", "not yet evaluated; run the model");
          return;
        }
      }
      td.classList.toggle("is-undetermined", cell.undetermined);
      td.textContent = cell.text;
    });
  }
  return element;
}

/** fillCell writes a rolled-up cell the engine has evaluated, replacing its placeholder. */
export function fillCell(td: HTMLElement, cell: TableCell): void {
  td.classList.remove("is-pending");
  td.removeAttribute("aria-label");
  td.classList.toggle("is-undetermined", cell.undetermined);
  td.textContent = cell.text;
}

// mountTable lists a scene as a table and has Run fill its rolled-up column.
export function mountTable(root: HTMLElement, scene: Scene, form: SceneTable, options: MountOptions): Mounted {
  if (scene.program.kind !== "rollup") {
    throw new Error(`scene ${scene.id} is a table but has no roll-up to list`);
  }
  const { feature } = scene.program;
  const stage = root.querySelector<HTMLElement>("[data-osml-stage]")!;
  const runBtn = root.querySelector<HTMLButtonElement>("[data-osml-run]")!;
  const zoomBar = stage.querySelector<HTMLElement>("[data-osml-zoom]")!;
  zoomBar.hidden = true;
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const ac = new AbortController();
  const { signal } = ac;

  const wrap = document.createElement("div");
  wrap.className = "osml-table";
  stage.append(wrap);
  root.classList.add("osml-schematic--drawn");

  const shell = mountShell(root, scene, options, {
    adopt(next) {
      model = next;
      // A model the engine has just read is one Run has not rolled up yet.
      filled = false;
      draw();
    },
    busy: () => false,
    reset: () => undefined,
    syncControls: () => undefined,
  });
  const { status, engine, runnable } = shell;
  let model = shell.model;
  let table = rollupTable(model, form.columns, feature);
  // Whether the rolled-up column shows the engine's values; Run fills it, a new model empties it.
  let filled = false;

  function draw(): void {
    table = rollupTable(model, form.columns, feature);
    wrap.replaceChildren(tableElement(table, scene.description, filled));
  }

  function rowOf(id: string): HTMLTableRowElement | null {
    return wrap.querySelector<HTMLTableRowElement>(`tr[data-osml-id="${CSS.escape(id)}"]`);
  }

  function delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  // animateRollup fills the rolled-up column a level at a time from the deepest parts up, lighting each level's rows.
  async function animateRollup(levels: readonly string[][]): Promise<void> {
    const cells = new Map(table.rows.map((row) => [row.id, row.cells[row.cells.length - 1]]));
    let lit: HTMLTableRowElement[] = [];
    try {
      for (const ids of levels) {
        if (signal.aborted) {
          break;
        }
        for (const row of lit) {
          row.classList.remove("is-live");
        }
        lit = [];
        for (const id of ids) {
          const row = rowOf(id);
          const td = wrap.querySelector<HTMLElement>(`td[data-osml-cell="${CSS.escape(id)}"]`);
          const cell = cells.get(id);
          if (td && cell) {
            fillCell(td, cell);
          }
          if (row) {
            row.classList.add("is-live");
            lit.push(row);
          }
        }
        await delay(reduceMotion.matches ? 350 : 900);
      }
    } finally {
      for (const row of lit) {
        row.classList.remove("is-live");
      }
    }
  }

  // runRollup evaluates the root's rolled-up feature on the engine, shows it climbing the composition, then reports it.
  async function runRollup(): Promise<void> {
    const current = await runnable();
    if (!current || signal.aborted) {
      return;
    }
    const started = performance.now();
    const total = await rollup(await engine(), current, feature);
    const ms = Math.max(1, Math.round(performance.now() - started));
    const rootName = shortName(shell.target.rootSymbol);
    const objects = current.parts.size + 1;
    status(`Instantiate made ${objects} objects; the roll-up of ${rootName}.${feature} climbs from the deepest parts…`);
    runBtn.textContent = "▶ Run it again";
    if (current !== model) {
      model = current;
      draw();
    }
    filled = true;
    await animateRollup([...rollupLevels(current), [current.root.id]]);
    status(
      `Instantiate made ${objects} objects and Evaluate rolled up ${rootName}.${feature} = ${valueText(total)} in ${ms} ms.`,
      total.kind === "undetermined",
    );
  }

  runBtn.addEventListener(
    "click",
    () => {
      if (shell.running) {
        return;
      }
      shell.setRunning(true);
      status("Loading the engine and reading the model…");
      void (async () => {
        try {
          await runRollup();
        } catch (error) {
          status(`The engine could not run the model (${message(error)}). The table still works.`, true);
        } finally {
          shell.setRunning(false);
        }
      })();
    },
    { signal },
  );

  // Instant navigation swaps the page out without unloading it; Material's document$ marks each swap.
  const navigation = window.document$?.subscribe(() => {
    if (!root.isConnected) {
      dispose();
      options.onLeave();
    }
  });

  function dispose(): void {
    if (signal.aborted) {
      return;
    }
    ac.abort();
    shell.dispose();
    navigation?.unsubscribe();
    wrap.remove();
    root.classList.remove("osml-schematic--drawn");
  }

  draw();
  return { dispose };
}
