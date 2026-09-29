import assert from "node:assert/strict";
import { test } from "node:test";
import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { AddSequenceEditSchema } from "../src/generated/sysml_pb.js";

test("recursive AddSequenceEdit fields round-trip on the wire", () => {
  const nested = create(AddSequenceEditSchema, {
    memberKind: "if",
    condition: "ready",
    body: [
      create(AddSequenceEditSchema, {
        memberKind: "assign",
        target: "x",
        value: "x + 1",
      }),
    ],
    elseBody: [
      create(AddSequenceEditSchema, {
        keyword: "else",
        ref: "done",
      }),
    ],
    multiplicity: "[1]",
    parameter: "message",
    via: "port",
    until: "finished",
  });
  assert.deepEqual(
    fromBinary(AddSequenceEditSchema, toBinary(AddSequenceEditSchema, nested)),
    nested,
  );
});
