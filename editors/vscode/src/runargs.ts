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
 * The `sysml` arguments running the element over the given model paths — the
 * workspace folders the language server indexes, so a name another file declares
 * without an import resolves as it does in the editor — e.g.
 * `["-calc", "Demo::Fall", "/ws"]`; undefined for a kind no check takes.
 */
export function runArguments(args: RunElementArgs, paths: string[]): string[] | undefined {
  const flag = FLAGS[args.kind];
  if (!flag || paths.length === 0) {
    return undefined;
  }
  return [flag, args.element, ...paths];
}

/**
 * The paths a run loads for a file: every workspace folder on disk, which is what
 * the language server indexes, or the file alone when it lies outside them all.
 */
export function runPaths(file: string, folders: readonly string[]): string[] {
  const sep = file.includes("\\") ? "\\" : "/";
  const inside = folders.some((folder) => file === folder || file.startsWith(folder.endsWith(sep) ? folder : folder + sep));
  return inside ? [...folders] : [file];
}

/** The title the task running the element is shown under. */
export function runTitle(args: RunElementArgs): string {
  const verb = args.kind === "action" || args.kind === "state" ? "Run" : "Evaluate";
  return `${verb} ${args.element}`;
}
