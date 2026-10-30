// Where route-goldens.ts may write: paths inside the repository checkout,
// canonicalized through their deepest existing ancestor before anything is created or written.
import { lstatSync, readlinkSync, realpathSync } from "node:fs";
import path from "node:path";

// Symlink hops before a chain is treated as a loop, as the kernel's ELOOP limit does.
const maxSymlinkHops = 40;

function isSymlink(target: string): boolean | null {
  try {
    return lstatSync(target).isSymbolicLink();
  } catch {
    return null;
  }
}

// realpath of a path whose tail may not exist yet, so `..` and symlinks in the existing part
// are resolved before anything is created. A dangling symlink counts as existing and is followed.
export function canonicalPendingPath(target: string, hops = 0): string {
  let existing = path.resolve(target);
  const pending: string[] = [];
  let symlink = isSymlink(existing);
  while (symlink === null) {
    pending.unshift(path.basename(existing));
    const parent = path.dirname(existing);
    if (parent === existing) {
      break;
    }
    existing = parent;
    symlink = isSymlink(existing);
  }
  if (symlink) {
    if (hops >= maxSymlinkHops) {
      throw new Error(`too many levels of symbolic links under '${target}'`);
    }
    const linked = path.resolve(path.dirname(existing), readlinkSync(existing));
    return canonicalPendingPath(path.join(linked, ...pending), hops + 1);
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
