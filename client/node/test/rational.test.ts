// Exact Rationals: the rational arms, the double a Rational falls back to, and
// the rational_values capability that gates sending one.

import assert from "node:assert/strict";
import { test } from "node:test";
import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import {
  QuantitySchema,
  RationalSchema,
  ValueSchema,
  VectorSchema,
} from "../src/generated/sysml_pb.js";
import {
  CAPABILITY_RATIONAL_VALUES,
  MalformedValueError,
  MissingCapabilityError,
  ServerInfo,
  buildBindings,
  decodeValue,
  encodeValue,
  formatRational,
  formatValue,
  rational,
  rationalToNumber,
  rationalsAsReals,
  toValue,
  valuesEqual,
  type SysMLValue,
} from "../src/node/index.js";
import { bindingRationalsAsReals } from "../src/core/document.js";
import { fakeConnection } from "./support/fake.js";

const third = rational(1n, 3n);
const tenth = rational(1n, 10n);

function roundTrip(value: SysMLValue): SysMLValue {
  return decodeValue(fromBinary(ValueSchema, toBinary(ValueSchema, encodeValue(value))));
}

test("a Rational no double holds travels as rational_value and reads back exactly", () => {
  for (const value of [third, tenth, rational(-7n, 3n), rational(1n, 3n ** 200n), rational(2n ** 60n + 1n)]) {
    const wire = encodeValue(value);
    assert.equal(wire.kind.case, "rationalValue");
    assert.deepEqual(roundTrip(value), value);
  }
});

test("a Rational a double holds is sent as rational_value", () => {
  for (const value of [rational(1n, 2n), rational(-3n, 4n), rational(5n), rational(0n)]) {
    assert.deepEqual(encodeValue(value).kind, {
      case: "rationalValue",
      value: create(RationalSchema, { numerator: value.numerator.toString(), denominator: value.denominator.toString() }),
    });
  }
});

test("a Rational a double holds is that double for a service without rational_values", () => {
  const asReal = (value: SysMLValue) => {
    const wire = encodeValue(value);
    rationalsAsReals(wire);
    return wire;
  };
  for (const [value, double] of [
    [rational(1n, 2n), 0.5],
    [rational(-3n, 4n), -0.75],
    [rational(5n), 5],
    [rational(1n, 2n ** 1074n), 2 ** -1074],
    [rational(2n ** 1023n), 2 ** 1023],
  ] as const) {
    assert.deepEqual(asReal(value).kind, { case: "realValue", value: double });
  }
  assert.equal(asReal(rational(1n, 2n ** 1075n)).kind.case, "rationalValue");
  assert.equal(asReal(rational(2n ** 1024n)).kind.case, "rationalValue");
  assert.equal(asReal(rational(2n ** 53n + 1n)).kind.case, "rationalValue");
  assert.equal(asReal(third).kind.case, "rationalValue");
});

test("rational reduces over a positive denominator and refuses a zero one", () => {
  assert.deepEqual(rational(6n, -4n), { kind: "rational", numerator: -3n, denominator: 2n });
  assert.throws(() => rational(1n, 0n), RangeError);
});

test("a rational_value not in canonical form is malformed", () => {
  for (const [numerator, denominator] of [
    ["2", "6"],
    ["1", "-3"],
    ["1", "0"],
    ["1", "2"],
    ["x", "3"],
  ]) {
    const wire = create(ValueSchema, {
      kind: { case: "rationalValue", value: create(RationalSchema, { numerator, denominator }) },
    });
    assert.throws(() => decodeValue(wire), MalformedValueError);
  }
});

test("a terminating Rational prints as its decimal, any other as numerator/denominator", () => {
  assert.equal(formatValue(tenth), "0.1");
  assert.equal(formatValue(rational(-1n, 8n)), "-0.125");
  assert.equal(formatValue(rational(5n)), "5.0");
  assert.equal(formatValue(third), "1/3");
  assert.equal(formatRational(rational(-7n, 3n)), "-7/3");
});

test("a Rational equals the number it is, Integer and Real alike, never rounding", () => {
  assert.ok(valuesEqual(rational(4n, 2n), { kind: "int", value: 2n }));
  assert.ok(valuesEqual(rational(1n, 2n), { kind: "real", value: 0.5 }));
  assert.ok(!valuesEqual(tenth, { kind: "real", value: 0.1 }));
  assert.ok(!valuesEqual(third, rational(1n, 4n)));
  assert.ok(valuesEqual(third, rational(2n, 6n)));
  assert.ok(!valuesEqual(third, { kind: "real", value: Infinity }));
});

test("a Rational read as a Real is the double nearest it", () => {
  assert.equal(rationalToNumber(third), 1 / 3);
  assert.equal(rationalToNumber(tenth), 0.1);
  assert.equal(rationalToNumber(rational(-2n, 3n)), -2 / 3);
  assert.equal(rationalToNumber(rational(10n ** 400n, 3n * 10n ** 399n)), 10 / 3);
  assert.equal(rationalToNumber(rational(2n ** 1100n, 3n)), Infinity);
});

test("a quantity, a vector and a document value keep a Rational exactly", () => {
  const quantity: SysMLValue = { kind: "quantity", magnitude: third, unit: "kg" };
  const wire = encodeValue(quantity);
  assert.equal(wire.kind.case, "quantity");
  assert.equal(wire.kind.value.magnitude.case, "rationalMagnitude");
  assert.deepEqual(roundTrip(quantity), quantity);
  const half: SysMLValue = { kind: "quantity", magnitude: rational(1n, 2n), unit: "kg" };
  const halfWire = encodeValue(half);
  assert.equal(halfWire.kind.case === "quantity" ? halfWire.kind.value.magnitude.case : "", "rationalMagnitude");
  rationalsAsReals(halfWire);
  assert.equal(halfWire.kind.case === "quantity" ? halfWire.kind.value.magnitude.case : "", "realMagnitude");

  const vector: SysMLValue = { kind: "vector", components: [third, { kind: "int", value: 2n }] };
  assert.deepEqual(roundTrip(vector), vector);
  assert.throws(
    () =>
      decodeValue(
        create(ValueSchema, {
          kind: {
            case: "vector",
            value: create(VectorSchema, {
              components: [
                create(ValueSchema, {
                  kind: { case: "rationalValue", value: create(RationalSchema, { numerator: "4", denominator: "6" }) },
                }),
              ],
            }),
          },
        }),
      ),
    MalformedValueError,
  );

  const [bound] = buildBindings({ x: third });
  assert.equal(bound.values[0].kind.case, "rationalValue");
  const [boundHalf] = buildBindings({ x: rational(1n, 2n) });
  assert.equal(boundHalf.values[0].kind.case, "rationalValue");
  const [boundQuantity] = buildBindings({ m: { kind: "quantity", magnitude: tenth, unit: "kg" } });
  const magnitude = boundQuantity.values[0].kind;
  assert.equal(magnitude.case === "quantity" ? magnitude.value.magnitude.case : "", "rationalMagnitude");
});

test("a Rational no double holds is refused before the call by a service without rational_values", () => {
  const older = new ServerInfo({
    version: "v",
    capabilities: ["structured_values", "tensor_values", "set_values", "big_int_values"],
    answered: true,
    origin: "test",
  });
  const unitTerm = { scaleNum: 1, scaleDen: 1, factors: [{ unitId: "SI::kg", exponent: 1 }] };
  for (const input of [
    third,
    [1n, third],
    { kind: "set", elements: [third] },
    { kind: "quantity", magnitude: third, unit: "" },
    { kind: "vector", components: [third] },
    { kind: "vectorQuantity", components: [{ magnitude: third, unit: "", unitTerm }] },
    { kind: "tensorQuantity", dimensions: [1n], components: [{ magnitude: third, unit: "", unitTerm }] },
  ] as const) {
    assert.throws(
      () => toValue(input as never, older),
      (error: unknown) => {
        assert.ok(error instanceof MissingCapabilityError);
        assert.equal(error.capability, CAPABILITY_RATIONAL_VALUES);
        return true;
      },
    );
  }
  assert.deepEqual(toValue(rational(1n, 2n), older).kind, { case: "realValue", value: 0.5 });
  const nested = toValue([rational(1n, 2n)], older);
  assert.deepEqual(
    nested.kind.case === "sequence" ? nested.kind.value.elements[0].kind : undefined,
    { case: "realValue", value: 0.5 },
  );
  const current = new ServerInfo({
    version: "v",
    capabilities: [CAPABILITY_RATIONAL_VALUES],
    answered: true,
    origin: "test",
  });
  assert.equal(toValue(third, current).kind.case, "rationalValue");
  assert.equal(toValue(rational(1n, 4n), current).kind.case, "rationalValue");
  assert.equal(toValue(0.25, current).kind.case, "realValue");
});

test("a document query bound to a Rational no double holds needs rational_values", async () => {
  const conn = await fakeConnection(["document_query"]);
  for (const bound of [third, { kind: "quantity", magnitude: third, unit: "kg" }] as const) {
    await assert.rejects(
      () => conn.runDocumentQuery("hash", "Q::q", { x: bound }),
      (error: unknown) => {
        assert.ok(error instanceof MissingCapabilityError);
        assert.equal(error.capability, CAPABILITY_RATIONAL_VALUES);
        return true;
      },
    );
  }
});

test("a quantity magnitude reads rational_magnitude", () => {
  const wire = create(ValueSchema, {
    kind: {
      case: "quantity",
      value: create(QuantitySchema, {
        magnitude: { case: "rationalMagnitude", value: create(RationalSchema, { numerator: "-1", denominator: "3" }) },
        unit: "m",
      }),
    },
  });
  assert.deepEqual(decodeValue(wire), { kind: "quantity", magnitude: rational(-1n, 3n), unit: "m" });
});

test("a document binding a double holds reaches a service without rational_values as that double", () => {
  const [bound] = buildBindings({ x: rational(1n, 2n) });
  bindingRationalsAsReals(bound);
  assert.deepEqual(bound.values[0].kind, { case: "realValue", value: 0.5 });
  const [quantity] = buildBindings({ m: { kind: "quantity", magnitude: rational(1n, 4n), unit: "kg" } });
  bindingRationalsAsReals(quantity);
  const magnitude = quantity.values[0].kind;
  assert.deepEqual(magnitude.case === "quantity" ? magnitude.value.magnitude : undefined, { case: "realMagnitude", value: 0.25 });
});
