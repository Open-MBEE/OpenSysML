'use strict';
// Runs the converter as the sysml binary does — one body on stdin, one JSON
// tree on stdout — and checks the tree for each construct the planner maps.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const path = require('node:path');

const converter = process.env.OPENSYSML_ASCIIDOCTOR || path.join(__dirname, 'asciidoctor-json.js');

function convert(body) {
  const run = spawnSync(process.execPath, [converter], { input: body, encoding: 'utf8' });
  assert.equal(run.status, 0, run.stderr);
  const lines = run.stdout.trim().split('\n');
  assert.equal(lines.length, 1, 'one JSON line');
  return JSON.parse(lines[0]).blocks;
}

test('inline runs are split out of the paragraph with their style, target and text', () => {
  const [paragraph] = convert('Plain _em_ *strong* `mono` link:https://x.io/a[lbl] <<id,text>> stem:[a^2] (C) -- end\n');
  assert.equal(paragraph.kind, 'paragraph');
  assert.equal(paragraph.line, 1);
  assert.deepEqual(paragraph.runs, [
    { kind: 'text', style: 'plain', text: 'Plain ' },
    { kind: 'text', style: 'emphasis', text: 'em' },
    { kind: 'text', style: 'plain', text: ' ' },
    { kind: 'text', style: 'strong', text: 'strong' },
    { kind: 'text', style: 'plain', text: ' ' },
    { kind: 'text', style: 'monospaced', text: 'mono' },
    { kind: 'text', style: 'plain', text: ' ' },
    { kind: 'link', target: 'https://x.io/a', text: 'lbl' },
    { kind: 'text', style: 'plain', text: ' ' },
    { kind: 'xref', target: 'id', text: 'text' },
    { kind: 'text', style: 'plain', text: ' ' },
    { kind: 'math', text: 'a^2' },
    { kind: 'text', style: 'plain', text: ' \u00a9\u2009\u2014\u2009end' },
  ]);
});

test('the sysml inline macros surface as nodes of their own', () => {
  const [paragraph] = convert('sysml:ref[Pkg::part] sysml:ref[Pkg::part, a label] sysml:value[Pkg::part.mass] sysml:other[x]\n');
  assert.deepEqual(
    paragraph.runs.filter((run) => run.kind !== 'text'),
    [
      { kind: 'ref', target: 'Pkg::part', text: '' },
      { kind: 'ref', target: 'Pkg::part', text: 'a label' },
      { kind: 'value', target: 'Pkg::part.mass' },
      { kind: 'unsupported', construct: 'sysml_unknown text' },
    ],
  );
});

test('special characters come back as the characters written', () => {
  const [paragraph] = convert('a < b & c > d, `x<y>`\n');
  assert.deepEqual(paragraph.runs, [
    { kind: 'text', style: 'plain', text: 'a < b & c > d, ' },
    { kind: 'text', style: 'monospaced', text: 'x<y>' },
  ]);
});

test('lists carry their items with line numbers, and nested lists as item blocks', () => {
  const [ulist, olist] = convert('* one\n* two _soft_\n\n[start=1]\n. first\n. second\n** deep\n');
  assert.equal(ulist.kind, 'list');
  assert.equal(ulist.ordered, false);
  assert.deepEqual(ulist.items.map((item) => item.line), [1, 2]);
  assert.deepEqual(ulist.items[1].runs, [
    { kind: 'text', style: 'plain', text: 'two ' },
    { kind: 'text', style: 'emphasis', text: 'soft' },
  ]);
  assert.equal(olist.ordered, true);
  assert.equal(olist.items[1].blocks[0].kind, 'list');
});

test('an admonition keeps its label and style', () => {
  const [note] = convert('NOTE: Mind the *gap*.\n');
  assert.equal(note.kind, 'admonition');
  assert.equal(note.style, 'NOTE');
  assert.equal(note.label, 'Note');
  assert.equal(note.runs[1].style, 'strong');
});

test('a table answers its header, body, footer and specified column widths', () => {
  const [table] = convert('.Masses\n[cols="3,1",options="footer"]\n|===\n|Part |Mass\n\n|Mirror |12 kg\n|Total |12 kg\n|===\n');
  assert.equal(table.kind, 'table');
  assert.equal(table.title, 'Masses');
  assert.deepEqual(table.cols, [3, 1]);
  assert.deepEqual(table.header, [[{ runs: [{ kind: 'text', style: 'plain', text: 'Part' }] }, { runs: [{ kind: 'text', style: 'plain', text: 'Mass' }] }]]);
  assert.equal(table.rows.length, 1);
  assert.equal(table.footer.length, 1);
  const [plain] = convert('|===\n|a |b\n|===\n');
  assert.deepEqual(plain.cols, []);
  assert.deepEqual(plain.header, []);
  assert.equal(plain.rows.length, 1);
});

test('stem blocks and block images answer their source, target, alt and title', () => {
  const [stem, image] = convert('.Scaling\n[stem]\n++++\nm \\propto D^2\n++++\n\n.Optics\nimage::figures/optics.png[The optics]\n');
  assert.deepEqual(stem, { kind: 'stem', line: 3, title: 'Scaling', styledTitle: false, style: 'latexmath', source: 'm \\propto D^2' });
  assert.deepEqual(image, { kind: 'image', line: 8, title: 'Optics', styledTitle: false, target: 'figures/optics.png', alt: 'The optics' });
});

test('blocks no content block carries are named by their context, and a header precedes all', () => {
  const blocks = convert('= Title\n\npara\n\n== Section\n\n----\ncode\n----\n');
  assert.deepEqual(blocks.map((block) => block.kind), ['header', 'preamble', 'section']);
  const [listing, quote] = convert('----\ncode\n----\n\n____\nsaid\n____\n');
  assert.equal(listing.kind, 'listing');
  assert.equal(quote.kind, 'quote');
});

test('nested quoted text, styled link text, inline images and footnotes are reported, not dropped', () => {
  const [paragraph] = convert('*a _b_* link:https://x.io[*l*] image:x.png[] here footnote:[n]\n');
  assert.deepEqual(
    paragraph.runs.filter((run) => run.kind !== 'text'),
    [
      { kind: 'unsupported', construct: 'nested quoted text' },
      { kind: 'unsupported', construct: 'styled link text' },
      { kind: 'image', target: 'x.png', alt: 'x' },
      { kind: 'unsupported', construct: 'footnote' },
    ],
  );
});
