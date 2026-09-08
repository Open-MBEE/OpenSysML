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
  TensorQuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
  ValueSchema,
  ValueSequenceSchema,
  ValueSetSchema,
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
  valuesEqual,
  type QuantityValue,
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

const setOf = (...elements: ReturnType<typeof int>[]) =>
  create(ValueSchema, { kind: { case: "set", value: create(ValueSetSchema, { elements }) } });
const tensor = (dimensions: bigint[], ...components: ReturnType<typeof metres>[]) =>
  create(ValueSchema, {
    kind: { case: "tensorQuantity", value: create(TensorQuantitySchema, { dimensions, components }) },
  });

test("a set is its elements, each once, in the order the service sent them", () => {
  const set = decodeValue(setOf(int(1n), int(2n), int(3n)));
  assert.deepEqual(set, { kind: "set", elements: [1n, 2n, 3n].map((value) => ({ kind: "int", value })) });
  assert.equal(formatValue(set), "{1, 2, 3}");

  // An empty set is a set of nothing, distinct from an empty sequence.
  assert.deepEqual(decodeValue(setOf()), { kind: "set", elements: [] });
  assert.equal(formatValue({ kind: "set", elements: [] }), "{}");

  // A set nests, and is nested, in place.
  const nested = decodeValue(setOf(setOf(int(1n)), setOf()));
  assert.deepEqual(nested, {
    kind: "set",
    elements: [{ kind: "set", elements: [{ kind: "int", value: 1n }] }, { kind: "set", elements: [] }],
  });
  const sequence = create(ValueSchema, {
    kind: { case: "sequence", value: create(ValueSequenceSchema, { elements: [setOf(int(1n)), int(2n)] }) },
  });
  assert.deepEqual(decodeValue(sequence), {
    kind: "sequence",
    elements: [{ kind: "set", elements: [{ kind: "int", value: 1n }] }, { kind: "int", value: 2n }],
  });

  // Sent in any order: the client does not reorder what a caller wrote.
  const sent = encodeValue({ kind: "set", elements: [3n, 1n, 2n].map((value) => ({ kind: "int", value })) });
  assert.equal(sent.kind.case, "set");
  assert.deepEqual(
    sent.kind.value.elements.map((e) => e.kind.value),
    [3n, 1n, 2n],
  );
});

const seqOf = (...elements: ReturnType<typeof int>[]) =>
  create(ValueSchema, { kind: { case: "sequence", value: create(ValueSequenceSchema, { elements }) } });
const bool = (value: boolean) => create(ValueSchema, { kind: { case: "boolValue", value } });
const quantity = (q: ReturnType<typeof metres>) => create(ValueSchema, { kind: { case: "quantity", value: q } });
const complex = (real: number, imaginary: number) =>
  create(ValueSchema, { kind: { case: "complex", value: create(ComplexSchema, { real, imaginary }) } });
const intMetres = (value: bigint) =>
  create(QuantitySchema, { ...metres(Number(value)), magnitude: { case: "intMagnitude", value } });

test("a set that lists a member twice is malformed, judged by value", () => {
  const twice = [
    setOf(int(1n), int(2n), int(1n)),
    setOf(int(1n), real(1)),
    setOf(real(1.5), complex(1.5, 0)),
    setOf(int(2n), complex(2, 0)),
    setOf(quantity(metres(1)), quantity(intMetres(1n))),
    setOf(seqOf(int(1n), int(2n)), seqOf(int(1n), int(2n))),
    setOf(setOf(int(1n), int(2n)), setOf(int(2n), int(1n))),
    setOf(setOf(), setOf()),
    setOf(quantity(metres(1)), quantity(metres(1))),
    setOf(bool(true), seqOf(), bool(true)),
    setOf(array([1n], int(1n)), array([1n], int(1n))),
  ];
  for (const set of twice) {
    assert.throws(() => decodeValue(set), {
      name: "MalformedValueError",
      message: /^a set lists a member twice: /,
    });
  }

  // Members that merely look alike are distinct: an int is not a real of another
  // value, even one it would round to; a sequence's order counts; a sequence is
  // never a set of the same elements.
  const alike = [
    setOf(int(1n), real(1.5)),
    setOf(int(2n ** 53n + 1n), real(2 ** 53)),
    setOf(real(1), complex(1, 1)),
    setOf(bool(true), int(1n)),
    setOf(seqOf(int(1n), int(2n)), seqOf(int(2n), int(1n))),
    setOf(seqOf(int(1n)), setOf(int(1n))),
    setOf(setOf(), setOf(setOf())),
    setOf(array([1n, 2n], int(1n), int(2n)), array([2n, 1n], int(1n), int(2n))),
    setOf(quantity(metres(1)), quantity(metres(2))),
  ];
  for (const set of alike) {
    const decoded = decodeValue(set);
    assert.equal(decoded.kind, "set");
    assert.equal(decoded.elements.length, 2);
    assert.ok(!valuesEqual(decoded.elements[0], decoded.elements[1]));
  }

  // valuesEqual is the membership test itself: sets by membership, sequences in order.
  const a = decodeValue(setOf(int(1n), setOf(int(2n), int(3n))));
  const b = decodeValue(setOf(setOf(int(3n), int(2n)), int(1n)));
  assert.ok(valuesEqual(a, b));
  assert.ok(!valuesEqual(a, decodeValue(setOf(int(1n), setOf(int(2n))))));
  assert.ok(valuesEqual({ kind: "null", reason: "x" }, { kind: "null", reason: "y" }));
  assert.ok(!valuesEqual({ kind: "unset" }, { kind: "absent" }));
});

test("valuesEqual judges numbers by value, as the service does", () => {
  const i = (value: bigint): SysMLValue => ({ kind: "int", value });
  const r = (value: number): SysMLValue => ({ kind: "real", value });
  const c = (real: number, imaginary: number): SysMLValue => ({ kind: "complex", value: { real, imaginary } });
  const cases: [SysMLValue, SysMLValue, boolean][] = [
    [i(1n), r(1), true],
    [i(1n), r(1.5), false],
    [i(2n ** 53n + 1n), r(2 ** 53), false],
    [i(2n ** 53n), r(2 ** 53), true],
    [i(2n ** 63n - 1n), r(2 ** 63), false],
    [i(-(2n ** 63n)), r(-(2 ** 63)), true],
    [i(0n), r(Infinity), false],
    [i(0n), r(-0), true],
    [r(2.5), c(2.5, 0), true],
    [i(2n), c(2, 0), true],
    [i(2n), c(2, 1), false],
    [c(2, 1), c(2, 1), true],
    [i(1n), { kind: "boolean", value: true }, false],
    [i(1n), { kind: "string", value: "1" }, false],
    [decodeValue(quantity(intMetres(1n))), decodeValue(quantity(metres(1))), true],
    [decodeValue(quantity(intMetres(1n))), decodeValue(quantity(metres(2))), false],
    [
      { kind: "vector", components: [{ kind: "int", value: 1n }, { kind: "real", value: 2 }] },
      { kind: "vector", components: [{ kind: "real", value: 1 }, { kind: "int", value: 2n }] },
      true,
    ],
    [{ kind: "set", elements: [i(1n), r(2.5)] }, { kind: "set", elements: [r(2.5), r(1)] }, true],
    [{ kind: "set", elements: [i(2n ** 53n + 1n)] }, { kind: "set", elements: [r(2 ** 53)] }, false],
    // A set assembled with a member listed twice equals only a set of the same members.
    [{ kind: "set", elements: [i(1n), i(1n)] }, { kind: "set", elements: [i(1n), i(2n)] }, false],
    [{ kind: "set", elements: [i(1n), i(1n), i(2n)] }, { kind: "set", elements: [i(1n), i(2n), i(2n)] }, true],
    [{ kind: "set", elements: [i(1n), r(1)] }, { kind: "set", elements: [i(1n)] }, true],
  ];
  for (const [a, b, want] of cases) {
    assert.equal(valuesEqual(a, b), want, `${formatValue(a)} vs ${formatValue(b)}`);
    assert.equal(valuesEqual(b, a), want, `${formatValue(b)} vs ${formatValue(a)}`);
  }
});

test("valuesEqual judges quantities over their base units, as the service does", () => {
  const term = (scaleNum: number, scaleDen: number, ...factors: [string, number][]) => ({
    scaleNum,
    scaleDen,
    factors: factors.map(([unitId, exponent]) => ({ unitId, exponent })),
  });
  const q = (
    magnitude: bigint | number,
    unit: string,
    unitTerm?: ReturnType<typeof term>,
  ): { kind: "quantity" } & QuantityValue => ({
    kind: "quantity",
    magnitude: typeof magnitude === "bigint" ? { kind: "int", value: magnitude } : { kind: "real", value: magnitude },
    unit,
    ...(unitTerm === undefined ? {} : { unitTerm }),
  });
  const m = (v: bigint | number) => q(v, "m", term(1, 1, ["SI::metre", 1]));
  const cm = (v: bigint | number) => q(v, "cm", term(1, 100, ["SI::metre", 1]));
  const cmDecimal = (v: bigint | number) => q(v, "cm", term(0.01, 1, ["SI::metre", 1]));
  const km = (v: bigint | number) => q(v, "km", term(1000, 1, ["SI::metre", 1]));
  const s = (v: bigint | number) => q(v, "s", term(1, 1, ["SI::second", 1]));
  const kmh = (v: bigint | number) => q(v, "km/h", term(1000, 3600, ["SI::metre", 1], ["SI::second", -1]));
  const ms = (v: bigint | number) => q(v, "m/s", term(1, 1, ["SI::second", -1], ["SI::metre", 1]));
  const huge = 2n ** 53n + 1n;
  const cases: [SysMLValue, SysMLValue, boolean][] = [
    [m(1n), cm(100n), true],
    [m(1n), cmDecimal(100n), true],
    [m(1), cm(100), true],
    [m(1n), cm(100), true],
    [m(1n), cm(1n), false],
    [m(1000n), km(1n), true],
    [m(1000), km(1n), true],
    [m(1001n), km(1n), false],
    [m(1000n * huge), km(huge), true],
    [m(1000n * huge + 1n), km(huge), false],
    [m(1n), s(1n), false],
    [kmh(5.4), ms(1.5), true],
    [kmh(36n), ms(10n), true],
    [kmh(36n), ms(11n), false],
    [kmh(1n), m(1n), false],
    [m(1n), q(1n, "m", term(1, 1, ["SI::metre", 1], ["SI::second", -1], ["SI::second", 1])), true],
    [q(1n, "m"), q(1, "m"), true],
    [q(1n, "m"), q(100n, "cm"), false],
    [q(1n, "m"), m(1n), false],
    [q(0n, "x", term(0, 1, ["SI::metre", 1])), m(0n), false],
    [{ kind: "set", elements: [m(1n), km(2n)] }, { kind: "set", elements: [m(2000n), cm(100n)] }, true],
    [{ kind: "set", elements: [m(1n), km(2n)] }, { kind: "set", elements: [m(2000n), cm(1n)] }, false],
    [
      { kind: "vectorQuantity", components: [m(1n), km(1n)] },
      { kind: "vectorQuantity", components: [cm(100n), m(1000n)] },
      true,
    ],
    [
      { kind: "tensorQuantity", dimensions: [1n, 1n], components: [m(1n)] },
      { kind: "tensorQuantity", dimensions: [1n, 1n], components: [cm(100n)] },
      true,
    ],
  ];
  for (const [a, b, want] of cases) {
    assert.equal(valuesEqual(a, b), want, `${formatValue(a)} vs ${formatValue(b)}`);
    assert.equal(valuesEqual(b, a), want, `${formatValue(b)} vs ${formatValue(a)}`);
  }

  // Membership and duplicate detection follow: a set holding 1 m holds 100 cm,
  // and one listing both is refused, arriving or about to be sent.
  assert.throws(() => encodeValue({ kind: "set", elements: [m(1n), cm(100n)] }), {
    name: "MalformedValueError",
    message: /^a set lists a member twice: /,
  });
  const sent = decodeValue(encodeValue({ kind: "set", elements: [m(1n), cm(1n)] }));
  assert.equal(sent.kind, "set");
  assert.equal(sent.elements.length, 2);
  assert.throws(() => decodeValue(encodeValue({ kind: "set", elements: [m(1000), km(1n)] })), {
    name: "MalformedValueError",
    message: /^a set lists a member twice: /,
  });
});

test("valuesEqual judges measurement references by their reduction, as the service does", () => {
  const ref = (
    unit: string,
    unitId: string | undefined,
    scaleNum: number,
    scaleDen: number,
    ...factors: [string, number][]
  ): SysMLValue => ({
    kind: "measurementRef",
    unit,
    unitTerm: { scaleNum, scaleDen, factors: factors.map(([unitId, exponent]) => ({ unitId, exponent })) },
    ...(unitId === undefined ? {} : { unitId }),
  });
  const namedSpeed = ref("SI::'m/s'", "SI::'m/s'", 1, 1, ["SI::metre", 1], ["SI::second", -1]);
  const composedSpeed = ref("m / s", undefined, 1, 1, ["SI::second", -1], ["SI::metre", 1]);
  const km = ref("km", "SI::kilometre", 1000, 1, ["SI::metre", 1]);
  const rad = ref("rad", "SI::radian", 1, 1);
  const sr = ref("sr", "SI::steradian", 1, 1);
  const ratio = ref("m / m", undefined, 1, 1, ["SI::metre", 1], ["SI::metre", -1]);
  const cases: [SysMLValue, SysMLValue, boolean][] = [
    [namedSpeed, composedSpeed, true],
    [km, ref("km", "SI::km", 2000, 2, ["SI::metre", 1]), true],
    [ref("km/m", undefined, 1000, 1), ref("m/mm", undefined, 1, 0.001), true],
    [km, ref("m", "SI::metre", 1, 1, ["SI::metre", 1]), false],
    [ref("m", "SI::metre", 1, 1, ["SI::metre", 1]), ref("s", "SI::second", 1, 1, ["SI::second", 1]), false],
    [ref("x", undefined, 0, 1, ["SI::metre", 1]), ref("x", undefined, 0, 1, ["SI::metre", 1]), false],
    [rad, sr, false],
    [rad, ref("SI::rad", "SI::radian", 1, 1, ["SI::metre", 1], ["SI::metre", -1]), true],
    [rad, ratio, false],
    [ratio, ref("", undefined, 1, 1), true],
    [{ kind: "set", elements: [namedSpeed, rad] }, { kind: "set", elements: [rad, composedSpeed] }, true],
    [{ kind: "set", elements: [namedSpeed, rad] }, { kind: "set", elements: [sr, composedSpeed] }, false],
  ];
  for (const [a, b, want] of cases) {
    assert.equal(valuesEqual(a, b), want, `${formatValue(a)} vs ${formatValue(b)}`);
    assert.equal(valuesEqual(b, a), want, `${formatValue(b)} vs ${formatValue(a)}`);
  }
  for (const twice of [[namedSpeed, composedSpeed], [km, ref("km", "SI::km", 2000, 2, ["SI::metre", 1])]]) {
    assert.throws(() => encodeValue({ kind: "set", elements: twice }), {
      name: "MalformedValueError",
      message: /^a set lists a member twice: /,
    });
  }
  assert.throws(() => decodeValue(setOf(encodeValue(rad), encodeValue(ref("SI::rad", "SI::radian", 1, 1)))), {
    name: "MalformedValueError",
    message: /^a set lists a member twice: /,
  });
  const sent = decodeValue(encodeValue({ kind: "set", elements: [rad, sr] }));
  assert.equal(sent.kind, "set");
  assert.equal(sent.elements.length, 2);
});

test("valuesEqual judges enumeration literals by their literalId, as the service does", () => {
  const lit = (literalId: string, enumerationId = "", name = ""): SysMLValue => ({
    kind: "enum",
    value: { literalId, enumerationId, name },
  });
  const red = lit("D::Color::red", "D::Color", "Color::red");
  const same = lit("D::Color::red", "E::Palette", "red");
  const green = lit("D::Color::green", "D::Color", "Color::red");
  assert.equal(valuesEqual(red, same), true);
  assert.equal(valuesEqual(red, green), false);
  assert.equal(
    valuesEqual({ kind: "set", elements: [red, green] }, { kind: "set", elements: [lit("D::Color::green"), same] }),
    true,
  );
  assert.throws(() => encodeValue({ kind: "set", elements: [red, same] }), {
    name: "MalformedValueError",
    message: /^a set lists a member twice: /,
  });
  const sent = decodeValue(encodeValue({ kind: "set", elements: [red, green] }));
  assert.equal(sent.kind, "set");
  assert.equal(sent.elements.length, 2);
});

test("valuesEqual judges functions by the calc read against its object, as the service does", () => {
  const sq: SysMLValue = { kind: "function", calcId: "M::Sq" };
  const cube: SysMLValue = { kind: "function", calcId: "M::Cube" };
  const scale: SysMLValue = { kind: "function", calcId: "M::scale", selfId: 7n };
  assert.equal(valuesEqual(sq, { kind: "function", calcId: "M::Sq" }), true);
  assert.equal(valuesEqual(sq, cube), false);
  assert.equal(valuesEqual(scale, { kind: "function", calcId: "M::scale", selfId: 8n }), false);
  assert.equal(valuesEqual(scale, { kind: "function", calcId: "M::scale" }), false);
  assert.equal(valuesEqual(sq, { kind: "string", value: "M::Sq" }), false);
  assert.equal(
    valuesEqual({ kind: "set", elements: [sq, cube] }, { kind: "set", elements: [cube, sq] }),
    true,
  );
  assert.throws(() => encodeValue({ kind: "set", elements: [sq, cube, { kind: "function", calcId: "M::Sq" }] }), {
    name: "MalformedValueError",
    message: /^a set lists a member twice: /,
  });
});

test("a set assembled with a member listed twice is refused before it is sent", () => {
  const i = (value: bigint): SysMLValue => ({ kind: "int", value });
  const r = (value: number): SysMLValue => ({ kind: "real", value });
  const set = (...elements: SysMLValue[]): SysMLValue => ({ kind: "set", elements });
  const seq = (...elements: SysMLValue[]): SysMLValue => ({ kind: "sequence", elements });
  for (const twice of [
    set(i(1n), i(2n), i(1n)),
    set(i(1n), r(1)),
    set(r(1.5), { kind: "complex", value: { real: 1.5, imaginary: 0 } }),
    set(seq(i(1n), i(2n)), seq(i(1n), i(2n))),
    set(set(i(1n), i(2n)), set(i(2n), i(1n))),
    set(i(3n), set(i(1n), i(1n))),
  ]) {
    assert.throws(() => encodeValue(twice), {
      name: "MalformedValueError",
      message: /^a set lists a member twice: /,
    });
  }
  for (const alike of [
    set(i(2n ** 53n + 1n), r(2 ** 53)),
    set(i(1n), { kind: "boolean", value: true }),
    set(seq(i(1n)), set(i(1n))),
    set(seq(i(1n), i(2n)), seq(i(2n), i(1n))),
    set(set(), set(set())),
  ]) {
    assert.deepEqual(decodeValue(encodeValue(alike)), alike);
  }
});

test("a tensor quantity keeps its rank, its shape and its row-major components", () => {
  const cube = decodeValue(tensor([2n, 2n, 2n], ...[1, 2, 3, 4, 5, 6, 7, 8].map(metres)));
  assert.equal(cube.kind, "tensorQuantity");
  assert.deepEqual(cube.dimensions, [2n, 2n, 2n]);
  assert.equal(cube.components.length, 8);
  assert.deepEqual(cube.components[7], { magnitude: { kind: "real", value: 8 }, unit: "m", unitTerm: METRE });
  assert.equal(formatValue(cube), "Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0][m]");

  // A rank-one tensor stays a tensor, never a vector quantity.
  const line = decodeValue(tensor([2n], metres(1), metres(2)));
  assert.equal(line.kind, "tensorQuantity");
  assert.deepEqual(line.dimensions, [2n]);

  // Components with differing units each show their own.
  const speed = create(QuantitySchema, {
    magnitude: { case: "intMagnitude", value: 5n },
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
  assert.equal(formatValue(decodeValue(tensor([1n, 2n], metres(1), speed))), "Tensor(1, 2)[1.0[m], 5[m/s]]");

  // Shape and components must agree, both ways, and every dimension is positive.
  const malformed = (message: RegExp) => (error: unknown) =>
    error instanceof MalformedValueError && message.test(error.message);
  assert.throws(() => decodeValue(tensor([2n, 2n], metres(1), metres(2), metres(3))), malformed(/holds 3/));
  assert.throws(() => decodeValue(tensor([2n], metres(1), metres(2), metres(3))), malformed(/holds 3/));
  assert.throws(() => decodeValue(tensor([0n])), malformed(/not positive/));
  assert.throws(() => decodeValue(tensor([-1n], metres(1))), malformed(/not positive/));
  assert.throws(() => decodeValue(tensor([], metres(1), metres(2))), malformed(/holds 2/));
  assert.throws(
    () => encodeValue({ kind: "tensorQuantity", dimensions: [2n, 2n], components: [] }),
    malformed(/holds 0/),
  );
  assert.throws(
    () => encodeValue({ kind: "tensorQuantity", dimensions: [0n], components: [] }),
    malformed(/not positive/),
  );

  // A component without a magnitude is malformed, never read as zero.
  const noMagnitude = create(QuantitySchema, { unit: "m" });
  assert.throws(() => decodeValue(tensor([1n], noMagnitude)), malformed(/no magnitude/));
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
    { kind: "set", elements: [] },
    {
      kind: "set",
      elements: [
        { kind: "int", value: 3n },
        { kind: "string", value: "a" },
        { kind: "set", elements: [{ kind: "boolean", value: true }] },
        { kind: "sequence", elements: [{ kind: "int", value: 1n }] },
      ],
    },
    {
      kind: "tensorQuantity",
      dimensions: [2n, 1n, 2n],
      components: [
        { magnitude: { kind: "real", value: 1 }, unit: "m", unitTerm: METRE },
        { magnitude: { kind: "int", value: 2n }, unit: "m", unitTerm: METRE },
        { magnitude: { kind: "real", value: 3 }, unit: "m", unitTerm: METRE },
        { magnitude: { kind: "int", value: 4n }, unit: "m", unitTerm: METRE },
      ],
    },
    {
      kind: "array",
      dimensions: [1n],
      elements: [{ kind: "set", elements: [{ kind: "int", value: 1n }] }],
    },
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

test("only an asserted infinity arm is the unbounded value", () => {
  const asserted = create(ValueSchema, { kind: { case: "infinity", value: true } });
  assert.deepEqual(decodeValue(asserted), { kind: "infinity" });
  const denied = create(ValueSchema, { kind: { case: "infinity", value: false } });
  assert.throws(() => decodeValue(denied), MalformedValueError);
  const nested = create(ValueSchema, {
    kind: {
      case: "sequence",
      value: create(ValueSequenceSchema, { elements: [denied] }),
    },
  });
  assert.throws(() => decodeValue(nested), MalformedValueError);
});
