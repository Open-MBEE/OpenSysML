// Where route-goldens.ts may write: paths inside the repository checkout,
// canonicalized through their deepest existing ancestor before anything is created or written.
import { existsSync, realpathSync } from "node:fs";
import path from "node:path";

// realpath of a path whose tail may not exist yet, so `..` and symlinks in the
// existing part are resolved before the directory is created.
export function canonicalPendingPath(target: string): string {
  let existing = path.resolve(target);
  const pending: string[] = [];
  while (!existsSync(existing)) {
    pending.unshift(path.basename(existing));
    const parent = path.dirname(existing);
    if (parent === existing) {
      break;
    }
    existing = parent;
  }
  return path.join(realpathSync(existing), ...pending);
}

// The canonical form of `requested` (a directory, or a file that may be a symlink),
// or an error when it resolves outside `root`.
export function confinedPath(requested: string, root: string): string {
  const resolved = canonicalPendingPath(requested);
  const base = realpathSync(root);
  if (resolved !== base && !resolved.startsWith(base + path.sep)) {
    throw new Error(`output path '${requested}' resolves to ${resolved}, outside the repository checkout ${base}`);
  }
  return resolved;
}
