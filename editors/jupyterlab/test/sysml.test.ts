import assert from 'node:assert/strict';
import { test } from 'node:test';

import { StreamLanguage, StringStream } from '@codemirror/language';
import { EditorState } from '@codemirror/state';

import { kerml, parser, State, sysml } from '../src/sysml';
import table from '../src/syntax.json';

type Token = [string, string | null];

/** Tokenizes text the way the editor does: line by line, carrying the state across lines. */
function tokenize(language: typeof sysml, text: string): Token[] {
  const tokens: Token[] = [];
  const state: State = language.startState!(2);
  for (const line of text.split('\n')) {
    const stream = new StringStream(line, 2, 2);
    if (line === '') {
      language.blankLine?.(state, 2);
      continue;
    }
    while (!stream.eol()) {
      const token = language.token(stream, state);
      const text = stream.current();
      if (text.trim() !== '') {
        tokens.push([text, token]);
      }
      stream.start = stream.pos;
    }
  }
  return tokens;
}

test('keywords take the generated group tags', () => {
  const tokens = tokenize(sysml, 'part def Vehicle :> Base { attribute mass : Real = 1.5; }');
  assert.deepEqual(tokens, [
    ['part', 'definitionKeyword'],
    ['def', 'definitionKeyword'],
    ['Vehicle', 'variableName'],
    [':>', 'operator'],
    ['Base', 'variableName'],
    ['{', 'punctuation'],
    ['attribute', 'definitionKeyword'],
    ['mass', 'variableName'],
    [':', 'punctuation'],
    ['Real', 'variableName'],
    ['=', 'operator'],
    ['1.5', 'number'],
    [';', 'punctuation'],
    ['}', 'punctuation']
  ]);
});

test('every keyword of the table is a keyword token in both languages', () => {
  const groups = Object.values(table.keywords).flat();
  assert.ok(groups.length > 100, `only ${groups.length} keywords`);
  for (const language of [sysml, kerml]) {
    for (const word of groups) {
      const [[, token]] = tokenize(language, word);
      assert.ok((token ?? '').toLowerCase().includes('keyword') || token === 'modifier', `${word}: ${token}`);
    }
    for (const word of table.constants) {
      const [[, token]] = tokenize(language, word);
      assert.ok(token === 'bool' || token === 'null', `${word}: ${token}`);
    }
  }
});

test('contextual words follow the language', () => {
  for (const word of table.contextual.sysml) {
    assert.deepEqual(tokenize(sysml, word), [[word, 'keyword']]);
  }
  assert.deepEqual(tokenize(kerml, 'var'), [['var', 'keyword']]);
  assert.deepEqual(tokenize(sysml, 'var'), [['var', 'variableName']]);
  assert.ok(!table.contextual.sysml.includes('var'));
});

test('an identifier that merely starts with a keyword is a name', () => {
  assert.deepEqual(tokenize(sysml, 'partial parts'), [
    ['partial', 'variableName'],
    ['parts', 'variableName']
  ]);
});

test('line and block comments, across lines', () => {
  assert.deepEqual(tokenize(sysml, 'part p; // trailing\n/* a\nb */ part q;'), [
    ['part', 'definitionKeyword'],
    ['p', 'variableName'],
    [';', 'punctuation'],
    ['// trailing', 'comment'],
    ['/* a', 'comment'],
    ['b */', 'comment'],
    ['part', 'definitionKeyword'],
    ['q', 'variableName'],
    [';', 'punctuation']
  ]);
});

test('doc and comment bodies are documentation, not comments', () => {
  assert.deepEqual(tokenize(sysml, 'doc /* The vehicle. */'), [
    ['doc', 'definitionKeyword'],
    ['/* The vehicle. */', 'docComment']
  ]);
  assert.deepEqual(tokenize(sysml, "comment Note about Vehicle locale \"en\"\n/* Multi\nline */"), [
    ['comment', 'definitionKeyword'],
    ['Note', 'variableName'],
    ['about', 'keyword'],
    ['Vehicle', 'variableName'],
    ['locale', 'keyword'],
    ['"en"', 'string'],
    ['/* Multi', 'docComment'],
    ['line */', 'docComment']
  ]);
  assert.deepEqual(tokenize(sysml, 'doc; /* plain */'), [
    ['doc', 'definitionKeyword'],
    [';', 'punctuation'],
    ['/* plain */', 'comment']
  ]);
});

test('strings keep their escapes', () => {
  assert.deepEqual(tokenize(sysml, 'attribute s = "say \\"hi\\" \\\\";'), [
    ['attribute', 'definitionKeyword'],
    ['s', 'variableName'],
    ['=', 'operator'],
    ['"say \\"hi\\" \\\\"', 'string'],
    [';', 'punctuation']
  ]);
});

test('numbers', () => {
  for (const literal of ['0', '42', '3.14', '1e10', '2.5E-3']) {
    assert.deepEqual(tokenize(sysml, literal), [[literal, 'number']]);
  }
  assert.deepEqual(tokenize(sysml, '1..3'), [
    ['1', 'number'],
    ['..', 'operator'],
    ['3', 'number']
  ]);
});

test('unrestricted and qualified names', () => {
  assert.deepEqual(tokenize(sysml, "part 'Front Wheel' : Wheels::'Road Wheel';"), [
    ['part', 'definitionKeyword'],
    ["'Front Wheel'", 'variableName'],
    [':', 'punctuation'],
    ['Wheels', 'variableName.special'],
    ['::', 'punctuation'],
    ["'Road Wheel'", 'variableName'],
    [';', 'punctuation']
  ]);
  assert.deepEqual(tokenize(sysml, 'import ISQ::*; import A::B::**;'), [
    ['import', 'keyword'],
    ['ISQ', 'variableName.special'],
    ['::', 'punctuation'],
    ['*', 'operator'],
    [';', 'punctuation'],
    ['import', 'keyword'],
    ['A', 'variableName.special'],
    ['::', 'punctuation'],
    ['B', 'variableName.special'],
    ['::', 'punctuation'],
    ['**', 'operator'],
    [';', 'punctuation']
  ]);
});

test('operators and punctuation', () => {
  assert.deepEqual(tokenize(sysml, 'ref :>> x = a -> select { in y; y > 0 } ::> b; #m @M;'), [
    ['ref', 'modifier'],
    [':>>', 'operator'],
    ['x', 'variableName'],
    ['=', 'operator'],
    ['a', 'variableName'],
    ['->', 'operator'],
    ['select', 'variableName'],
    ['{', 'punctuation'],
    ['in', 'modifier'],
    ['y', 'variableName'],
    [';', 'punctuation'],
    ['y', 'variableName'],
    ['>', 'operator'],
    ['0', 'number'],
    ['}', 'punctuation'],
    ['::>', 'operator'],
    ['b', 'variableName'],
    [';', 'punctuation'],
    ['#', 'punctuation'],
    ['m', 'variableName'],
    ['@', 'operator'],
    ['M', 'variableName'],
    [';', 'punctuation']
  ]);
  for (const op of table.operators) {
    assert.deepEqual(tokenize(sysml, op), [[op, 'operator']], op);
  }
});

test('kernel meta-commands are the first token of a line', () => {
  assert.deepEqual(tokenize(sysml, '%help\n  %show Vehicle\npart p; %eval 1'), [
    ['%help', 'meta'],
    ['%show', 'meta'],
    ['Vehicle', 'variableName'],
    ['part', 'definitionKeyword'],
    ['p', 'variableName'],
    [';', 'punctuation'],
    ['%', null],
    ['eval', 'variableName'],
    ['1', 'number']
  ]);
  assert.deepEqual(tokenize(sysml, '%'), [['%', null]]);
});

test('the parsers define languages whose comment tokens the editor can read', () => {
  for (const kind of ['sysml', 'kerml'] as const) {
    const language = StreamLanguage.define(parser(kind));
    assert.equal(language.name, kind);
    const state = EditorState.create({ doc: 'part def P { attribute a : Real; }', extensions: [language] });
    assert.deepEqual(state.languageDataAt('commentTokens', 0), [{ line: '//', block: { open: '/*', close: '*/' } }]);
    assert.equal(language.parser.parse(state.doc.toString()).length, state.doc.length);
  }
});
