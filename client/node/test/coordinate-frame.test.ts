// Coordinate frames and transformations, read as the structures they are.

import assert from "node:assert/strict";
import { test } from "node:test";
import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { ValueSchema, type Value } from "../src/generated/sysml_pb.js";
import { MalformedValueError } from "../src/core/errors.js";
import { CAPABILITY_COORDINATE_FRAME_VALUES } from "../src/core/capabilities.js";
import {
  decodeValue,
  encodeValue,
  formatValue,
  valuesEqual,
} from "../src/core/values.js";

const millimetre = {
  unit: "mm",
  unitId: "SI::millimetre",
  unitTerm: {
    scaleNum: 1,
    scaleDen: 1000,
    factors: [{ unitId: "SI::metre", exponent: 1 }],
  },
};

function frame(name: string, extra: Record<string, unknown> = {}) {
  return {
    name,
    frameId: `Frames::${name}`,
    typeId: "ISQSpaceTime::CartesianSpatial3dCoordinateFrame",
    dimensions: [3n],
    axes: [millimetre, millimetre, millimetre],
    ...extra,
  };
}

function length(real: number) {
  return {
    magnitude: { case: "realMagnitude" as const, value: real },
    unit: "mm",
    unitTerm: millimetre.unitTerm,
  };
}

function vector(...reals: number[]) {
  return {
    kind: {
      case: "vectorQuantity" as const,
      value: { components: reals.map(length) },
    },
  };
}

// Through the wire encoding, so what is read is what a service's bytes hold.
function wire(init: Parameters<typeof create<typeof ValueSchema>>[1]): Value {
  return fromBinary(
    ValueSchema,
    toBinary(ValueSchema, create(ValueSchema, init)),
  );
}

test("a frame reads with its identity and axes", () => {
  const value = decodeValue(
    wire({
      kind: {
        case: "coordinateFrame",
        value: frame("datum", { instanceId: 4n }),
      },
    }),
  );
  assert.ok(value.kind === "coordinateFrame");
  assert.equal(value.frameId, "Frames::datum");
  assert.equal(value.typeId, "ISQSpaceTime::CartesianSpatial3dCoordinateFrame");
  assert.equal(value.instanceId, 4n);
  assert.deepEqual(value.dimensions, [3n]);
  assert.deepEqual(
    value.axes.map((axis) => axis.unitId),
    ["SI::millimetre", "SI::millimetre", "SI::millimetre"],
  );
  assert.equal(value.transformation, undefined);
  assert.equal(value.scale, undefined);
  assert.equal(formatValue(value), "datum [mm, mm, mm]");
  // Something to read, never to send.
  assert.throws(() => encodeValue(value), MalformedValueError);
});

test("a posed frame carries its sequence, its source and its pose", () => {
  const posed = wire({
    kind: {
      case: "coordinateFrame",
      value: frame("lbcf", {
        transformation: {
          name: "transformation",
          source: frame("datum"),
          shape: {
            case: "sequence",
            value: {
              steps: [
                {
                  step: { case: "translation", value: vector(10, 0, 0) },
                  instanceId: 7n,
                },
                {
                  step: {
                    case: "rotation",
                    value: {
                      axisDirection: vector(0, 0, 1),
                      angle: {
                        magnitude: { case: "intMagnitude", value: 180n },
                        unit: "°",
                        unitTerm: { scaleNum: 1, scaleDen: 1 },
                      },
                      isIntrinsic: true,
                    },
                  },
                  instanceId: 8n,
                },
              ],
            },
          },
          pose: { origin: [10, 0, 0], basis: [-1, 0, 0, 0, -1, 0, 0, 0, 1] },
        },
      }),
    },
  });
  const value = decodeValue(posed);
  assert.ok(
    value.kind === "coordinateFrame" && value.transformation !== undefined,
  );
  const t = value.transformation;
  assert.equal(t.source?.frameId, "Frames::datum");
  assert.equal(t.target, undefined);
  assert.ok(t.shape?.kind === "sequence");
  const [shift, turn] = t.shape.steps;
  assert.ok(
    shift.kind === "translation" && shift.translation.kind === "vectorQuantity",
  );
  assert.equal(shift.instanceId, 7n);
  assert.ok(turn.kind === "rotation" && turn.intrinsic);
  assert.deepEqual(turn.angle.magnitude, { kind: "int", value: 180n });
  assert.deepEqual(t.pose, {
    origin: [10, 0, 0],
    basis: [-1, 0, 0, 0, -1, 0, 0, 0, 1],
  });

  assert.ok(valuesEqual(value, decodeValue(posed)));
  const datum = decodeValue(
    wire({ kind: { case: "coordinateFrame", value: frame("datum") } }),
  );
  assert.ok(!valuesEqual(value, datum));
  assert.ok(!valuesEqual(value, { kind: "null", reason: "" }));
});

test("a transformation reads its endpoints and the shape it is stated in", () => {
  const placed = decodeValue(
    wire({
      kind: {
        case: "coordinateTransformation",
        value: {
          name: "placement",
          source: frame("datum"),
          target: frame("placed"),
          shape: {
            case: "placement",
            value: {
              origin: vector(1, 2, 3),
              basisDirections: [vector(1, 0, 0)],
            },
          },
        },
      },
    }),
  );
  assert.ok(
    placed.kind === "coordinateTransformation" &&
      placed.shape?.kind === "placement",
  );
  assert.equal(placed.shape.basisDirections.length, 1);
  assert.equal(placed.pose, undefined);
  assert.equal(formatValue(placed), "placement (datum → placed)");
  assert.throws(() => encodeValue(placed), MalformedValueError);

  const affine = decodeValue(
    wire({
      kind: {
        case: "coordinateTransformation",
        value: {
          shape: {
            case: "affine",
            value: {
              rotation: [1, 0, 0, 0, 1, 0, 0, 0, 1],
              translation: [1, 2, 3],
            },
          },
        },
      },
    }),
  );
  assert.ok(
    affine.kind === "coordinateTransformation" &&
      affine.shape?.kind === "affine",
  );
  assert.deepEqual(affine.shape.translation, [1, 2, 3]);
  assert.equal(formatValue(affine), " (? → ?)");
  assert.ok(!valuesEqual(affine, placed));
});

test("a measurement scale reads its unit and its mapping", () => {
  const value = decodeValue(
    wire({
      kind: {
        case: "coordinateFrame",
        value: {
          name: "celsius",
          scale: {
            unit: millimetre,
            mapping: { mapped: length(0), reference: length(273.15) },
          },
        },
      },
    }),
  );
  assert.ok(
    value.kind === "coordinateFrame" && value.scale?.mapping !== undefined,
  );
  assert.deepEqual(value.scale.mapping.reference.magnitude, {
    kind: "real",
    value: 273.15,
  });
  assert.equal(formatValue(value), "celsius [mm]");
});

test("a frame that contradicts itself is malformed", () => {
  assert.throws(
    () =>
      decodeValue(
        wire({
          kind: { case: "coordinateFrame", value: { name: "f", axes: [{}] } },
        }),
      ),
    MalformedValueError,
  );
  assert.throws(
    () =>
      decodeValue(
        wire({
          kind: {
            case: "coordinateTransformation",
            value: { shape: { case: "sequence", value: { steps: [{}] } } },
          },
        }),
      ),
    MalformedValueError,
  );
});

test("the capability is named as the service names it", () => {
  assert.equal(CAPABILITY_COORDINATE_FRAME_VALUES, "coordinate_frame_values");
});
