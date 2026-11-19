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

const headKey = (editingContextId: string, server: string, project: string, branch: string): string =>
  `head.${JSON.stringify([editingContextId, server, project, branch])}`;

interface Remembered extends PublishTarget {
  server: string;
  /** The branch as it was asked for: empty when the publication went to the project's default. */
  asked: string;
}

const NOTHING: Remembered = { server: '', project: '', branch: '', asked: '' };

const remembered = (editingContextId: string): Remembered => {
  const stored = read(`target.${editingContextId}`);
  if (!stored) {
    return NOTHING;
  }
  try {
    const parsed: unknown = JSON.parse(stored);
    if (typeof parsed === 'object' && parsed !== null) {
      const { server, project, branch, asked } = parsed as Record<string, unknown>;
      if (
        typeof server === 'string' &&
        typeof project === 'string' &&
        typeof branch === 'string' &&
        typeof asked === 'string'
      ) {
        return { server, project, branch, asked };
      }
    }
  } catch {
    // A value this module did not write is no memory.
  }
  return NOTHING;
};

/** The project and branch to offer: what the last publication from this browser asked for. */
export const rememberedTarget = (editingContextId: string): PublishTarget => {
  const { project, asked } = remembered(editingContextId);
  return { project, branch: asked };
};

/** The commit the branch was last seen at, or null when it was never published to from here. */
export const lastSeenHead = (editingContextId: string, target: PublishTarget): string | null => {
  const { server, project, branch, asked } = remembered(editingContextId);
  // The head is only known for the branch the last publication resolved.
  if (project === '' || target.project !== project || (target.branch !== asked && target.branch !== branch)) {
    return null;
  }
  return read(headKey(editingContextId, server, project, branch));
};

/** Keeps where a publication went: {@code asked} is the target as typed, {@code branch} the branch it resolved to. */
export const rememberPublication = (
  editingContextId: string,
  server: string,
  project: string,
  asked: string,
  branch: string,
  head: string
): void => {
  write(`target.${editingContextId}`, JSON.stringify({ server, project, branch, asked }));
  write(headKey(editingContextId, server, project, branch), head);
};
