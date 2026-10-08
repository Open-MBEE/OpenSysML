import assert from "node:assert/strict";
import { test } from "node:test";

import { showReferencesArguments } from "./references";

const convert = {
  asUri: (value: string) => ({ uri: value }) as never,
  asPosition: (value: { line: number; character: number }) => ({ pos: `${value.line}:${value.character}` }) as never,
  asLocation: (value: { uri: string; range: unknown }) => ({ loc: value.uri }) as never,
};

const location = (uri: string) => ({ uri, range: { start: { line: 1, character: 2 }, end: { line: 1, character: 7 } } });

test("a server's uri, position and locations are converted for the peek", () => {
  const args = showReferencesArguments(
    ["file:///ws/a.sysml", { line: 3, character: 9 }, [location("file:///ws/a.sysml"), location("file:///ws/b.sysml")]],
    convert,
  );
  assert.deepEqual(args, [{ uri: "file:///ws/a.sysml" }, { pos: "3:9" }, [{ loc: "file:///ws/a.sysml" }, { loc: "file:///ws/b.sysml" }]]);
});

test("arguments of another shape are left alone", () => {
  assert.equal(showReferencesArguments(undefined, convert), undefined);
  assert.equal(showReferencesArguments(["file:///ws/a.sysml", { line: 3, character: 9 }], convert), undefined);
  assert.equal(showReferencesArguments(["file:///ws/a.sysml", { line: 3 }, []], convert), undefined);
  assert.equal(showReferencesArguments(["file:///ws/a.sysml", { line: 3, character: 9 }, [{ uri: "x" }]], convert), undefined);
});
