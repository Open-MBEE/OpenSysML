#!/usr/bin/env node
'use strict';
// Reads one AsciiDoc body on stdin and writes one JSON tree of its blocks and
// inline runs on stdout; a parse failure exits non-zero with the message.
const asciidoctor = require('@asciidoctor/core')();

const OPEN = '\u0001';
const CLOSE = '\u0002';
const SEP = '\u0003';

const NAMED_ENTITIES = { lt: '<', gt: '>', amp: '&', quot: '"', apos: "'", nbsp: '\u00a0' };

function decodeEntities(text) {
  return text.replace(/&(#x[0-9a-fA-F]+|#\d+|[a-zA-Z]+);/g, (whole, body) => {
    if (body[0] === '#') {
      const code = body[1] === 'x' || body[1] === 'X' ? parseInt(body.slice(2), 16) : parseInt(body.slice(1), 10);
      return Number.isFinite(code) ? String.fromCodePoint(code) : whole;
    }
    return Object.prototype.hasOwnProperty.call(NAMED_ENTITIES, body) ? NAMED_ENTITIES[body] : whole;
  });
}

// mark wraps an inline node between markers the surrounding text is split on
// afterwards: its JSON hex-encoded, so that nothing in it meets the
// substitution pipeline, then — for a node with text of its own — the text as
// it stands, so that the substitutions still to run convert what it nests.
function mark(node, text) {
  const head = OPEN + Buffer.from(JSON.stringify(node), 'utf8').toString('hex');
  return text === undefined ? head + CLOSE : head + SEP + text + CLOSE;
}

function unmark(encoded) {
  return JSON.parse(Buffer.from(encoded, 'hex').toString('utf8'));
}

// segments splits converted content into text and marked nodes, each node
// with the text it nests (null for one without), markers balanced.
function segments(content) {
  const out = [];
  let textStart = 0;
  let i = 0;
  while (i < content.length) {
    if (content[i] !== OPEN) {
      i++;
      continue;
    }
    if (i > textStart) out.push(content.slice(textStart, i));
    let j = i + 1;
    while (content[j] !== SEP && content[j] !== CLOSE) j++;
    const node = unmark(content.slice(i + 1, j));
    let inner = null;
    if (content[j] === SEP) {
      let depth = 1;
      let k = j + 1;
      while (depth > 0) {
        if (content[k] === OPEN) depth++;
        else if (content[k] === CLOSE) depth--;
        k++;
      }
      inner = content.slice(j + 1, k - 1);
      j = k - 1;
    }
    out.push({ node, inner });
    i = j + 1;
    textStart = i;
  }
  if (textStart < content.length) out.push(content.slice(textStart));
  return out;
}

// runsOf maps converted inline content onto runs: plain text between
// markers, and the marked inline nodes, whose nested text is mapped recursively.
function runsOf(content, style) {
  const runs = [];
  for (const segment of segments(content)) {
    if (typeof segment === 'string') pushText(runs, segment, style);
    else runs.push(...expand(segment.node, segment.inner, style));
  }
  return runs;
}

function pushText(runs, text, style) {
  if (text === '') return;
  runs.push({ kind: 'text', style, text: decodeEntities(text) });
}

function expand(node, inner, style) {
  switch (node.kind) {
    case 'quoted': {
      if (style !== 'plain') return [{ kind: 'unsupported', construct: 'nested quoted text' }];
      return runsOf(inner, node.style);
    }
    case 'math':
      return [{ kind: 'math', text: decodeEntities(node.text) }];
    case 'link':
    case 'xref':
    case 'ref': {
      const text = plainText(inner);
      if (text === null) return [{ kind: 'unsupported', construct: 'styled ' + node.kind + ' text' }];
      return [Object.assign({}, node, { text })];
    }
    default:
      return [node];
  }
}

// plainText reads converted content that is text alone; null when it holds
// any other run.
function plainText(content) {
  const runs = runsOf(content, 'plain');
  if (runs.some((run) => run.kind !== 'text' || run.style !== 'plain')) return null;
  return runs.map((run) => run.text).join('');
}

class JSONConverter {
  constructor() {
    this.backendTraits = { basebackend: 'json', outfilesuffix: '.json', supports_templates: false };
  }

  convert(node, transform) {
    const name = transform || node.getNodeName();
    switch (name) {
      case 'inline_quoted':
        return this.inlineQuoted(node);
      case 'inline_anchor':
        return this.inlineAnchor(node);
      case 'inline_image':
        return mark({ kind: 'image', target: node.getTarget(), alt: node.getAlt() });
      case 'inline_break':
        return mark({ kind: 'text', style: 'plain', text: node.getText() + '\n' });
      default:
        if (name.startsWith('inline_')) return mark({ kind: 'unsupported', construct: name.slice('inline_'.length) });
        return '';
    }
  }

  inlineQuoted(node) {
    const type = node.getType();
    switch (type) {
      case 'emphasis':
      case 'strong':
      case 'monospaced':
        return mark({ kind: 'quoted', style: type }, node.getText());
      case 'unquoted':
        return node.getText();
      case 'latexmath':
        return mark({ kind: 'math', text: node.getText() });
      case 'sysml_ref':
        return mark({ kind: 'ref', target: node.getAttribute('target') }, node.getText());
      case 'sysml_value':
        return mark({ kind: 'value', target: node.getAttribute('target') });
      default:
        return mark({ kind: 'unsupported', construct: type === 'asciimath' ? 'asciimath' : type + ' text' });
    }
  }

  inlineAnchor(node) {
    switch (node.getType()) {
      case 'link':
        return mark({ kind: 'link', target: node.getTarget() }, node.getText());
      case 'xref': {
        const target = node.getAttribute('refid') || node.getTarget().replace(/^#/, '');
        return mark({ kind: 'xref', target }, node.getText() || '');
      }
      default:
        return mark({ kind: 'unsupported', construct: node.getType() + ' anchor' });
    }
  }
}

function lineOf(node) {
  const line = node.getLineNumber && node.getLineNumber();
  return typeof line === 'number' ? line : 0;
}

function block(node) {
  const base = { kind: node.getContext(), line: lineOf(node) };
  if (node.hasTitle && node.hasTitle()) {
    const title = plainText(node.getTitle());
    base.title = title === null ? '' : title;
    base.styledTitle = title === null;
  }
  switch (node.getContext()) {
    case 'paragraph':
      return Object.assign(base, { runs: runsOf(node.getContent(), 'plain') });
    case 'admonition':
      return Object.assign(base, {
        style: String(node.getAttribute('name')).toUpperCase(),
        label: String(node.getAttribute('textlabel') || node.getAttribute('name')),
        runs: runsOf(node.getContent(), 'plain'),
      });
    case 'ulist':
    case 'olist':
      return Object.assign(base, { kind: 'list', ordered: node.getContext() === 'olist', items: node.getItems().map(listItem) });
    case 'table':
      return Object.assign(base, table(node));
    case 'stem':
      return Object.assign(base, { style: node.getStyle(), source: node.getSource() });
    case 'image':
      return Object.assign(base, { target: node.getAttribute('target'), alt: node.getAttribute('alt') });
    default:
      return base;
  }
}

function listItem(item) {
  const entry = { line: lineOf(item), runs: runsOf(item.getText(), 'plain') };
  if (item.hasBlocks()) entry.blocks = item.getBlocks().map(block);
  return entry;
}

function table(node) {
  const rows = node.getRows();
  const header = rows.getHead().map((row) => row.map(cell));
  const body = rows.getBody().map((row) => row.map(cell));
  const footer = rows.getFoot().map((row) => row.map(cell));
  const specified = node.hasAttribute('cols');
  return {
    cols: specified ? node.getColumns().map((column) => Number(column.getAttribute('width')) || 1) : [],
    header,
    rows: body,
    footer,
  };
}

function cell(c) {
  const style = c.getStyle();
  if (style === 'asciidoc') return { runs: [{ kind: 'unsupported', construct: 'AsciiDoc table cell' }] };
  const span = { colspan: c.getAttribute('colspan') || 1, rowspan: c.getAttribute('rowspan') || 1 };
  if (span.colspan > 1 || span.rowspan > 1) return { runs: [{ kind: 'unsupported', construct: 'spanning table cell' }] };
  return { runs: runsOf(c.getText(), style && style !== 'default' ? style : 'plain') };
}

function main() {
  const chunks = [];
  process.stdin.on('data', (chunk) => chunks.push(chunk));
  process.stdin.on('end', () => {
    const body = Buffer.concat(chunks).toString('utf8');
    const registry = asciidoctor.Extensions.create();
    registry.inlineMacro('sysml', function () {
      this.positionalAttributes(['target', 'text']);
      this.process((parent, kind, attributes) => {
        if (kind !== 'ref' && kind !== 'value') {
          return this.createInline(parent, 'quoted', '', { type: 'sysml_unknown' });
        }
        return this.createInline(parent, 'quoted', attributes.text || '', {
          type: 'sysml_' + kind,
          attributes: { target: attributes.target || '' },
        });
      });
    });
    asciidoctor.ConverterFactory.register(new JSONConverter(), ['json']);
    let doc;
    try {
      doc = asciidoctor.load(body, {
        backend: 'json',
        safe: 'secure',
        sourcemap: true,
        extension_registry: registry,
        attributes: { stem: 'latexmath', 'skip-front-matter': '' },
      });
    } catch (err) {
      process.stderr.write(String(err && err.message ? err.message : err) + '\n');
      process.exit(1);
    }
    const blocks = doc.getBlocks().map(block);
    if (doc.hasHeader()) blocks.unshift({ kind: 'header', line: 1 });
    const out = { blocks };
    process.stdout.write(JSON.stringify(out) + '\n');
  });
}

main();
