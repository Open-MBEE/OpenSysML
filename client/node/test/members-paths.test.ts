import assert from "node:assert/strict";
import { before, test } from "node:test";
import {
  CAPABILITY_INSTANCE_PATHS,
  CAPABILITY_SYMBOL_MEMBERS,
  InstancePaths,
  MissingCapabilityError,
  connect,
} from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";
import { fakeConnection } from "./support/fake.js";

before(() => {
  useServiceBinary();
});

const MEMBERS = `package Demo {
    part def Base { part p; }
    part def Mid :> Base;
    part m : Mid;
}
`;

const VEHICLE = `package Demo {
    part def Engine { attribute power = 300.0; }
    part def Vehicle {
        attribute mass default = 1500.0;
        part engine : Engine;
    }
    part sedan : Vehicle { attribute :>> mass = 1200.0; }
}
`;

test("a usage lists the members it inherits through its type", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has(CAPABILITY_SYMBOL_MEMBERS));
  const model = await connection.loads(MEMBERS);
  const m = await model.symbolById("Demo::m");
  assert.deepEqual(m.childIds, []);
  assert.deepEqual(m.ownMembers(), []);
  const [p] = m.inheritedMembers();
  assert.deepEqual(p, {
    id: "Demo::Base::p",
    name: "p",
    kind: "partUsage",
    inherited: true,
    inheritedFrom: "Demo::Base",
    library: false,
  });
  assert.ok(m.members.every((member) => !member.library));
  assert.ok(m.withheldLibraryMembers > 0);

  const base = await model.symbolById("Demo::Base");
  assert.deepEqual(base.childIds, ["Demo::Base::p"]);
  assert.deepEqual(base.ownMembers(), [
    { id: "Demo::Base::p", name: "p", kind: "partUsage", inherited: false, inheritedFrom: "", library: false },
  ]);

  const full = await m.withLibraryMembers();
  assert.equal(full.withheldLibraryMembers, 0);
  assert.deepEqual(full.members.slice(0, m.members.length), m.members);
  const library = full.members.filter((member) => member.library);
  assert.equal(library.length, m.withheldLibraryMembers);
  assert.ok(library.every((member) => member.inherited && member.inheritedFrom !== ""));
  const again = await model.symbolById("Demo::m", { includeLibraryMembers: true });
  assert.deepEqual(again.members, full.members);
});

test("an object says how it was reached, and every path to it is listed", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has(CAPABILITY_INSTANCE_PATHS));
  const model = await connection.loads(VEHICLE);
  const tree = await model.instantiate("Demo::sedan");
  assert.equal(tree.root.reachedBy, undefined);
  assert.equal(tree.root.ownerId, undefined);
  assert.equal(tree.root.path, "");
  assert.equal(tree.owner(tree.root), undefined);
  const engineValue = tree.get("engine");
  assert.ok(engineValue?.kind === "single" && engineValue.value.kind === "instance");
  const engine = tree.byId(engineValue.value.id);
  assert.ok(engine !== undefined);
  assert.deepEqual(engine.reachedBy, {
    instanceId: tree.root.id,
    featureName: "engine",
    featureSymbolId: "Demo::Vehicle::engine",
    index: 0,
    path: "engine",
  });
  assert.equal(engine.ownerId, tree.root.id);
  assert.equal(engine.path, "engine");
  assert.equal(tree.owner(engine)?.id, tree.root.id);

  const paths = await model.instancePaths(engine);
  assert.ok(paths instanceof InstancePaths);
  assert.equal(paths.truncated, false);
  assert.equal(paths.length, 1);
  const [path] = paths;
  assert.equal(path.rootInstanceId, tree.root.id);
  assert.equal(path.rootSymbolId, "Demo::sedan");
  assert.equal(path.path, "engine");
  assert.deepEqual(path.steps, [
    { featureName: "engine", featureSymbolId: "Demo::Vehicle::engine", index: 0, instanceId: engine.id },
  ]);
  assert.deepEqual(await model.instancePaths(engine.id), paths);
  assert.deepEqual(await model.instancePaths("Demo::sedan.engine"), paths);
  assert.equal((await model.instancePaths(tree.root)).paths[0]?.path, "");
  await assert.rejects(model.instancePaths(9999n), (error: Error) => /NOT_FOUND|no object/.test(error.message));
});

test("the reads are refused before asking a service without their capabilities", async () => {
  const older = await fakeConnection(["query"], (method) => {
    throw new Error(`the client asked ${method}`);
  });
  const model = older.model("hash");
  await assert.rejects(model.symbolById("Demo::m", { includeLibraryMembers: true }), (error: MissingCapabilityError) => {
    assert.ok(error instanceof MissingCapabilityError);
    assert.equal(error.capability, CAPABILITY_SYMBOL_MEMBERS);
    return true;
  });
  await assert.rejects(model.instancePaths(1n), (error: MissingCapabilityError) => {
    assert.ok(error instanceof MissingCapabilityError);
    assert.equal(error.capability, CAPABILITY_INSTANCE_PATHS);
    return true;
  });
});

test("a symbol from a service without symbol_members enumerates no members", async () => {
  const older = await fakeConnection(["query"], (method) => {
    if (method !== "GetSymbol") {
      throw new Error(`the client asked ${method}`);
    }
    return {
      symbol: {
        id: "Demo::Base",
        name: "Base",
        kind: "partDef",
        metadata: {},
        childIds: ["Demo::Base::p"],
        attributes: [],
        specializations: [],
        withheldLibraryAttributes: 0,
        members: [],
        withheldLibraryMembers: 0,
        annotatingMetadataIds: [],
      },
    };
  });
  const base = await older.model("hash").symbolById("Demo::Base");
  assert.deepEqual(base.childIds, ["Demo::Base::p"]);
  assert.deepEqual(base.members, []);
  assert.equal(base.withheldLibraryMembers, 0);
  await assert.rejects(base.withLibraryMembers(), MissingCapabilityError);
});
