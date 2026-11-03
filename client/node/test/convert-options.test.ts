// The compact and check-diagnostics options of connection.convert: what they
// send, the capabilities they need and how the check's diagnostics come back.

import assert from "node:assert/strict";
import { before, test } from "node:test";
import { MissingCapabilityError, connect } from "../src/node/index.js";
import { CAPABILITY_CONVERT_CHECK_DIAGNOSTICS, CAPABILITY_CONVERT_COMPACT } from "../src/core/capabilities.js";
import { ALL_CAPABILITIES, fakeConnection } from "./support/fake.js";
import { useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

const SOURCE = "package P { attribute a : ScalarValues::Integer = 1; }";

function recordingConvert(): { requests: Record<string, unknown>[]; answer: (method: string, input: unknown) => unknown } {
  const requests: Record<string, unknown>[] = [];
  return {
    requests,
    answer: (method, input) => {
      if (method !== "Convert") {
        throw new Error(`the fake service answers nothing for ${method}`);
      }
      requests.push(input as Record<string, unknown>);
      return {
        content: "{}",
        fromFormat: "sysml",
        toFormat: "api-json",
        error: "",
        diagnostics: [
          { severity: "warning", message: "unused import", code: "", orderDependent: [], independent: false },
          { severity: "error", message: "unresolved name", code: "", orderDependent: [], independent: false },
        ],
        experimental: false,
        experimentalNotice: "",
        libraryReferences: [],
      };
    },
  };
}

test("compact, omitDerived, keepDerived and includeCheckDiagnostics reach the wire", async () => {
  const { requests, answer } = recordingConvert();
  await using connection = await fakeConnection(
    [...ALL_CAPABILITIES, CAPABILITY_CONVERT_COMPACT, CAPABILITY_CONVERT_CHECK_DIAGNOSTICS],
    answer,
  );
  const asked = await connection.convert("api-json", { content: SOURCE }, {
    compact: true,
    omitDerived: true,
    keepDerived: ["qualifiedName", "name"],
    includeCheckDiagnostics: true,
  });
  await connection.convert("api-json", { content: SOURCE });

  const [wanted, defaulted] = requests as [Record<string, unknown>, Record<string, unknown>];
  assert.equal(wanted.compact, true);
  assert.equal(wanted.omitDerived, true);
  assert.deepEqual(wanted.keepDerived, ["qualifiedName", "name"]);
  assert.equal(wanted.includeCheckDiagnostics, true);
  assert.equal(defaulted.compact, false);
  assert.equal(defaulted.omitDerived, false);
  assert.deepEqual(defaulted.keepDerived, []);
  assert.equal(defaulted.includeCheckDiagnostics, false);
  // Each diagnostic keeps the severity the service gave it.
  assert.deepEqual(
    asked.diagnostics.map((diagnostic) => [diagnostic.severity, diagnostic.message]),
    [["warning", "unused import"], ["error", "unresolved name"]],
  );
});

for (const [name, options] of [
  ["compact", { compact: true }],
  ["omitDerived", { omitDerived: true }],
  ["keepDerived", { keepDerived: ["name"] }],
] as const) {
  test(`${name} is refused before the call when convert_compact is absent`, async () => {
    const { requests, answer } = recordingConvert();
    await using connection = await fakeConnection([...ALL_CAPABILITIES, CAPABILITY_CONVERT_CHECK_DIAGNOSTICS], answer);
    await assert.rejects(
      connection.convert("api-json", { content: SOURCE }, options),
      (error: unknown) => error instanceof MissingCapabilityError && error.capability === CAPABILITY_CONVERT_COMPACT,
    );
    assert.equal(requests.length, 0, "the request was sent to a service that would ignore it");
  });
}

test("includeCheckDiagnostics is refused before the call when convert_check_diagnostics is absent", async () => {
  const { requests, answer } = recordingConvert();
  await using connection = await fakeConnection([...ALL_CAPABILITIES, CAPABILITY_CONVERT_COMPACT], answer);
  await assert.rejects(
    connection.convert("api-json", { content: SOURCE }, { includeCheckDiagnostics: true }),
    (error: unknown) =>
      error instanceof MissingCapabilityError && error.capability === CAPABILITY_CONVERT_CHECK_DIAGNOSTICS,
  );
  assert.equal(requests.length, 0);
});

test("the service writes the compact document and reports the check's diagnostics", async () => {
  await using connection = await connect();
  const compact = await connection.convert("api-json", { content: SOURCE }, {
    fromFormat: "sysml",
    compact: true,
    omitDerived: true,
    keepDerived: ["qualifiedName"],
  });
  const document = JSON.parse(compact.content) as { format?: string };
  assert.equal(document.format, "api-json-compact/1");

  const broken = "package Broken { part def Boat { part motor : Missing; } }";
  const checked = await connection.convert("api-json", { content: broken }, {
    fromFormat: "sysml",
    includeCheckDiagnostics: true,
  });
  assert.ok(checked.diagnostics.some((diagnostic) => diagnostic.severity === "error"));
  const unchecked = await connection.convert("api-json", { content: broken }, { fromFormat: "sysml" });
  assert.deepEqual(unchecked.diagnostics, []);
});
