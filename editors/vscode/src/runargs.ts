// What a Run or Evaluate code lens runs: the `sysml` check that takes the lensed
// element by name. Mirrors runElementArgs in internal/frontend/lsp/codelens.go.

/** The arguments a `opensysml.runElement` lens carries. */
export interface RunElementArgs {
  uri: string;
  element: string;
  kind: string;
}

/** The `sysml` flag that runs each kind of element. */
const FLAGS: Record<string, string> = {
  action: "-action",
  state: "-state",
  calc: "-calc",
  constraint: "-constraint",
  requirement: "-requirement",
};

/** Narrows a lens argument to RunElementArgs; undefined for anything else. */
export function asRunElementArgs(value: unknown): RunElementArgs | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const { uri, element, kind } = value as Record<string, unknown>;
  if (typeof uri !== "string" || typeof element !== "string" || typeof kind !== "string" || !uri || !element) {
    return undefined;
  }
  return { uri, element, kind };
}

/**
 * The `sysml` arguments running the element on its file, e.g.
 * `["-calc", "Demo::Fall", "/ws/demo.sysml"]`; undefined for a kind no check takes.
 */
export function runArguments(args: RunElementArgs, file: string): string[] | undefined {
  const flag = FLAGS[args.kind];
  if (!flag) {
    return undefined;
  }
  return [flag, args.element, file];
}

/** The title the task running the element is shown under. */
export function runTitle(args: RunElementArgs): string {
  const verb = args.kind === "action" || args.kind === "state" ? "Run" : "Evaluate";
  return `${verb} ${args.element}`;
}
