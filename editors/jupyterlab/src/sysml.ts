import { StreamParser, StringStream } from '@codemirror/language';

import table from './syntax.json';

/** Which of the two languages a parser reads; KerML has a few more contextual words. */
export type Kind = 'sysml' | 'kerml';

/** The tokenizer's state between lines: an open block comment or an awaited annotation body. */
export interface State {
  /** Inside a `/* ... *\/` block: the token it produces, or null when not in one. */
  block: 'comment' | 'docComment' | null;
  /** A `doc` or `comment` keyword has been read and its body is still to come. */
  expectBody: boolean;
  /** No token has been produced yet on the current line. */
  lineStart: boolean;
}

/** The CodeMirror highlighting tag each generated keyword group takes. */
const groupTokens: Record<string, string> = {
  declaration: 'definitionKeyword',
  control: 'controlKeyword',
  modifier: 'modifier',
  relationship: 'keyword',
  operator: 'operatorKeyword',
  other: 'keyword'
};

function keywordTokens(kind: Kind): Map<string, string> {
  const tokens = new Map<string, string>();
  for (const [group, words] of Object.entries(table.keywords)) {
    const token = groupTokens[group];
    if (token === undefined) {
      throw new Error(`syntax.json has a keyword group ${group} the tokenizer does not know`);
    }
    for (const word of words) {
      tokens.set(word, token);
    }
  }
  for (const word of table.constants) {
    tokens.set(word, word === 'null' ? 'null' : 'bool');
  }
  for (const word of table.contextual[kind]) {
    tokens.set(word, 'keyword');
  }
  return tokens;
}

const operators = table.operators.map(escapeRegExp).join('|');
const operatorPattern = new RegExp(`^(?:${operators}|[${escapeRegExp(table.operatorChars)}])`);
const metaCommandPattern = /^%[A-Za-z][\w-]*/;
const identifierPattern = /^[A-Za-z_][A-Za-z0-9_]*/;
const numberPattern = /^(?:\d+\.\d+(?:[eE][+-]?\d+)?|\d+[eE][+-]?\d+|\d+)/;
const qualifierPattern = /^\s*::(?!>)/;
const punctuationPattern = /^[;,.:{}()\[\]#]/;

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\\-]/g, '\\$&');
}

function readBlock(stream: StringStream, state: State): string {
  const token = state.block as string;
  while (!stream.eol()) {
    if (stream.match('*/')) {
      state.block = null;
      return token;
    }
    stream.next();
  }
  return token;
}

function readString(stream: StringStream, quote: string): void {
  while (!stream.eol()) {
    const ch = stream.next();
    if (ch === '\\') {
      stream.next();
    } else if (ch === quote) {
      return;
    }
  }
}

/** The name's role from what follows it: a namespace segment when `::` comes next. */
function nameToken(stream: StringStream): string {
  return stream.match(qualifierPattern, false) ? 'namespace' : 'variableName';
}

/** A CodeMirror stream parser for SysML v2 or KerML text. */
export function parser(kind: Kind): StreamParser<State> {
  const keywords = keywordTokens(kind);
  return {
    name: kind,
    startState: () => ({ block: null, expectBody: false, lineStart: true }),
    copyState: (state) => ({ ...state }),
    token(stream, state) {
      if (stream.sol()) {
        state.lineStart = true;
      }
      if (state.block !== null) {
        return readBlock(stream, state);
      }
      if (stream.eatSpace()) {
        return null;
      }
      const lineStart = state.lineStart;
      state.lineStart = false;
      if (lineStart && stream.match(metaCommandPattern)) {
        return 'meta';
      }
      if (stream.match('//')) {
        stream.skipToEnd();
        return 'comment';
      }
      if (stream.match('/*')) {
        state.block = state.expectBody ? 'docComment' : 'comment';
        state.expectBody = false;
        return readBlock(stream, state);
      }
      if (stream.match('"')) {
        readString(stream, '"');
        return 'string';
      }
      if (stream.match("'")) {
        readString(stream, "'");
        return nameToken(stream);
      }
      if (stream.match(numberPattern)) {
        return 'number';
      }
      const word = stream.match(identifierPattern);
      if (word) {
        const text = (word as RegExpMatchArray)[0];
        const token = keywords.get(text);
        if (token === undefined) {
          return nameToken(stream);
        }
        if (text === 'doc' || text === 'comment') {
          state.expectBody = true;
        }
        return token;
      }
      if (stream.match(operatorPattern) || stream.match('::')) {
        return stream.current() === '::' ? 'punctuation' : 'operator';
      }
      const punctuation = stream.match(punctuationPattern);
      if (punctuation) {
        const text = (punctuation as RegExpMatchArray)[0];
        if (text === ';' || text === '{' || text === '}') {
          state.expectBody = false;
        }
        return 'punctuation';
      }
      stream.next();
      return null;
    },
    languageData: {
      commentTokens: { line: '//', block: { open: '/*', close: '*/' } },
      closeBrackets: { brackets: ['(', '[', '{', "'", '"'] }
    }
  };
}

export const sysml = parser('sysml');
export const kerml = parser('kerml');
