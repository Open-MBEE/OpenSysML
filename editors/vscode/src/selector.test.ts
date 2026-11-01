import assert from "node:assert/strict";
import { test } from "node:test";

import { documentSelector } from "./selector";

test("the selector serves files, library documents and notebook cells of both languages", () => {
  const selector = documentSelector("sysml-stdlib");
  for (const language of ["sysml", "kerml"]) {
    assert.deepEqual(
      selector.filter((entry) => typeof entry !== "string" && entry.language === language),
      [
        { scheme: "file", language },
        { scheme: "sysml-stdlib", language },
        { notebook: "*", language },
      ],
    );
  }
  assert.equal(selector.length, 6);
});
