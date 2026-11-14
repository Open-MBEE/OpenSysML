import assert from "node:assert/strict";
import { before, test } from "node:test";
import {
  CAPABILITY_SYMBOL_ANNOTATING_METADATA,
  MissingCapabilityError,
  connect,
} from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";
import { fakeConnection } from "./support/fake.js";

before(() => {
  useServiceBinary();
});

const ANNOTATED = `package D10 {
    metadata def FailureMode { attribute severity : ScalarValues::Integer; }
    #FailureMode part def PumpP;
    part def PumpB { @FailureMode { severity = 3; } }
    part def PumpA;
    metadata pumpA_fm : FailureMode about PumpA { severity = 2; }
}
`;

test("a symbol names the metadata usages annotating it in every form", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has(CAPABILITY_SYMBOL_ANNOTATING_METADATA));
  const model = await connection.loads(ANNOTATED);
  const ids = async (id: string) => (await model.symbolById(id)).annotatingMetadataIds;
  assert.deepEqual(await ids("D10::PumpP"), ["D10::PumpP::@0"]);
  assert.deepEqual(await ids("D10::PumpB"), ["D10::PumpB::@0"]);
  assert.deepEqual(await ids("D10::PumpA"), ["D10::pumpA_fm"]);
  assert.deepEqual(await ids("D10::pumpA_fm"), []);
});

test("an about usage is read from what it annotates, with its type and value", async () => {
  await using connection = await connect();
  const model = await connection.loads(ANNOTATED);
  const pump = await model.symbolById("D10::PumpA");
  assert.deepEqual(pump.childIds, []);
  const [usage] = await pump.annotatingMetadata();
  assert.equal(usage.id, "D10::pumpA_fm");
  assert.equal(usage.kind, "metadataUsage");
  assert.equal(usage.type?.resolvedId, "D10::FailureMode");
  assert.deepEqual(usage.attribute("severity")?.value, { kind: "int", value: 2n });
});

test("a symbol from a service without symbol_annotating_metadata refuses to list them", async () => {
  const older = await fakeConnection(["query"], (method) => {
    if (method !== "GetSymbol") {
      throw new Error(`the client asked ${method}`);
    }
    return {
      symbol: {
        id: "D10::PumpA",
        name: "PumpA",
        kind: "partDef",
        metadata: {},
        childIds: [],
        attributes: [],
        specializations: [],
        withheldLibraryAttributes: 0,
        members: [],
        withheldLibraryMembers: 0,
        annotatingMetadataIds: [],
      },
    };
  });
  const pump = await older.model("hash").symbolById("D10::PumpA");
  assert.deepEqual(pump.annotatingMetadataIds, []);
  await assert.rejects(pump.annotatingMetadata(), (error: MissingCapabilityError) => {
    assert.ok(error instanceof MissingCapabilityError);
    assert.equal(error.capability, CAPABILITY_SYMBOL_ANNOTATING_METADATA);
    return true;
  });
});
