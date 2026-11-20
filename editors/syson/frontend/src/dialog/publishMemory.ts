/**
 * Where this browser last published an editing context, and the commit it left each branch at.
 * Kept in local storage per editing context; a token is never kept.
 */
export interface PublishTarget {
  project: string;
  branch: string;
}

const PREFIX = 'opensysml.publish.';

const read = (key: string): string | null => {
  try {
    return window.localStorage.getItem(PREFIX + key);
  } catch {
    // Storage can be denied; publishing then works without a memory.
    return null;
  }
};

const write = (key: string, value: string): void => {
  try {
    window.localStorage.setItem(PREFIX + key, value);
  } catch {
    // See read.
  }
};

interface Memory {
  /** The target the last publication asked for, offered again when the dialog opens. */
  last: PublishTarget;
  /** Per project, the branch a publication to its default branch resolved to. */
  defaults: Record<string, string>;
  /** Per project and resolved branch, the commit a publication left the branch at. */
  heads: Record<string, string>;
}

const headKey = (project: string, branch: string): string => JSON.stringify([project, branch]);

const isStrings = (value: unknown): value is Record<string, string> =>
  typeof value === 'object' &&
  value !== null &&
  !Array.isArray(value) &&
  Object.values(value).every((entry) => typeof entry === 'string');

const nothing = (): Memory => ({ last: { project: '', branch: '' }, defaults: {}, heads: {} });

const remembered = (editingContextId: string): Memory => {
  const stored = read(editingContextId);
  if (!stored) {
    return nothing();
  }
  try {
    const parsed: unknown = JSON.parse(stored);
    if (typeof parsed === 'object' && parsed !== null) {
      const { last, defaults, heads } = parsed as Record<string, unknown>;
      if (isStrings(last) && 'project' in last && 'branch' in last && isStrings(defaults) && isStrings(heads)) {
        return { last: { project: last.project ?? '', branch: last.branch ?? '' }, defaults, heads };
      }
    }
  } catch {
    // A value this module did not write is no memory.
  }
  return nothing();
};

/** The project and branch to offer: what the last publication from this browser asked for. */
export const rememberedTarget = (editingContextId: string): PublishTarget => remembered(editingContextId).last;

/** The commit the branch was last seen at, or null when it was never published to from here. */
export const lastSeenHead = (editingContextId: string, target: PublishTarget): string | null => {
  const { defaults, heads } = remembered(editingContextId);
  const branch = target.branch === '' ? defaults[target.project] : target.branch;
  return branch === undefined ? null : heads[headKey(target.project, branch)] ?? null;
};

/** Keeps where a publication went: `asked` is the target as typed, `branch` the branch it resolved to. */
export const rememberPublication = (
  editingContextId: string,
  asked: PublishTarget,
  branch: string,
  head: string
): void => {
  const memory = remembered(editingContextId);
  memory.last = asked;
  if (asked.branch === '') {
    memory.defaults[asked.project] = branch;
  }
  memory.heads[headKey(asked.project, branch)] = head;
  write(editingContextId, JSON.stringify(memory));
};
