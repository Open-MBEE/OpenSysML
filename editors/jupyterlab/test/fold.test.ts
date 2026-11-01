import assert from 'node:assert/strict';
import { test } from 'node:test';

import { foldable, StreamLanguage } from '@codemirror/language';
import { EditorState } from '@codemirror/state';

import { braceFolding, bracePairs } from '../src/fold';
import { kerml, sysml } from '../src/sysml';

/** The text each line of doc folds away, in order, '' for a line that does not fold. */
function folds(doc: string, language = sysml): string[] {
  const state = EditorState.create({ doc, extensions: [StreamLanguage.define(language), braceFolding] });
  const out: string[] = [];
  for (let n = 1; n <= state.doc.lines; n++) {
    const line = state.doc.line(n);
    const range = foldable(state, line.from, line.to);
    out.push(range === null ? '' : state.doc.sliceString(range.from, range.to));
  }
  return out;
}

test('a block spanning lines folds from after its brace to before the closing one', () => {
  assert.deepEqual(folds('package P {\n  part def A;\n}\n'), ['\n  part def A;\n', '', '', '']);
});

test('nested blocks fold separately and the last brace a line opens is the one that folds', () => {
  const doc = 'package P {\n  part def A { attribute x; } part def B {\n    attribute y;\n  }\n}';
  assert.deepEqual(folds(doc), ['\n  part def A { attribute x; } part def B {\n    attribute y;\n  }\n', '\n    attribute y;\n  ', '', '', '']);
});

test('a block closed on its own line does not fold', () => {
  assert.deepEqual(folds('part def A { attribute x; }\npart def B;'), ['', '']);
});

test('braces inside strings, quoted names and comments open and close nothing', () => {
  const doc = [
    "part def 'odd { name' {",
    '  doc /* a { comment */',
    '  // a } line comment',
    '  //* a { block comment */',
    '  attribute s = "{\\"}";',
    '}'
  ].join('\n');
  assert.deepEqual(
    bracePairs(doc).map((pair) => [doc[pair.open], doc[pair.close], doc.slice(0, pair.open).split('\n').length, doc.slice(0, pair.close).split('\n').length]),
    [['{', '}', 1, 6]]
  );
  assert.equal(folds(doc)[0], doc.slice(doc.indexOf('{', 20) + 1, doc.length - 1));
});

test('an unclosed brace folds nothing and a stray closing brace is passed over', () => {
  assert.deepEqual(bracePairs('package P {\n  part def A;\n'), []);
  assert.deepEqual(bracePairs('}\npackage P {\n}'), [{ open: 12, close: 14 }]);
});

test('KerML folds as SysML does', () => {
  assert.deepEqual(folds('classifier C {\n  feature f;\n}', kerml), ['\n  feature f;\n', '', '']);
});
