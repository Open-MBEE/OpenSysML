import assert from "node:assert/strict";
import { test } from "node:test";

import { serverSettings } from "./settings";

function configuration(values: Record<string, unknown>) {
  return {
    get<T>(key: string, defaultValue: T): T {
      return key in values ? (values[key] as T) : defaultValue;
    },
  };
}

test("serverSettings defaults keepLineBreaks to the statement layout", () => {
  assert.deepEqual(serverSettings(configuration({})), { sysml: { keepLineBreaks: false } });
});

test("serverSettings forwards opensysml.format.keepLineBreaks", () => {
  assert.deepEqual(serverSettings(configuration({ "format.keepLineBreaks": true })), {
    sysml: { keepLineBreaks: true },
  });
});
