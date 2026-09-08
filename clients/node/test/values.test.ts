// The unions a consumer switches on, and how a protobuf Value becomes one.

import assert from "node:assert/strict";
import { test } from "node:test";
import { create, toBinary, fromBinary } from "@bufbuild/protobuf";
import {
  ArraySchema,
  ComplexSchema,
  EnumLiteralSchema,
  FailureReason,
  FunctionSchema,
  MeasurementRefSchema,
  QuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
  ValueSchema,
  ValueSequenceSchema,
  VectorQuantitySchema,
  VectorSchema,
  VerdictSchema,
} from "../src/generated/sysml_pb.js";
import { MalformedValueError } from "../src/core/errors.js";
import {
  decodeValue,
  decodeVerdict,
  encodeValue,
  failureCause,
  formatValue,
  type SysMLValue,
} from "../src/core/values.js";

test("a value the service never sent is absent, and an unset feature is unset", () => {
  assert.deepEqual(decodeValue(undefined), { kind: "absent" });
  assert.deepEqual(decodeValue(create(ValueSchema, {})), { kind: "absent" });
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "unset", value: true } })), {
    kind: "unset",
  });
});

test("integers keep their width and reals stay numbers", () => {
  const big = 9007199254740993n;
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "intValue", value: big } })), {
    kind: "int",
    value: big,
  });
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "realValue", value: 0.5 } })), {
    kind: "real",
    value: 0.5,
  });
});

test("a complex number is one value with both parts, never two reals", () => {
  const complex = (real: number, imaginary: number) =>
    create(ValueSchema, { kind: { case: "complex", value: create(ComplexSchema, { real, imaginary }) } });
  assert.deepEqual(decodeValue(complex(1.5, -2)), {
    kind: "complex",
    value: { real: 1.5, imaginary: -2 },
  });
  assert.equal(formatValue(decodeValue(complex(1.5, -2))), "1.5 - 2.0i");
  assert.equal(formatValue(decodeValue(complex(1, 2))), "1.0 + 2.0i");
  assert.equal(formatValue(decodeValue(complex(0, 0))), "0.0 + 0.0i");
  assert.equal(formatValue(decodeValue(complex(-0.25, 1e300))), "-0.25 + 1e+300i");
  // An empty message is zero, as every proto3 default is.
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "complex", value: create(ComplexSchema, {}) } })), {
    kind: "complex",
    value: { real: 0, imaginary: 0 },
  });

  const nested = create(ValueSchema, {
    kind: {
      case: "sequence",
      value: create(ValueSequenceSchema, { elements: [complex(1, 2), complex(3, -4)] }),
    },
  });
  assert.deepEqual(decodeValue(nested), {
    kind: "sequence",
    elements: [
      { kind: "complex", value: { real: 1, imaginary: 2 } },
      { kind: "complex", value: { real: 3, imaginary: -4 } },
    ],
  });
  assert.equal(formatValue(decodeValue(nested)), "(1.0 + 2.0i, 3.0 - 4.0i)");
});

test("a sequence decodes its elements", () => {
  const value = create(ValueSchema, {
    kind: {
      case: "sequence",
      value: create(ValueSequenceSchema, {
        elements: [
          create(ValueSchema, { kind: { case: "intValue", value: 1n } }),
          create(ValueSchema, { kind: { case: "stringValue", value: "two" } }),
        ],
      }),
    },
  });
  const decoded = decodeValue(value);
  assert.equal(decoded.kind, "sequence");
  assert.deepEqual(decoded.elements, [
    { kind: "int", value: 1n },
    { kind: "string", value: "two" },
  ]);
  assert.equal(formatValue(decoded), '(1, "two")');
});

test("a quantity carries its magnitude, unit and reduction", () => {
  const value = create(ValueSchema, {
    kind: {
      case: "quantity",
      value: create(QuantitySchema, {
        magnitude: { case: "realMagnitude", value: 1500 },
        unit: "kg",
        unitTerm: create(UnitTermSchema, {
          scaleNum: 1000,
          scaleDen: 1,
          factors: [create(UnitFactorSchema, { unitId: "SI::g", exponent: 1 })],
        }),
      }),
    },
  });
  const decoded = decodeValue(value);
  assert.equal(decoded.kind, "quantity");
  assert.deepEqual(decoded.magnitude, { kind: "real", value: 1500 });
  assert.equal(decoded.unit, "kg");
  assert.deepEqual(decoded.unitTerm, {
    scaleNum: 1000,
    scaleDen: 1,
    factors: [{ unitId: "SI::g", exponent: 1 }],
  });
  assert.equal(formatValue(decoded), "1500.0[kg]");
});

const int = (value: bigint) => create(ValueSchema, { kind: { case: "intValue", value } });
const real = (value: number) => create(ValueSchema, { kind: { case: "realValue", value } });
const metres = (value: number) =>
  create(QuantitySchema, {
    magnitude: { case: "realMagnitude", value },
    unit: "m",
    unitTerm: create(UnitTermSchema, {
      scaleNum: 1,
      scaleDen: 1,
      factors: [create(UnitFactorSchema, { unitId: "SI::metre", exponent: 1 })],
    }),
  });
const array = (dimensions: bigint[], ...elements: ReturnType<typeof int>[]) =>
  create(ValueSchema, { kind: { case: "array", value: create(ArraySchema, { dimensions, elements }) } });
const vector = (...components: ReturnType<typeof int>[]) =>
  create(ValueSchema, { kind: { case: "vector", value: create(VectorSchema, { components }) } });
const vectorQuantity = (...components: ReturnType<typeof metres>[]) =>
  create(ValueSchema, {
    kind: { case: "vectorQuantity", value: create(VectorQuantitySchema, { components }) },
  });
const METRE = { scaleNum: 1, scaleDen: 1, factors: [{ unitId: "SI::metre", exponent: 1 }] };

test("an array keeps its dimensions and its elements in row-major order", () => {
  const grid = decodeValue(array([2n, 3n], int(1n), int(2n), int(3n), int(4n), int(5n), int(6n)));
  assert.deepEqual(grid, {
    kind: "array",
    dimensions: [2n, 3n],
    elements: [1n, 2n, 3n, 4n, 5n, 6n].map((value) => ({ kind: "int", value })),
  });
  assert.equal(formatValue(grid), "Array(2, 3)[1, 2, 3, 4, 5, 6]");

  // Rank 0 holds exactly one element; rank 1 and 3 keep every extent.
  assert.deepEqual(decodeValue(array([], real(7))), {
    kind: "array",
    dimensions: [],
    elements: [{ kind: "real", value: 7 }],
  });
  const line = decodeValue(array([3n], int(1n), int(2n), int(3n)));
  assert.equal(line.kind, "array");
  assert.deepEqual(line.dimensions, [3n]);
  const cube = decodeValue(array([2n, 2n, 2n], ...[0n, 1n, 2n, 3n, 4n, 5n, 6n, 7n].map(int)));
  assert.equal(cube.kind, "array");
  assert.deepEqual(cube.dimensions, [2n, 2n, 2n]);
  assert.equal(cube.elements.length, 8);

  // An element is any value, a nested array or a quantity included.
  const nested = decodeValue(
    array(
      [2n],
      array([1n], create(ValueSchema, { kind: { case: "quantity", value: metres(3) } })),
      vector(real(1), real(2)),
    ),
  );
  assert.deepEqual(nested, {
    kind: "array",
    dimensions: [2n],
    elements: [
      {
        kind: "array",
        dimensions: [1n],
        elements: [{ kind: "quantity", magnitude: { kind: "real", value: 3 }, unit: "m", unitTerm: METRE }],
      },
      { kind: "vector", components: [{ kind: "real", value: 1 }, { kind: "real", value: 2 }] },
    ],
  });
});

test("an array whose elements do not fill its dimensions is malformed", () => {
  assert.throws(() => decodeValue(array([2n, 3n], int(1n), int(2n))), MalformedValueError);
  assert.throws(() => decodeValue(array([0n])), MalformedValueError);
  assert.throws(() => decodeValue(array([-1n], int(1n))), MalformedValueError);
  assert.throws(
    () => encodeValue({ kind: "array", dimensions: [2n], elements: [{ kind: "int", value: 1n }] }),
    MalformedValueError,
  );
});

test("a vector is one value whose components stay integers or reals", () => {
  const reals = decodeValue(vector(real(3), real(4)));
  assert.deepEqual(reals, {
    kind: "vector",
    components: [{ kind: "real", value: 3 }, { kind: "real", value: 4 }],
  });
  assert.equal(formatValue(reals), "⟨3.0, 4.0⟩");
  assert.deepEqual(decodeValue(vector(int(1n), real(2.5))), {
    kind: "vector",
    components: [{ kind: "int", value: 1n }, { kind: "real", value: 2.5 }],
  });
  assert.deepEqual(decodeValue(vector()), { kind: "vector", components: [] });

  const text = create(ValueSchema, { kind: { case: "stringValue", value: "two" } });
  assert.throws(() => decodeValue(vector(real(1), text)), MalformedValueError);
  assert.throws(() => decodeValue(vector(create(ValueSchema, {}))), MalformedValueError);
});

test("a vector quantity carries one quantity per component, each with its unit", () => {
  const position = decodeValue(vectorQuantity(metres(3), metres(4)));
  assert.deepEqual(position, {
    kind: "vectorQuantity",
    components: [
      { magnitude: { kind: "real", value: 3 }, unit: "m", unitTerm: METRE },
      { magnitude: { kind: "real", value: 4 }, unit: "m", unitTerm: METRE },
    ],
  });
  assert.equal(formatValue(position), "⟨3.0[m], 4.0[m]⟩");

  // The units may differ per component, and a composed unit keeps its reduction.
  const speed = create(QuantitySchema, {
    magnitude: { case: "realMagnitude", value: 5 },
    unit: "m/s",
    unitTerm: create(UnitTermSchema, {
      scaleNum: 1,
      scaleDen: 1,
      factors: [
        create(UnitFactorSchema, { unitId: "SI::metre", exponent: 1 }),
        create(UnitFactorSchema, { unitId: "SI::second", exponent: -1 }),
      ],
    }),
  });
  const mixed = decodeValue(vectorQuantity(metres(1), speed));
  assert.equal(mixed.kind, "vectorQuantity");
  assert.equal(mixed.components[1]?.unit, "m/s");
  assert.deepEqual(mixed.components[1]?.unitTerm?.factors, [
    { unitId: "SI::metre", exponent: 1 },
    { unitId: "SI::second", exponent: -1 },
  ]);

  assert.throws(() => decodeValue(vectorQuantity()), MalformedValueError);
  assert.throws(() => encodeValue({ kind: "vectorQuantity", components: [] }), MalformedValueError);

  // A component without a magnitude is malformed, never read as zero;
  // so is a lone quantity without one.
  const noMagnitude = create(QuantitySchema, { unit: "m" });
  assert.throws(
    () => decodeValue(vectorQuantity(metres(3), noMagnitude)),
    (error: unknown) => error instanceof MalformedValueError && /no magnitude/.test(error.message),
  );
  assert.throws(
    () => decodeValue(create(ValueSchema, { kind: { case: "quantity", value: noMagnitude } })),
    MalformedValueError,
  );
});

const unitTerm = (scaleNum: number, ...factors: [string, number][]) =>
  create(UnitTermSchema, {
    scaleNum,
    scaleDen: 1,
    factors: factors.map(([unitId, exponent]) => create(UnitFactorSchema, { unitId, exponent })),
  });
const measurementRef = (unit: string, unitId: string, term?: ReturnType<typeof unitTerm>) =>
  create(ValueSchema, {
    kind: {
      case: "measurementRef",
      value: create(MeasurementRefSchema, { unit, unitId, ...(term === undefined ? {} : { unitTerm: term }) }),
    },
  });

test("a measurement reference keeps its unit, its reduction and the declaration it names", () => {
  const km = decodeValue(measurementRef("km", "SI::kilometre", unitTerm(1000, ["SI::metre", 1])));
  assert.deepEqual(km, {
    kind: "measurementRef",
    unit: "km",
    unitTerm: { scaleNum: 1000, scaleDen: 1, factors: [{ unitId: "SI::metre", exponent: 1 }] },
    unitId: "SI::kilometre",
  });
  assert.equal(formatValue(km), "km");

  // A unit an operation composed names no declaration, and none is invented.
  const speed = decodeValue(
    measurementRef("m/s", "", unitTerm(1, ["SI::metre", 1], ["SI::second", -1])),
  );
  assert.equal(speed.kind, "measurementRef");
  assert.equal(speed.unitId, undefined);
  assert.deepEqual(
    speed.unitTerm.factors.map((f) => f.exponent),
    [1, -1],
  );

  // A reference the service never wrote down renders from its reduction.
  const bare = decodeValue(
    measurementRef("", "", unitTerm(1000, ["SI::metre", 1], ["SI::second", -1])),
  );
  assert.equal(formatValue(bare), "1000\u00b7SI::metre\u00b7SI::second^-1");

  // Naming no unit, or a unit without its reduction, is malformed, at any depth.
  assert.throws(
    () => decodeValue(measurementRef("", "")),
    (error: unknown) => error instanceof MalformedValueError && /names no unit/.test(error.message),
  );
  assert.throws(
    () => decodeValue(measurementRef("km", "SI::kilometre")),
    (error: unknown) =>
      error instanceof MalformedValueError && /km has no reduction/.test(error.message),
  );
  const nested = create(ValueSchema, {
    kind: {
      case: "sequence",
      value: create(ValueSequenceSchema, { elements: [measurementRef("km", "")] }),
    },
  });
  assert.throws(() => decodeValue(nested), MalformedValueError);
});

test("a function is the calc it names, read against an object or none", () => {
  const fn = (calcId: string, selfId: bigint) =>
    create(ValueSchema, {
      kind: { case: "function", value: create(FunctionSchema, { calcId, selfId }) },
    });
  assert.deepEqual(decodeValue(fn("Demo::Sq", 0n)), { kind: "function", calcId: "Demo::Sq" });
  const scale = decodeValue(fn("Demo::Scaler::scale", 7n));
  assert.deepEqual(scale, { kind: "function", calcId: "Demo::Scaler::scale", selfId: 7n });
  assert.equal(formatValue(scale), "Demo::Scaler::scale");

  // A function naming no calc is malformed, at any depth.
  assert.throws(
    () => decodeValue(fn("", 0n)),
    (error: unknown) => error instanceof MalformedValueError && /names no calc/.test(error.message),
  );
  const nested = create(ValueSchema, {
    kind: { case: "sequence", value: create(ValueSequenceSchema, { elements: [fn("", 3n)] }) },
  });
  assert.throws(() => decodeValue(nested), MalformedValueError);
  assert.throws(() => encodeValue({ kind: "function", calcId: "" }), MalformedValueError);
});

test("encodeValue is the inverse of decodeValue, through the wire bytes", () => {
  const values: SysMLValue[] = [
    { kind: "int", value: 9007199254740993n },
    { kind: "real", value: 0.5 },
    { kind: "complex", value: { real: 1.5, imaginary: -2 } },
    { kind: "boolean", value: true },
    { kind: "string", value: "s" },
    { kind: "instance", id: 7n },
    { kind: "quantity", magnitude: { kind: "int", value: 3n }, unit: "m", unitTerm: METRE },
    { kind: "quantity", magnitude: { kind: "real", value: 2 }, unit: "" },
    { kind: "measurementRef", unit: "m", unitTerm: METRE, unitId: "SI::metre" },
    {
      kind: "measurementRef",
      unit: "m/s",
      unitTerm: {
        scaleNum: 1,
        scaleDen: 1,
        factors: [
          { unitId: "SI::metre", exponent: 1 },
          { unitId: "SI::second", exponent: -1 },
        ],
      },
    },
    { kind: "sequence", elements: [{ kind: "measurementRef", unit: "m", unitTerm: METRE, unitId: "SI::metre" }] },
    { kind: "function", calcId: "Demo::Sq" },
    { kind: "function", calcId: "Demo::Scaler::scale", selfId: 7n },
    { kind: "enum", value: { name: "red", literalId: "P::Color::red", enumerationId: "P::Color" } },
    { kind: "null", reason: "" },
    { kind: "unset" },
    {
      kind: "array",
      dimensions: [2n, 2n],
      elements: [
        { kind: "int", value: 1n },
        { kind: "real", value: 2.5 },
        { kind: "quantity", magnitude: { kind: "real", value: 3 }, unit: "m", unitTerm: METRE },
        { kind: "vector", components: [{ kind: "int", value: 1n }, { kind: "real", value: 2 }] },
      ],
    },
    { kind: "vector", components: [{ kind: "real", value: 3 }, { kind: "int", value: 4n }] },
    {
      kind: "vectorQuantity",
      components: [
        { magnitude: { kind: "real", value: 3 }, unit: "m", unitTerm: METRE },
        { magnitude: { kind: "int", value: 4n }, unit: "m", unitTerm: METRE },
      ],
    },
    { kind: "sequence", elements: [{ kind: "vector", components: [{ kind: "real", value: 1 }] }] },
  ];
  for (const value of values) {
    const bytes = toBinary(ValueSchema, encodeValue(value));
    assert.deepEqual(decodeValue(fromBinary(ValueSchema, bytes)), value, formatValue(value));
  }
  const sent = encodeValue({ kind: "vector", components: [{ kind: "real", value: 3 }, { kind: "int", value: 4n }] });
  assert.equal(sent.kind.case, "vector");
  assert.deepEqual(
    sent.kind.value.components.map((c) => c.kind.case),
    ["realValue", "intValue"],
  );
  assert.throws(() => encodeValue({ kind: "absent" }), MalformedValueError);
});

test("an enum literal keeps the enumeration that declares it", () => {
  const value = create(ValueSchema, {
    kind: {
      case: "enumLiteral",
      value: create(EnumLiteralSchema, {
        name: "Colour::red",
        literalId: "D::Colour::red",
        enumerationId: "D::Colour",
      }),
    },
  });
  assert.deepEqual(decodeValue(value), {
    kind: "enum",
    value: { name: "Colour::red", literalId: "D::Colour::red", enumerationId: "D::Colour" },
  });
});

test("a verdict holds, fails or is undecided, and always names its subject", () => {
  const held = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "Sample::Check",
      element: "Sample::Check",
      instanceId: 7n,
      holds: true,
    }),
  );
  assert.equal(held.kind, "holds");
  assert.deepEqual(held.subject, {
    kind: "constraint",
    elementId: "Sample::Check",
    element: "Sample::Check",
    instanceId: 7n,
  });

  const failed = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "Sample::Check",
      element: "Sample::Check",
      holds: false,
      condition: "mass < 1000",
    }),
  );
  assert.equal(failed.kind, "fails");
  assert.equal(failed.condition, "mass < 1000");

  // holds: false with an error is no answer at all, not a failed verdict.
  const undecided = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "Sample::Check",
      element: "Sample::Check",
      holds: false,
      error: "radius is unbound",
      failureReason: FailureReason.EVALUATION,
    }),
  );
  assert.equal(undecided.kind, "undecided");
  assert.equal(undecided.cause, "evaluation");
});

test("every failure reason has a name", () => {
  assert.equal(failureCause(FailureReason.UNSPECIFIED), "unspecified");
  assert.equal(failureCause(FailureReason.EVALUATION), "evaluation");
  assert.equal(failureCause(FailureReason.WRONG_KIND), "wrong_kind");
  assert.equal(failureCause(FailureReason.AMBIGUOUS_SUBJECT), "ambiguous_subject");
});
