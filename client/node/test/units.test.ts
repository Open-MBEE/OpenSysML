// The pure pieces: source documents, query building, bindings, input gating,
// verdict and exploration decoding, and the capability machinery — all without
// a service, the fake transport answering what a service would.

import assert from "node:assert/strict";
import { test } from "node:test";
import { create, toJsonString } from "@bufbuild/protobuf";
import {
  DocumentValueSchema,
  ValueSchema,
  VerdictSchema,
} from "../src/generated/sysml_pb.js";
import {
  CAPABILITY_BIG_INT_VALUES,
  CAPABILITY_PARSE_SOURCES,
  CAPABILITY_QUERY,
  CAPABILITY_SCHEDULE,
  CAPABILITY_SCHEDULE_EXPLORE,
  CAPABILITY_VERIFICATION,
  MissingCapabilityError,
  ServerInfo,
  capabilityRefusal,
  mismatchReason,
  requireCapability,
  upgradeRemedy,
} from "../src/node/index.js";
import { SourceDocument, sourceDocuments } from "../src/node/index.js";
import { buildQuery } from "../src/node/index.js";
import { ElementRef, ObjectRef, DocumentVerdict, buildBindings } from "../src/node/index.js";
import { Conversion, formatOfPath, isExperimental } from "../src/node/index.js";
import { Exploration, Outcome } from "../src/node/index.js";
import {
  explainVerdict,
  raiseVerdictError,
  verdictEvaluated,
  verdictHolds,
} from "../src/node/index.js";
import { decodeVerdict, toValue } from "../src/node/index.js";
import { ExecutionError, MalformedValueError, UnsupportedValueError } from "../src/node/index.js";
import { fakeConnection } from "./support/fake.js";

const ALL: string[] = [
  CAPABILITY_PARSE_SOURCES,
  CAPABILITY_QUERY,
  CAPABILITY_VERIFICATION,
  CAPABILITY_SCHEDULE,
  CAPABILITY_SCHEDULE_EXPLORE,
];

function info(capabilities: readonly string[], version = "test"): ServerInfo {
  return new ServerInfo({
    version,
    capabilities: [...capabilities],
    answered: true,
    origin: "test",
  });
}

test("a path is a file document; a tuple an inline one", () => {
  const docs = sourceDocuments(["lib.sysml", ["top", "package Top;"]]);
  assert.equal(docs[0].path, "lib.sysml");
  assert.equal(docs[0].documentName, "lib.sysml");
  assert.deepEqual(docs[0].toPb().source, { case: "filePath", value: "lib.sysml" });
  assert.equal(docs[1].content, "package Top;");
  assert.equal(docs[1].documentName, "top");
  assert.deepEqual(docs[1].toPb().source, { case: "content", value: "package Top;" });
});

test("SourceDocument.file and .inline validate their arguments", () => {
  assert.throws(() => SourceDocument.file(""), RangeError);
  assert.throws(() => SourceDocument.inline("", "package A;"), /name/i);
  assert.throws(
    () => SourceDocument.inline("top", "package A;", { language: "cobol" }),
    /sysml.*kerml|language/i,
  );
  const doc = SourceDocument.inline("top", "package A;", { language: "kerml" });
  assert.equal(doc.language, "kerml");
  assert.equal(doc.toPb().language, "kerml");
});

test("source_documents rejects the forms Python rejects", () => {
  assert.throws(() => sourceDocuments([]), RangeError);
  assert.throws(
    () => sourceDocuments([["same", "a"], ["same", "b"]]),
    RangeError,
  );
  assert.throws(() => sourceDocuments([1] as never), TypeError);
  assert.throws(() => sourceDocuments(["lib.sysml", ["lib.sysml", "x"]]), RangeError);
});

test("the cookbook payload translates verbatim", () => {
  const payload = {
    "@type": "Query",
    where: {
      "@type": "PrimitiveConstraint",
      operator: "=",
      property: "@type",
      value: ["PartUsage"],
    },
  };
  const query = buildQuery({ payload });
  const where = query.where;
  assert.ok(where !== undefined);
  if (where.constraint.case !== "primitive") {
    assert.fail("the cookbook's where must build a primitive constraint");
  }
  assert.equal(where.constraint.value.property, "@type");
  assert.deepEqual(where.constraint.value.value, ["PartUsage"]);
});

test("keyword form builds the same query", () => {
  const query = buildQuery({
    where: {
      "@type": "PrimitiveConstraint",
      operator: "=",
      property: "@type",
      value: ["PartUsage"],
    },
  });
  assert.equal(query.where?.constraint.case, "primitive");
});

test("composite constraints nest and operators translate", () => {
  const query = buildQuery({
    where: {
      "@type": "CompositeConstraint",
      operator: "and",
      constraint: [
        { "@type": "PrimitiveConstraint", operator: "=", property: "@type", value: ["PartUsage"] },
        { "@type": "PrimitiveConstraint", operator: ">", property: "mass", value: [100] },
      ],
    },
  });
  const where = query.where;
  assert.ok(where !== undefined);
  if (where.constraint.case !== "composite") {
    assert.fail("a composite constraint must build a composite constraint");
  }
  assert.equal(where.constraint.value.constraint.length, 2);
});

test("a payload the standard does not describe is refused", () => {
  assert.throws(() => buildQuery({ payload: { "@type": "Nonsense" } }), /Query/);
  assert.throws(
    () => buildQuery({ payload: { "@type": "Query", frobnicate: true } }),
    /frobnicate/,
  );
  assert.throws(
    () => buildQuery({ payload: { "@type": "Query" }, select: ["x"] }),
    /payload.*keyword|keyword.*payload|both/i,
  );
});

test("bindings translate each accepted form and refuse the rest", () => {
  const bindings = buildBindings({
    root: new ElementRef("Demo::car"),
    obj: new ObjectRef({ id: 7n }),
    name: "x",
    n: 3n,
    f: 1.5,
    b: true,
  });
  const byName = new Map(bindings.map((binding) => [binding.parameter, binding]));
  assert.equal(byName.get("root")?.values[0].kind.case, "elementId");
  assert.equal(byName.get("obj")?.values[0].kind.case, "object");
  assert.equal(byName.get("name")?.values[0].kind.value, "x");
  assert.equal(byName.get("n")?.values[0].kind.value, 3n);
  assert.equal(byName.get("f")?.values[0].kind.value, 1.5);
  assert.equal(byName.get("b")?.values[0].kind.value, true);
});

test("a binding refuses what queries answer rather than bind", () => {
  assert.throws(
    () =>
      buildBindings({
        v: new DocumentVerdict({
          assertion: new ElementRef("a"),
          kind: "constraint",
          text: "t",
          path: "",
          status: "holds",
        }),
      }),
    /verdict.*answered|answered.*verdict/i,
  );
  assert.throws(() => buildBindings({ o: new ObjectRef() }), /id or by path/);
  assert.throws(() => buildBindings({ q: create(DocumentValueSchema) as never }), /binding is/);
});

test("a bound Integer beyond int64 goes out as its decimal, in a value or a magnitude", () => {
  const [wide, mass] = buildBindings({
    n: 1n << 63n,
    mass: { kind: "quantity", magnitude: { kind: "int", value: -(1n << 70n) }, unit: "kg" },
  });
  assert.deepEqual(wide.values[0].kind, { case: "bigIntValue", value: "9223372036854775808" });
  const quantity = mass.values[0].kind;
  assert.equal(quantity.case, "quantity");
  assert.deepEqual(quantity.value.magnitude, {
    case: "bigIntMagnitude",
    value: "-1180591620717411303424",
  });
  const [edge] = buildBindings({ n: (1n << 63n) - 1n });
  assert.deepEqual(edge.values[0].kind, { case: "intValue", value: (1n << 63n) - 1n });
});

test("bound quantity values go out as wire quantities", () => {
  const bindings = buildBindings({
    mass: { kind: "quantity", magnitude: { kind: "real", value: 1.5 }, unit: "kg" },
  });
  const value = bindings[0].values[0];
  assert.equal(value.kind.case, "quantity");
  assert.deepEqual(value.kind.value.magnitude, { case: "realMagnitude", value: 1.5 });
});

test("formatOfPath names a format by extension and refuses an unknown one", () => {
  assert.equal(formatOfPath("a.sysml"), "sysml");
  assert.equal(formatOfPath("a.kerml"), "sysml");
  assert.equal(formatOfPath("a.ttl"), "ttl");
  assert.equal(formatOfPath("a.turtle"), "ttl");
  assert.equal(formatOfPath("a.json"), "api-json");
  assert.throws(() => formatOfPath("a.docx"), /format/);
});

test("isExperimental marks the same conversions Python marks", () => {
  assert.ok(isExperimental("sysml", "ttl"));
  assert.ok(isExperimental("ttl", "sysml"));
  assert.ok(isExperimental("xmi", "sysml"));
  assert.ok(!isExperimental("sysml", "sysml"));
});

test("Conversion renders as its content", () => {
  const conversion = new Conversion({
    content: "package A;",
    fromFormat: "sysml",
    toFormat: "sysml",
    diagnostics: [],
    experimental: false,
    experimentalNotice: "",
  });
  assert.equal(String(conversion), "package A;");
  assert.equal(conversion.length, "package A;".length);
});

test("mismatchReason says how a reported service differs", () => {
  const good = info(["a", "b"], "v1");
  assert.equal(mismatchReason(good, { version: "v1" }), undefined);
  assert.equal(
    mismatchReason(info([]), { version: "v1" }),
    "it reports version test, but v1 was asked for",
  );
  const missing = mismatchReason(info(["b"], "v1"), { capabilities: ["a", "b", "c"] });
  assert.match(missing ?? "", /does not report the "a", "c" capabilities/);
  const silent = new ServerInfo({
    version: "",
    capabilities: [],
    answered: false,
    origin: "test",
  });
  assert.match(mismatchReason(silent, { version: "v2" }) ?? "", /did not answer GetServerInfo/);
});

test("capabilityRefusal names the capability the details name", () => {
  const refusal = capabilityRefusal(info(["query"]), [CAPABILITY_QUERY, CAPABILITY_SCHEDULE]);
  assert.ok(refusal !== undefined);
  const error = refusal("the method requires schedule support");
  assert.ok(error instanceof MissingCapabilityError);
  assert.equal(error.capability, CAPABILITY_SCHEDULE);
  const fallback = refusal("the call is not implemented");
  assert.equal(fallback.capability, CAPABILITY_QUERY);
  assert.equal(capabilityRefusal(info([]), []), undefined);
});

test("toValue maps primitives and gates exotic kinds", () => {
  const full = new ServerInfo({
    version: "v",
    capabilities: ["complex_values", "set_values"],
    answered: true,
    origin: "test",
  });
  assert.equal(toValue(true, full).kind.case, "boolValue");
  assert.equal(toValue("s", full).kind.case, "stringValue");
  assert.equal(toValue(4n, full).kind.case, "intValue");
  assert.equal(toValue(1.5, full).kind.case, "realValue");
  const sparse = new ServerInfo({
    version: "v",
    capabilities: [],
    answered: true,
    origin: "test",
  });
  assert.throws(
    () =>
      toValue({ kind: "complex", value: { real: 1, imaginary: 2 } }, sparse),
    MissingCapabilityError,
  );
  assert.throws(
    () => toValue({ kind: "set", elements: [] }, sparse),
    MissingCapabilityError,
  );
});

test("a gated call refuses before it is sent, naming the capability", async () => {
  const conn = await fakeConnection([CAPABILITY_VERIFICATION]);
  await assert.rejects(
    () => conn.query("hash", {}),
    (error: unknown) => {
      assert.ok(error instanceof MissingCapabilityError);
      assert.equal(error.capability, CAPABILITY_QUERY);
      return true;
    },
  );
  const exploring = await fakeConnection([CAPABILITY_VERIFICATION, CAPABILITY_SCHEDULE]);
  await assert.rejects(() => exploring.exploreAnalysis("hash", "A::a"), MissingCapabilityError);
});

test("decoding a verdict keeps witness, question, standing and verifications", () => {
  const pb = create(VerdictSchema, {
    kind: "constraint",
    element: "mass constraint",
    elementId: "Demo::massOk",
    condition: "mass <= 2000",
    status: "violated",
    question: "holds",
    instanceId: 1n,
    witness: [
      {
        feature: "mass",
        value: { kind: { case: "realValue", value: 3000 } },
        unit: "kg",
        exact: "3000",
      },
    ],
    engine: "solver",
    strength: "proved",
    bounds: [{ name: "runs", limit: 64n, reached: false }],
  });
  const verdict = decodeVerdict(pb, []);
  assert.equal(verdict.kind, "fails");
  assert.equal(verdict.subject.elementId, "Demo::massOk");
  assert.equal(verdict.question, "holds");
  assert.equal(verdict.witness.length, 1);
  assert.equal(verdict.witness[0].exact, "3000");
  assert.equal(verdict.standing.engine, "solver");
  assert.equal(verdictEvaluated(verdict), true);
  assert.equal(verdictHolds(verdict), false);
  assert.match(explainVerdict(verdict), /✗ .* fails/);
});

test("a holding verdict explains as holding; undecided raises on request", () => {
  const holds = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "c",
      holds: true,
      status: "holds",
    }),
    [],
  );
  assert.equal(verdictHolds(holds), true);
  assert.match(explainVerdict(holds), /✓ .* holds/);
  const undecided = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "c",
      status: "undecided",
      error: "no solver",
    }),
    [],
  );
  assert.equal(verdictEvaluated(undecided), false);
  assert.throws(() => raiseVerdictError(undecided), ExecutionError);
});

test("Exploration reads its status, names a hit budget, and fails per outcome", () => {
  const outcome = (error: string, winner: number) =>
    new Outcome({
      outputs: new Map([["winner", { kind: "int" as const, value: BigInt(winner) }]]),
      finalState: "",
      statesVisited: [],
      error,
      linearizations: 1,
      probability: 0,
      witness: [],
      diagnostics: [],
    });
  const exploration = new Exploration({
    outcomes: [outcome("", 1), outcome("division by zero", 0)],
    complete: true,
    runs: 2,
    budgetsHit: [],
    runsBudget: 1024,
    depthBudget: 64,
    probabilitiesLowerBound: false,
  });
  assert.equal(exploration.status, "complete (2 runs)");
  assert.equal(exploration.length, 2);
  assert.equal(exploration.outcomes[1].failed, true);
  assert.throws(() => exploration.outcomes[1].raiseForError(), ExecutionError);

  const hit = new Exploration({
    outcomes: [outcome("", 1)],
    complete: false,
    runs: 1,
    budgetsHit: ["runs"],
    runsBudget: 1,
    depthBudget: 64,
    probabilitiesLowerBound: true,
  });
  assert.equal(
    hit.status,
    "incomplete: runs budget 1 hit after 1 runs; probabilities are lower bounds",
  );
  assert.throws(() => hit.raiseForIncomplete(), ExecutionError);
});

test("a requirement must report the capability or the call refuses", () => {
  const sparse = info([]);
  assert.throws(
    () => {
      requireCapability(sparse, CAPABILITY_QUERY, upgradeRemedy(CAPABILITY_QUERY));
    },
    MissingCapabilityError,
  );
  requireCapability(
    info([CAPABILITY_QUERY]),
    CAPABILITY_QUERY,
    upgradeRemedy(CAPABILITY_QUERY),
  );
});

test("an undecodable output stays in the map as its error", async () => {
  const conn = await fakeConnection(ALL, () => ({
    outputs: {},
    performerAttributes: {},
    error: "",
    diagnostics: [],
    outcomes: [],
    finalTime: 0,
  }));
  assert.equal(conn.info.version, "test");
  assert.ok(UnsupportedValueError.prototype instanceof Error);
});

test("an integer input outside int64 travels as big_int_value, nested included", () => {
  const full = new ServerInfo({
    version: "v",
    capabilities: ["set_values", "big_int_values"],
    answered: true,
    origin: "test",
  });
  for (const value of [2n ** 63n, -(2n ** 63n) - 1n, 2n ** 100n]) {
    assert.deepEqual(toValue(value, full).kind, { case: "bigIntValue", value: value.toString() });
    assert.deepEqual(toValue({ kind: "int", value }, full).kind, {
      case: "bigIntValue",
      value: value.toString(),
    });
  }
  for (const input of [{ kind: "set", elements: [1n, 2n ** 63n] } as const, [1n, 2n ** 63n]]) {
    const wire = toValue(input, full);
    assert.ok(wire.kind.case === "set" || wire.kind.case === "sequence");
    assert.deepEqual(wire.kind.value.elements[1]?.kind, {
      case: "bigIntValue",
      value: (2n ** 63n).toString(),
    });
  }
  assert.equal(toValue(2n ** 63n - 1n, full).kind.case, "intValue");
  assert.equal(toValue(-(2n ** 63n), full).kind.case, "intValue");
});

test("a JavaScript array encodes as a sequence and a Set as a set", () => {
  const full = new ServerInfo({
    version: "v",
    capabilities: ["set_values"],
    answered: true,
    origin: "test",
  });
  const sequence = toValue([1n, "two", true], full);
  assert.equal(sequence.kind.case, "sequence");
  const set = toValue(new Set([{ kind: "int", value: 1n }, { kind: "int", value: 2n }]), full);
  assert.equal(set.kind.case, "set");
  // A SysMLValue collection accepts raw JavaScript elements the same way.
  const mixed = toValue({ kind: "set", elements: [1n, 2n] }, full);
  assert.equal(mixed.kind.case, "set");
  assert.throws(() => toValue({ kind: "set", elements: [1n, 1n] }, full), MalformedValueError);
});

test("an unset value and a foreign object cannot be sent as inputs", () => {
  const full = new ServerInfo({ version: "v", capabilities: [], answered: true, origin: "test" });
  assert.throws(
    () => toValue({ kind: "unset" }, full),
    (error: unknown) => {
      assert.ok(error instanceof RangeError);
      assert.match((error).message, /unset value cannot be sent/);
      return true;
    },
  );
  assert.throws(() => toValue({ nope: 1 } as never, full), RangeError);
});

test("a unit named without its reduction is refused before the call", () => {
  const full = new ServerInfo({
    version: "v",
    capabilities: ["measurement_refs", "structured_values", "tensor_values"],
    answered: true,
    origin: "test",
  });
  const unitTerm = { scaleNum: 1, scaleDen: 1, factors: [{ unitId: "SI::kg", exponent: 1 }] };
  assert.throws(
    () =>
      toValue(
        { kind: "quantity", magnitude: { kind: "real", value: 1 }, unit: "SI::kg" } as never,
        full,
      ),
    (error: unknown) => {
      assert.ok(error instanceof UnsupportedValueError);
      assert.match(
        (error).message,
        /quantity in \[SI::kg\] carries no reduction to base units/,
      );
      return true;
    },
  );
  assert.throws(
    () =>
      toValue({ kind: "measurementRef", unit: "SI::kg" } as never, full),
    (error: unknown) => {
      assert.ok(error instanceof UnsupportedValueError);
      assert.match(
        (error).message,
        /measurement reference SI::kg carries no reduction to base units/,
      );
      return true;
    },
  );
  assert.throws(
    () => toValue({ kind: "measurementRef", unit: "" } as never, full),
    (error: unknown) => {
      assert.ok(error instanceof UnsupportedValueError);
      assert.match((error).message, /naming no unit/);
      return true;
    },
  );
  // A reduction is sent as written.
  const ref = toValue(
    { kind: "measurementRef", unit: "SI::kg", unitTerm } as never,
    full,
  );
  assert.equal(ref.kind.case, "measurementRef");
});

test("an Integer outside int64 travels in every wire position, and an id outside it is refused", () => {
  const full = new ServerInfo({
    version: "v",
    capabilities: ["structured_values", "tensor_values", "function_values", "big_int_values"],
    answered: true,
    origin: "test",
  });
  const out = 2n ** 63n;
  const unitTerm = { scaleNum: 1, scaleDen: 1, factors: [{ unitId: "SI::kg", exponent: 1 }] };
  for (const input of [
    { kind: "quantity", magnitude: { kind: "int", value: out }, unit: "" },
    {
      kind: "vectorQuantity",
      components: [{ magnitude: { kind: "int", value: out }, unit: "", unitTerm }],
    },
    {
      kind: "tensorQuantity",
      dimensions: [1n],
      components: [{ magnitude: { kind: "int", value: out }, unit: "", unitTerm }],
    },
    { kind: "vector", components: [{ kind: "int", value: out }] },
    [{ kind: "quantity", magnitude: { kind: "int", value: out }, unit: "" }],
  ] as const) {
    assert.match(
      toJsonString(ValueSchema, toValue(input as never, full)),
      /"bigInt(Value|Magnitude)":"9223372036854775808"/,
    );
  }
  for (const input of [
    { kind: "instance", id: out },
    { kind: "function", calcId: "Demo::c", selfId: out },
  ] as const) {
    assert.throws(
      () => toValue(input, full),
      (error: unknown) => {
        assert.ok(error instanceof RangeError);
        assert.match((error).message, /value out of range/);
        return true;
      },
    );
  }
});

test("an Integer outside int64 is refused before the call by a service without big_int_values", () => {
  const older = new ServerInfo({
    version: "v",
    capabilities: ["structured_values", "tensor_values", "set_values"],
    answered: true,
    origin: "test",
  });
  const out = 2n ** 63n;
  const unitTerm = { scaleNum: 1, scaleDen: 1, factors: [{ unitId: "SI::kg", exponent: 1 }] };
  for (const input of [
    out,
    -(2n ** 63n) - 1n,
    { kind: "int", value: out },
    [1n, out],
    { kind: "set", elements: [out] },
    { kind: "quantity", magnitude: { kind: "int", value: out }, unit: "" },
    { kind: "vector", components: [{ kind: "int", value: out }] },
    {
      kind: "vectorQuantity",
      components: [{ magnitude: { kind: "int", value: out }, unit: "", unitTerm }],
    },
    {
      kind: "tensorQuantity",
      dimensions: [1n],
      components: [{ magnitude: { kind: "int", value: out }, unit: "", unitTerm }],
    },
  ] as const) {
    assert.throws(
      () => toValue(input as never, older),
      (error: unknown) => {
        assert.ok(error instanceof MissingCapabilityError);
        assert.equal(error.capability, CAPABILITY_BIG_INT_VALUES);
        return true;
      },
    );
  }
  assert.equal(toValue(2n ** 63n - 1n, older).kind.case, "intValue");
  assert.equal(toValue(-(2n ** 63n), older).kind.case, "intValue");
});

test("a measurement reference needs its reduction however the unit is named", () => {
  const full = new ServerInfo({
    version: "v",
    capabilities: ["measurement_refs"],
    answered: true,
    origin: "test",
  });
  // An id-only reference has a unit but still no reduction to send.
  assert.throws(
    () =>
      toValue(
        { kind: "measurementRef", unit: "", unitId: "SI::metre" } as never,
        full,
      ),
    (error: unknown) => {
      assert.ok(error instanceof UnsupportedValueError);
      assert.match(
        (error).message,
        /measurement reference SI::metre carries no reduction to base units/,
      );
      return true;
    },
  );
});

test("an array input refuses a shape its dimensions cannot fill before the call", async () => {
  let calls = 0;
  const connection = await fakeConnection(
    ["verification", "structured_values"],
    () => {
      calls += 1;
      return {};
    },
  );
  await assert.rejects(
    () =>
      connection.calc("hash", "Demo::c", {
        arguments: [
          { kind: "array", dimensions: [2n, 2n], elements: [1n, 2n, 3n] } as never,
        ],
      }),
    (error: unknown) => {
      assert.ok(error instanceof MalformedValueError);
      assert.match((error).message, /dimensions \(2, 2\) holds 3 element\(s\), want 4/);
      return true;
    },
  );
  assert.equal(calls, 0);
  await connection.close();
});
