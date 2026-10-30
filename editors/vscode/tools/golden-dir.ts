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

// Only the platform separators split components: on POSIX a backslash is part of a name.
const separators = path.sep === "/" ? /\/+/ : /[\\/]+/;

// The canonical form of a path whose tail may not exist yet: components are resolved in filesystem
// order, so `..` applies to the symlink target it follows, and a dangling symlink is followed too.
export function canonicalPendingPath(target: string): string {
  const absolute = path.isAbsolute(target) ? target : process.cwd() + path.sep + target;
  let current = path.parse(absolute).root;
  const parts = absolute.slice(current.length).split(separators);
  let hops = 0;
  while (parts.length > 0) {
    const part = parts.shift();
    if (part === undefined || part === "" || part === ".") {
      continue;
    }
    if (part === "..") {
      current = path.dirname(current);
      continue;
    }
    const next = path.join(current, part);
    if (isSymlink(next)) {
      if (++hops > maxSymlinkHops) {
        throw new Error(`too many levels of symbolic links under '${target}'`);
      }
      const linked = readlinkSync(next);
      if (path.isAbsolute(linked)) {
        current = path.parse(linked).root;
        parts.unshift(...linked.slice(current.length).split(separators));
      } else {
        parts.unshift(...linked.split(separators));
      }
      continue;
    }
    current = next;
  }
  return current;
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
