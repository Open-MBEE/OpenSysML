import { foldService } from '@codemirror/language';
import type { EditorState, Text } from '@codemirror/state';

/** The offsets of a `{` and the `}` that closes it. */
export interface BracePair {
  open: number;
  close: number;
}

/**
 * bracePairs finds every `{ … }` of the text, reading past the braces inside
 * strings, `'quoted names'` and comments as the lexer does. An unclosed brace
 * makes no pair; a stray `}` closes nothing.
 */
export function bracePairs(text: string): BracePair[] {
  const pairs: BracePair[] = [];
  const open: number[] = [];
  let i = 0;
  const skipTo = (end: string): void => {
    const at = text.indexOf(end, i);
    i = at === -1 ? text.length : at + end.length;
  };
  const skipQuoted = (quote: string): void => {
    while (i < text.length && text[i] !== quote) {
      i += text[i] === '\\' ? 2 : 1;
    }
    i += 1;
  };
  while (i < text.length) {
    const ch = text[i];
    const next = text[i + 1];
    i += 1;
    if (ch === '/' && next === '/') {
      i += 1;
      if (text[i] === '*') {
        i += 1;
        skipTo('*/');
      } else {
        skipTo('\n');
      }
    } else if (ch === '/' && next === '*') {
      i += 1;
      skipTo('*/');
    } else if (ch === '"' || ch === "'") {
      skipQuoted(ch);
    } else if (ch === '{') {
      open.push(i - 1);
    } else if (ch === '}' && open.length > 0) {
      pairs.push({ open: open.pop() as number, close: i - 1 });
    }
  }
  return pairs.sort((a, b) => a.open - b.open);
}

const pairsOf = new WeakMap<Text, BracePair[]>();

function pairsIn(doc: Text): BracePair[] {
  let pairs = pairsOf.get(doc);
  if (pairs === undefined) {
    pairs = bracePairs(doc.toString());
    pairsOf.set(doc, pairs);
  }
  return pairs;
}

/**
 * braceFoldRange is the range a line folds to: from after the last `{` the
 * line opens to before its `}`, when that `}` stands on a later line.
 */
export function braceFoldRange(state: EditorState, lineStart: number, lineEnd: number): { from: number; to: number } | null {
  let found: BracePair | null = null;
  for (const pair of pairsIn(state.doc)) {
    if (pair.open >= lineStart && pair.open < lineEnd && state.doc.lineAt(pair.close).from > lineEnd) {
      found = pair;
    }
  }
  return found === null ? null : { from: found.open + 1, to: found.close };
}

/** Folding of `{ … }` blocks, for cells and files in either language. */
export const braceFolding = foldService.of(braceFoldRange);
