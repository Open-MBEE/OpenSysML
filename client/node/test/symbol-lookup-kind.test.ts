import assert from "node:assert/strict";
import { before, test } from "node:test";
import {
  CAPABILITY_ELEMENT_PROPERTIES,
  CAPABILITY_SYMBOL_LOOKUP_KIND,
  CAPABILITY_SYMBOL_NAME_RESOLUTION,
  SymbolNotFoundError,
  connect,
} from "../src/node/index.js";
import { useServiceBinary } from "./support/service.js";
import { fakeConnection } from "./support/fake.js";
import { LookupKind } from "../src/generated/sysml_pb.js";

before(() => {
  useServiceBinary();
});

const COLLISIONS = `package Col__V__c_pend0 { part marker; }
package Col {
    port def P;
    part def V { port q : P; port r : P; connection c connect q to r; }
    part def W { part a; part b; part { part inner; } part '@2' { part named; } }
}
`;

test("a name and an id of one spelling are told apart by the stated kind", async () => {
  await using connection = await connect();
  assert.ok(connection.info.has(CAPABILITY_SYMBOL_LOOKUP_KIND));
  const model = await connection.loads(COLLISIONS);

  assert.equal((await model.symbolByName("Col__V__c_pend0")).kind, "package");
  assert.equal((await model.symbolByElementId("Col__V__c_pend0")).kind, "connectorEnd");
  assert.equal((await model.symbolById("Col__V__c_pend0")).kind, "package");
  await assert.rejects(model.symbolByName("Col__V__c_pend1"), SymbolNotFoundError);

  const owner = await model.symbolByName("Col::W");
  assert.deepEqual(owner.childIds, ["Col::W::a", "Col::W::b", "Col::W::@2", "Col::W::'@2'"]);
  const children = await owner.children();
  assert.deepEqual(children.map((child) => child.id), owner.childIds);
  assert.deepEqual(children[2]?.childIds, ["Col::W::@2::inner"]);
  assert.deepEqual(children[3]?.childIds, ["Col::W::'@2'::named"]);
  assert.equal((await model.symbolByName("Col::W::'@2'")).id, "Col::W::'@2'");
  assert.equal((await model.symbolByElementId("Col::W::@2")).name, "");
  await assert.rejects(model.symbolByName("Col::W::@2::inner"), SymbolNotFoundError);

  const named = await model.elementProperties({ name: "Col__V__c_pend0" }, "declaredName");
  assert.equal(named.element.metaclass, "Package");
  const identified = await model.elementProperties({ id: "Col__V__c_pend0" }, "declaredName");
  assert.equal(identified.element.metaclass, "ReferenceUsage");
});

test("the lookup kind is sent only to a service that reports symbol_lookup_kind", async () => {
  const symbol = {
    id: "Col__V__c_pend0",
    name: "",
    kind: "connectorEnd",
    metadata: {},
    childIds: [],
    attributes: [],
    specializations: [],
    withheldLibraryAttributes: 0,
    members: [],
    withheldLibraryMembers: 0,
  };
  const id = "Col__V__c_pend0";
  for (const [capabilities, want] of [
    [
      [CAPABILITY_SYMBOL_LOOKUP_KIND, CAPABILITY_SYMBOL_NAME_RESOLUTION, CAPABILITY_ELEMENT_PROPERTIES],
      [
        { symbolId: id, lookup: LookupKind.QUALIFIED_NAME, resolveName: true },
        { symbolId: id, lookup: LookupKind.ELEMENT_ID, resolveName: false },
        { symbolId: id, lookup: LookupKind.UNSPECIFIED, resolveName: true },
        { elementId: id, lookup: LookupKind.QUALIFIED_NAME },
        { elementId: id, lookup: LookupKind.ELEMENT_ID },
        { elementId: id, lookup: LookupKind.UNSPECIFIED },
      ],
    ],
    [
      [CAPABILITY_SYMBOL_NAME_RESOLUTION, CAPABILITY_ELEMENT_PROPERTIES],
      [
        { symbolId: id, lookup: LookupKind.UNSPECIFIED, resolveName: true },
        { symbolId: id, lookup: LookupKind.UNSPECIFIED, resolveName: false },
        { symbolId: id, lookup: LookupKind.UNSPECIFIED, resolveName: true },
        { elementId: id, lookup: LookupKind.UNSPECIFIED },
        { elementId: id, lookup: LookupKind.UNSPECIFIED },
        { elementId: id, lookup: LookupKind.UNSPECIFIED },
      ],
    ],
  ] as const) {
    const asked: unknown[] = [];
    const connection = await fakeConnection(capabilities, (method, input) => {
      const request = input as { lookup: LookupKind; symbolId: string; resolveName: boolean; elementId: string };
      if (method === "GetSymbol") {
        asked.push({ symbolId: request.symbolId, lookup: request.lookup, resolveName: request.resolveName });
        return { symbol };
      }
      if (method === "GetElementProperties") {
        asked.push({ elementId: request.elementId, lookup: request.lookup });
        return { element: { id, qualifiedName: "", metaclass: "ReferenceUsage" }, properties: [], error: "" };
      }
      throw new Error(`the client asked ${method}`);
    });
    const model = connection.model("hash");
    await model.symbolByName("Col__V__c_pend0");
    await model.symbolByElementId("Col__V__c_pend0");
    await model.symbolById("Col__V__c_pend0");
    await model.elementProperties({ name: "Col__V__c_pend0" });
    await model.elementProperties({ id: "Col__V__c_pend0" });
    await model.elementProperties("Col__V__c_pend0");
    assert.deepEqual(asked, want);
  }
});
