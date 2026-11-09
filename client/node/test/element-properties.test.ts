import assert from "node:assert/strict";
import { before, test } from "node:test";
import {
  CAPABILITY_ELEMENT_PROPERTIES,
  ElementIdentity,
  ElementProperties,
  MissingCapabilityError,
  SymbolNotFoundError,
  connect,
} from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";
import { fakeConnection } from "./support/fake.js";

before(() => {
  useServiceBinary();
});

const MODEL = `package EP {
    port def CmdPort { out attribute cmd : ScalarValues::Integer; }
    part def Unit { port p : CmdPort; }
    part def Pair :> Unit {
        part u1 : Unit;
        part u2 : Unit;
        connect u1 to u2;
    }
}
`;

test("derived properties are read by their specification names", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has(CAPABILITY_ELEMENT_PROPERTIES));
  const model = await connection.loads(MODEL);
  const read = await model.elementProperties("EP::Pair", "inheritedFeature", "isAbstract", "connectorEnd");
  assert.ok(read instanceof ElementProperties);
  assert.equal(read.element.qualifiedName, "EP::Pair");
  assert.equal(read.element.metaclass, "PartDefinition");
  const inherited = read.get("inheritedFeature");
  assert.ok(inherited?.many && inherited.derived && inherited.supported);
  assert.equal(inherited.definingClass, "Type");
  assert.equal(String(inherited.elements[0]), "EP::Unit::p");
  assert.ok(inherited.elements.some((element) => element.qualifiedName.startsWith("Parts::")));
  assert.equal(read.get("isAbstract")?.value, false);
  assert.match(read.get("connectorEnd")?.unsupported ?? "", /declares no property connectorEnd/);
  assert.equal(read.has("name"), false);
  assert.deepEqual([...read].map((property) => property.name), ["inheritedFeature", "isAbstract", "connectorEnd"]);
});

test("an unnamed connector is read by position and walked by form id", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const connector = await model.elementProperties("EP::Pair::@2", "relatedFeature", "sourceFeature");
  assert.equal(connector.element.metaclass, "ConnectionUsage");
  assert.equal(connector.element.qualifiedName, "");
  assert.deepEqual(
    connector.get("relatedFeature")?.elements.map(String),
    ["EP::Pair::u1", "EP::Pair::u2"],
  );
  const source = connector.get("sourceFeature")?.value;
  assert.ok(source instanceof ElementIdentity);
  const again = await model.elementProperties(source.id);
  assert.equal(again.element.qualifiedName, "EP::Pair::u1");
  assert.ok(again.properties.length > 10);
  assert.equal(again.get("isComposite")?.value, true);
});

test("a missing element and a missing capability are refused by kind", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  await assert.rejects(model.elementProperties("EP::Nowhere", "name"), SymbolNotFoundError);
  const older = await fakeConnection(["query"]);
  await assert.rejects(older.elementProperties("hash", "EP::Pair"), MissingCapabilityError);
});
