// The editor: operations, validation and capability gating over a fake
// transport, then the round trip against the real service.

import assert from "node:assert/strict";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import {
  type ApplyEditsRequest,
  type ApplyEditsResponse,
  type EditOperation,
} from "../src/generated/sysml_pb.js";
import {
  Body,
  EditError,
  EditResultError,
  EditTargetError,
  InvalidEditError,
  MemberNameTakenError,
  MissingCapabilityError,
  MoveReferencedError,
  NoEditsError,
  OverlappingEditsError,
  OwnerInsideTargetError,
  OwnerNotFoundError,
  OwnerNotNamespaceError,
  ReferencedElsewhereError,
  Referrer,
  RenameReferencedError,
  DeleteReferencedError,
  errorForFailure,
  failureName,
} from "../src/core/index.js";
import { connect, save } from "../src/node/index.js";
import { ALL_CAPABILITIES, fakeConnection } from "./support/fake.js";
import { useServiceBinary } from "./support/service.js";

const AUTHORING_CAPABILITIES = [
  "apply_edits",
  "edit_documents",
  "authoring",
  "connection_authoring",
  "satisfy_authoring",
  "requirement_constraint_authoring",
  "transition_authoring",
  "verification_objective_authoring",
  "metadata_authoring",
  "metadata_prefix_authoring",
  "sequence_authoring",
  "action_body_statement_authoring",
  "import_authoring",
  "documentation_authoring",
  "comment_authoring",
  "member_modifiers",
  "implicit_parameters",
  "constraint_body_authoring",
  "state_action_authoring",
];

const MODEL = `package Demo {
    // The mass of one unit, measured on the bench.
    part def SC {

        attribute unitMass : ISQ::MassValue default = 1000.0[SI::kg];

        // No margin has been agreed yet.
        attribute margin : ISQ::MassValue;

        attribute label : ScalarValues::String = "flight-1";
        attribute active : ScalarValues::Boolean = true;
        attribute total : ISQ::MassValue = unitMass;

        part avionics {
            part board {
                attribute count : ScalarValues::Integer = 2;
            }
        }
    }

    part sc : SC {
        attribute redefines unitMass = 1200.0[SI::kg];
    }
}
`;

function operationsOf(requests: ApplyEditsRequest[]): EditOperation[] {
  return requests.flatMap((request) => request.operations);
}

type OperationValue<C extends EditOperation["operation"]["case"]> =
  EditOperation["operation"] extends infer Op
    ? Op extends { case: C; value: infer V }
      ? V
      : never
    : never;

function operationValue<C extends EditOperation["operation"]["case"]>(
  operation: EditOperation["operation"],
  c: C,
): OperationValue<C> {
  assert.equal(operation.case, c);
  return operation.value as OperationValue<C>;
}

function stubAnswer(
  requests: ApplyEditsRequest[],
  content = "edited",
): (method: string, input: unknown) => unknown {
  return (method, input) => {
    assert.equal(method, "ApplyEdits");
    requests.push(input as ApplyEditsRequest);
    return {
      content,
      applied: [],
      documents: content === "" ? [] : [{ name: "<content>", content }],
      error: "",
      failure: 0,
      diagnostics: [],
      referringElements: [],
      referrers: [],
    };
  };
}

test("applyEdits names the missing capability before sending", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection([], stubAnswer(requests));
  await assert.rejects(
    () => connection.applyEdits("hash", [["set_value", "Demo::x", "1"]]),
    (error: unknown) =>
      error instanceof MissingCapabilityError && error.capability === "apply_edits",
  );
  assert.deepEqual(requests, []);
});

test("setValue and rename operations cross exactly as written", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(ALL_CAPABILITIES, stubAnswer(requests));
  await connection
    .edit("hash")
    .setValue("Demo::SC::unitMass", "1050.0[SI::kg]")
    .rename("Demo::SC::margin", "reserve")
    .apply();
  const [first, second] = operationsOf(requests);
  const setValue = operationValue(first.operation, "setValue");
  assert.equal(setValue.target, "Demo::SC::unitMass");
  assert.equal(setValue.value, "1050.0[SI::kg]");
  const rename = operationValue(second.operation, "rename");
  assert.equal(rename.target, "Demo::SC::margin");
  assert.equal(rename.newName, "reserve");
  assert.equal(requests[0].acceptDocuments, true);
  assert.equal(requests[0].modelHash, "hash");
});

test("addMember and delete requests are exact", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(ALL_CAPABILITIES, stubAnswer(requests));
  await connection
    .edit("hash")
    .addMember("Demo::SC", "part", "board", {
      type: "Board",
      multiplicity: "[1]",
      value: "1",
    })
    .delete("Demo::sc")
    .apply();
  const [add, remove] = operationsOf(requests);
  const member = operationValue(add.operation, "addMember");
  assert.equal(member.owner, "Demo::SC");
  assert.equal(member.kind, "part");
  assert.equal(member.name, "board");
  assert.equal(member.type, "Board");
  assert.equal(member.multiplicity, "[1]");
  assert.equal(member.value, "1");
  const deleted = operationValue(remove.operation, "delete");
  assert.equal(deleted.target, "Demo::sc");
  assert.equal(deleted.cascade, false);
});

test("member modifiers, direction and metadata serialize", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(ALL_CAPABILITIES, stubAnswer(requests));
  await connection
    .edit("hash")
    .addMember("Demo", "part def", "Abstract", { abstract: true })
    .addMember("Demo", "attribute", "r", {
      specializes: "Base::x",
      redefines: ["Base::y"],
      default: true,
      direction: "in",
      metadata: ["Safety"],
      expression: "x > 0",
      doc: "Docs.",
    })
    .apply();
  const [abstract, modified] = operationsOf(requests);
  if (abstract.operation.case === "addMember") {
    assert.equal(abstract.operation.value.isAbstract, true);
  }
  if (modified.operation.case === "addMember") {
    assert.deepEqual(modified.operation.value.specializes, ["Base::x"]);
    assert.deepEqual(modified.operation.value.redefines, ["Base::y"]);
    assert.equal(modified.operation.value.isDefault, true);
    assert.equal(modified.operation.value.direction, "in");
    assert.deepEqual(modified.operation.value.metadataPrefixes, ["Safety"]);
    assert.equal(modified.operation.value.bodyExpression, "x > 0");
    assert.equal(modified.operation.value.doc, "Docs.");
  }
});

test("sequence statements serialize their fields recursively", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(ALL_CAPABILITIES, stubAnswer(requests));
  const body = new Body()
    .addAccept("signal", { type: "Sig" })
    .addIf(
      "count > 0",
      new Body().addAssign("x", "x + 1").addSend("go", { to: "out" }),
      new Body().addTerminate("x"),
    )
    .addWhile("ready", new Body().addLoop(new Body().addElse("done")));
  await connection
    .edit("hash")
    .addFirst("Demo::a", "start", { after: "begin" })
    .addThen("Demo::a", { ref: "mid", multiplicity: "[2]" })
    .addFor("Demo::a", "item", "items", body, { type: "Item" })
    .addGuardedThen("Demo::a", "ok", "finish")
    .apply();
  const [first, thenOp, loop, guard] = operationsOf(requests);
  const firstOp = operationValue(first.operation, "addSequence");
  assert.equal(firstOp.keyword, "first");
  assert.equal(firstOp.ref, "start");
  assert.equal(firstOp.after, "begin");
  if (thenOp.operation.case === "addSequence") {
    assert.equal(thenOp.operation.value.keyword, "then");
    assert.equal(thenOp.operation.value.multiplicity, "[2]");
  }
  if (loop.operation.case === "addSequence") {
    assert.equal(loop.operation.value.keyword, "then");
    assert.equal(loop.operation.value.memberKind, "for");
    assert.equal(loop.operation.value.parameter, "item");
    assert.equal(loop.operation.value.value, "items");
    assert.equal(loop.operation.value.type, "Item");
    const [accept, branch, looped] = loop.operation.value.body;
    assert.equal(accept.memberKind, "accept");
    assert.equal(accept.parameter, "signal");
    assert.equal(accept.type, "Sig");
    assert.equal(branch.memberKind, "if");
    assert.equal(branch.condition, "count > 0");
    assert.equal(branch.body[0].memberKind, "assign");
    assert.equal(branch.body[1].memberKind, "send");
    assert.equal(branch.elseBody[0].memberKind, "terminate");
    assert.equal(looped.memberKind, "while");
    assert.equal(looped.body[0].body[0].memberKind, "");
    assert.equal(looped.body[0].body[0].keyword, "else");
  }
  const guardOp = operationValue(guard.operation, "addSequence");
  assert.equal(guardOp.keyword, "if");
  assert.equal(guardOp.condition, "ok");
  assert.equal(guardOp.ref, "finish");
});

test("each operation family names the capability it needs", async () => {
  const cases: { tuple: readonly unknown[]; missing: string }[] = [
    { tuple: ["delete", "P::x", false], missing: "authoring" },
    { tuple: ["move", "P::x", "P"], missing: "authoring" },
    { tuple: ["add_connection", "P", "flow", "a", "b", "", ""], missing: "connection_authoring" },
    { tuple: ["add_satisfy", "P", "R", "f", false, false], missing: "satisfy_authoring" },
    { tuple: ["add_requirement_constraint", "P", "require", "x > 0", ""], missing: "requirement_constraint_authoring" },
    { tuple: ["add_transition", "P", "", "a", "b", "", "", "", false], missing: "transition_authoring" },
    { tuple: ["add_verify", "P", "R"], missing: "verification_objective_authoring" },
    { tuple: ["add_metadata", "P", "M", "", [], [], false], missing: "metadata_authoring" },
    { tuple: ["add_metadata_prefix", "P::x", "M"], missing: "metadata_prefix_authoring" },
    { tuple: ["add_sequence", "P", "", "", "", "", "", ""], missing: "sequence_authoring" },
    { tuple: ["add_import", "P", "", "Q::*", false, false, []], missing: "import_authoring" },
    { tuple: ["add_documentation", "P::x", "body", "", "", false], missing: "documentation_authoring" },
    { tuple: ["add_comment", "P", "body", "", [], ""], missing: "comment_authoring" },
    { tuple: ["add_note", "P::x", "note"], missing: "comment_authoring" },
  ];
  for (const { tuple, missing } of cases) {
    const requests: ApplyEditsRequest[] = [];
    const capabilities = AUTHORING_CAPABILITIES.filter(
      (capability) => capability !== missing,
    );
    await using connection = await fakeConnection(capabilities, stubAnswer(requests));
    await assert.rejects(
      () => connection.applyEdits("hash", [tuple]),
      (error: unknown) =>
        error instanceof MissingCapabilityError && error.capability === missing,
    );
    assert.deepEqual(requests, [], `a ${missing}-gated operation reached the service`);
  }
});

test("implicit parameters and member modifiers are checked last", async () => {
  const withoutImplicit = AUTHORING_CAPABILITIES.filter((c) => c !== "implicit_parameters");
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(withoutImplicit, stubAnswer(requests));
  await assert.rejects(
    () =>
      connection.applyEdits("hash", [
        ["add_member", "P", "", "x", "T", "", "1", []],
      ]),
    (error: unknown) =>
      error instanceof MissingCapabilityError && error.capability === "implicit_parameters",
  );
  assert.deepEqual(requests, []);

  const withoutModifiers = AUTHORING_CAPABILITIES.filter((c) => c !== "member_modifiers");
  await using second = await fakeConnection(withoutModifiers, stubAnswer([]));
  await assert.rejects(
    () =>
      second.applyEdits("hash", [
        ["add_member", "P", "attribute", "x", "", "", "", [], false, [], false, "in"],
      ]),
    (error: unknown) =>
      error instanceof MissingCapabilityError && error.capability === "member_modifiers",
  );
});

test("malformed operations are refused before anything is sent", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(ALL_CAPABILITIES, stubAnswer(requests));
  await assert.rejects(
    () => connection.applyEdits("hash", [["delete", "Demo::SC", ""]]),
    /malformed delete operation/,
  );
  await assert.rejects(
    () => connection.applyEdits("hash", [["move", "Demo::SC"]]),
    /malformed move operation/,
  );
  await assert.rejects(
    () => connection.applyEdits("hash", [["frobnicate", "Demo::SC"]]),
    /unknown edit operation/,
  );
  await assert.rejects(
    () =>
      connection.applyEdits("hash", [
        ["add_member", "P", "part", "x", "", "", "", [], "yes"],
      ]),
    /malformed add_member/,
  );
  assert.deepEqual(requests, []);
});

test("the caller's arguments are checked when the operation is added", async () => {
  await using connection = await fakeConnection(ALL_CAPABILITIES);
  const edit = connection.edit("hash");
  assert.throws(() => edit.setValue("x", 1 as never), TypeError);
  assert.throws(() => edit.rename("x", 2 as never), TypeError);
  assert.throws(
    () => edit.addMember("P", "part", "x", { abstract: "yes" as never }),
    TypeError,
  );
  assert.throws(() => edit.addThen("P", {}), RangeError);
  assert.throws(
    () => edit.addThen("P", { ref: "a", action: "b" }),
    /exactly one of ref and action is required/,
  );
  assert.throws(
    () => edit.addThen("P", { ref: "a", kind: "state" }),
    /a then reference takes no type or kind/,
  );
  assert.throws(() => edit.addNote("P::x", "line one\nline two"), RangeError);
  assert.throws(
    () => edit.addCalcDef("P", "c", { returnType: "", returnExpression: "x" }),
    /return_expression requires return_type/,
  );
  assert.throws(
    () => edit.addCalcDef("P", "c", { expression: "x", returnExpression: "x" }),
    /expression and return_expression both bind the result; give one/,
  );
  assert.throws(
    () => new Body().addThen("a", "b"),
    /exactly one of ref and action is required/,
  );
  assert.throws(
    () => new Body().addAssign("x", "1", { multiplicity: "[2]" }),
    /multiplicity requires then=True/,
  );
  assert.throws(
    () => new Body().addIf("c", "not-a-body" as never),
    TypeError,
  );
});

test("an empty editor is refused and an applied one cannot be reused", async () => {
  const requests: ApplyEditsRequest[] = [];
  await using connection = await fakeConnection(ALL_CAPABILITIES, stubAnswer(requests));
  await assert.rejects(() => connection.edit("hash").apply(), NoEditsError);
  const edit = connection.edit("hash").setValue("P::x", "1");
  await edit.apply();
  assert.equal(edit.applied, true);
  assert.equal(edit.length, 1);
  await assert.rejects(() => edit.apply(), /already been applied/);
  assert.throws(() => edit.rename("P::x", "y"), /already been applied/);
});

test("a refusal becomes its own typed error with its details", async () => {
  const answer = (): ApplyEditsResponse =>
    ({
      content: "",
      applied: [],
      documents: [],
      error: "the name is taken",
      failure: 14,
      diagnostics: [
        { message: "taken", severity: "error", document: "doc.sysml", range: undefined },
      ],
      referringElements: ["P::a", "P::b"],
      referrers: [{ name: "P::a", document: "a.sysml" }],
    }) as unknown as ApplyEditsResponse;
  await using connection = await fakeConnection(ALL_CAPABILITIES, answer);
  await assert.rejects(
    () => connection.applyEdits("hash", [["set_value", "P::x", "1"]]),
    (error: unknown) => {
      if (!(error instanceof MemberNameTakenError)) {
        return false;
      }
      assert.equal(error.failure, "EDIT_FAILURE_MEMBER_NAME_TAKEN");
      assert.deepEqual([...error.referringElements], ["P::a", "P::b"]);
      assert.deepEqual(error.referrers, [new Referrer("P::a", "a.sysml")]);
      return true;
    },
  );
});

test("each failure kind maps to its own error class", () => {
  const cases: [number, new (message: string) => Error][] = [
    [1, NoEditsError],
    [2, EditTargetError],
    [3, EditTargetError],
    [4, EditTargetError],
    [5, InvalidEditError],
    [6, InvalidEditError],
    [7, EditTargetError],
    [8, RenameReferencedError],
    [9, OverlappingEditsError],
    [10, EditResultError],
    [11, OwnerNotFoundError],
    [12, OwnerNotNamespaceError],
    [13, InvalidEditError],
    [14, MemberNameTakenError],
    [15, DeleteReferencedError],
    [16, OwnerInsideTargetError],
    [17, MoveReferencedError],
    [18, ReferencedElsewhereError],
  ];
  for (const [failure, expected] of cases) {
    const error = errorForFailure(failureName(failure), "refused");
    assert.ok(error instanceof expected, `failure ${String(failure)}`);
    assert.ok(error instanceof EditError);
    assert.equal(error.failure, failureName(failure));
  }
  const unknown = errorForFailure(failureName(99), "mystery");
  assert.ok(unknown instanceof EditError);
  assert.equal(unknown.failure, "EDIT_FAILURE_99");
  assert.equal(failureName(0), "EDIT_FAILURE_UNSPECIFIED");
});

test("the result is a conversion carrying applied edits and documents", async () => {
  const answer = (): ApplyEditsResponse =>
    ({
      content: "edited",
      applied: [
        {
          operationIndex: 0,
          target: "P::x",
          offset: 4,
          length: 3,
          oldText: "one",
          newText: "two",
          document: "doc.sysml",
        },
      ],
      documents: [{ name: "doc.sysml", content: "edited" }],
      error: "",
      failure: 0,
      diagnostics: [],
      referringElements: [],
      referrers: [],
    }) as unknown as ApplyEditsResponse;
  await using connection = await fakeConnection(ALL_CAPABILITIES, answer);
  const result = await connection.applyEdits("hash", [["set_value", "P::x", "two"]]);
  assert.equal(result.content, "edited");
  assert.equal(result.fromFormat, "sysml");
  assert.equal(result.toFormat, "sysml");
  const [applied] = result.applied;
  assert.equal(applied.operationIndex, 0);
  assert.equal(applied.oldText, "one");
  assert.equal(applied.newText, "two");
  assert.equal(applied.document, "doc.sysml");
  assert.equal(String(applied), 'P::x: "one" -> "two"');
  assert.equal(result.documents[0].name, "doc.sysml");
  assert.equal(String(result.documents[0]), "edited");
});

before(() => {
  useServiceBinary();
});

test("a changed value is written back byte-exact outside its span", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.edit().setValue("Demo::sc::unitMass", "1050.0[SI::kg]").apply();
  const edited = result.content;
  assert.match(edited, /1050\.0\[SI::kg\]/);
  const [applied] = result.applied;
  assert.equal(edited.slice(0, applied.offset), MODEL.slice(0, applied.offset));
  assert.equal(
    edited.slice(applied.offset + applied.newText.length),
    MODEL.slice(applied.offset + applied.length),
  );
  assert.match(edited, /\/\/ The mass of one unit, measured on the bench\./);
  const again = await connection.loads(edited);
  assert.ok(again.ok, again.errors.map((diagnostic) => diagnostic.message).join(", "));
});

test("authored definitions and parts read back", async () => {
  await using connection = await connect();
  const model = await connection.loads("package Demo;\n");
  const result = await model
    .edit()
    .addPartDef("", "Vehicle")
    .addPart("Vehicle", "engine", { type: "Vehicle" })
    .apply();
  const again = await connection.loads(result.content);
  assert.ok((await again.find("Vehicle")) !== undefined);
  assert.ok((await again.find("Vehicle::engine")) !== undefined);
});

test("calc, action and state forms survive the round trip", async () => {
  const source = `package P {
    private import ScalarValues::*;
    action def A;
    constraint def ConstraintType;
    attribute x : Real = 1.0;
    state def S { state active; }
    part def Base { state cycle : S; }
    part def Host :> Base;
}
`;
  await using connection = await connect();
  const model = await connection.loads(source);
  const result = await model
    .edit()
    .addCalcDef("P", "DeliveredEnergy", {
      inputs: [["power", "Real"], ["seconds", "Real"]],
      returnType: "Real",
      returnExpression: "power * seconds",
    })
    .addConstraintDef("P", "Positive", { expression: "x > 0" })
    .addConstraint("P", "Bounded", { value: "true", expression: "x > 0" })
    .addAssertConstraint("P", { name: "checked", expression: "true" })
    .addAssertConstraint("P", { type: "ConstraintType" })
    .addAssertConstraint("P", { name: "notChecked", expression: "false", negated: true })
    .addExhibitState("P::Host", "shown", { type: "S" })
    .addMember("P::Host::shown", "state", "nested")
    .addExhibit("P::Host", "cycle")
    .addStateAction("P::S::active", "entry", "onEntry", { type: "A" })
    .addStateAction("P::S::active", "do", "work", { type: "A" })
    .addMember("P::S::active::work", "attribute", "input", {
      type: "ScalarValues::Real",
      direction: "in",
    })
    .addStateAction("P::S::active", "exit", "onExit", { type: "A" })
    .apply();
  const edited = result.content;
  assert.match(edited, /calc def DeliveredEnergy/);
  assert.match(edited, /in power : Real/);
  assert.match(edited, /return : Real = power \* seconds/);
  assert.match(edited, /constraint def Positive \{ x > 0 \}/);
  assert.match(edited, /constraint Bounded = true \{ x > 0 \}/);
  assert.match(edited, /assert constraint checked \{ true \}/);
  assert.match(edited, /assert constraint : ConstraintType;/);
  assert.match(edited, /assert not constraint notChecked \{ false \}/);
  assert.match(edited, /exhibit state shown : S \{/);
  assert.match(edited, /exhibit cycle;/);
  assert.match(edited, /do action work : A \{/);
  assert.match(edited, /in attribute input : ScalarValues::Real;/);
  const again = await connection.loads(edited);
  assert.ok(again.ok, again.errors.map((diagnostic) => diagnostic.message).join(", "));
});

test("transitions, imports, documentation, comments and notes round-trip", async () => {
  const source = `package Q {
    metadata def Safety;
    metadata def Approved;
}
package P {
    private import ScalarValues::*;
    attribute def CycleStart;
    attribute def CycleEnd;
    state def Mode { state off; state on; }
    part x;
}
`;
  await using connection = await connect();
  const model = await connection.loads(source);
  const result = await model
    .edit()
    .addTransition("P::Mode", "off", "on", { name: "switchOn", trigger: "CycleStart" })
    .addTransition("P::Mode", "on", "off", { name: "switchOff", trigger: "CycleEnd" })
    .addEntryTransition("P::Mode", "off")
    .addImport("P", "Q::*", { filter: ["@Q::Safety", "@Q::Approved"] })
    .addImport("P", "ISQ::*", { recursive: true, visibility: "public" })
    .addDocumentation("P::x", "The part.", { name: "details" })
    .addComment("P", "A comment.", { name: "note", about: ["P::x"] })
    .addNote("P::x", "see the comment")
    .apply();
  const edited = result.content;
  assert.match(edited, /transition switchOn first off accept CycleStart then on;/);
  assert.match(edited, /private import Q::\*\[@Q::Safety\]\[@Q::Approved\];/);
  assert.match(edited, /public import ISQ::\*::\*\*;/);
  assert.match(edited, /doc details \/\* The part\.\*\//);
  assert.match(edited, /comment note/);
  assert.match(edited, /\/\/ see the comment/);
  const again = await connection.loads(edited);
  assert.ok(again.ok, again.errors.map(String).join(", "));
});

test("connections, satisfy and requirement constraints author", async () => {
  const source = `package P {
    private import ScalarValues::*;
    port def Payload;
    part def Source { port outPort : Payload; }
    part def Sink { port inPort : Payload; }
    part src : Source;
    part dst : Sink;
    action def Step;
    action def Downstream;
    action start : Step;
    action finish : Downstream;
    requirement def R;
    requirement r : R;
}
`;
  await using connection = await connect();
  const model = await connection.loads(source);
  const result = await model
    .edit()
    .addConnection("P", "connection", "src.outPort", "dst.inPort", { name: "link" })
    .addAllocation("P", "src", "dst", { name: "assigned" })
    .addFlow("P", "src.outPort", "dst.inPort")
    .addSuccession("P", "start", "finish")
    .addSatisfy("P", "r", { by: "src", asserted: true })
    .addRequireConstraint("P::r", "src >= 0", "positive")
    .addAssumeConstraint("P::r", "dst >= 0")
    .apply();
  const edited = result.content;
  assert.match(edited, /connection link/);
  assert.match(edited, /allocation assigned/);
  assert.match(edited, /flow/);
  assert.match(edited, /succession/);
  assert.match(edited, /assert satisfy r by src/);
  assert.match(edited, /require constraint positive/);
  assert.match(edited, /assume constraint/);
  const again = await connection.loads(edited);
  assert.ok(again.ok, again.errors.map((diagnostic) => diagnostic.message).join(", "));
});

test("a referenced rename respells the references", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.edit().rename("Demo::SC::unitMass", "unitWeight").apply();
  const edited = result.content;
  assert.match(edited, /attribute unitWeight : ISQ::MassValue default = 1000\.0\[SI::kg\];/);
  assert.match(edited, /attribute total : ISQ::MassValue = unitWeight;/);
  assert.ok(!edited.includes("unitMass"));
  const again = await connection.loads(edited);
  assert.ok((await again.find("unitWeight")) !== undefined);
});

test("an unreferenced declaration is deleted, and move relocates", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const moved = await model
    .edit()
    .delete("Demo::SC::margin")
    .move("Demo::SC::label", "Demo::SC::avionics")
    .apply();
  const edited = moved.content;
  assert.ok(!edited.includes("margin"));
  const again = await connection.loads(edited);
  assert.ok(again.ok, again.errors.map(String).join(", "));
  assert.ok((await again.find("Demo::SC::avionics::label")) !== undefined);
});

test("typed refusals arrive from the service", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  await assert.rejects(
    () => model.edit().setValue("Demo::SC::nothing", "1").apply(),
    EditTargetError,
  );
  await assert.rejects(
    () => model.edit().setValue("Demo::SC::unitMass", "1050.0[").apply(),
    InvalidEditError,
  );
  await assert.rejects(
    () => model.edit().rename("Demo::SC::label", "part").apply(),
    InvalidEditError,
  );
  await assert.rejects(
    () =>
      model
        .edit()
        .setValue("Demo::SC::unitMass", "1.0[SI::kg]")
        .setValue("Demo::SC::unitMass", "2.0[SI::kg]")
        .apply(),
    OverlappingEditsError,
  );
});

test("deleting a referenced declaration is refused", async () => {
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  await assert.rejects(
    () => model.edit().delete("Demo::SC::unitMass").apply(),
    DeleteReferencedError,
  );
});

test("a refusal to move a referenced declaration is typed", async () => {
  const answer = (): ApplyEditsResponse =>
    ({
      content: "",
      applied: [],
      documents: [],
      error: "P::x is referenced by P::y and cannot move",
      failure: 17,
      diagnostics: [],
      referringElements: ["P::y"],
      referrers: [],
    }) as unknown as ApplyEditsResponse;
  await using connection = await fakeConnection(ALL_CAPABILITIES, answer);
  await assert.rejects(
    () => connection.applyEdits("hash", [["move", "P::x", "P"]]),
    MoveReferencedError,
  );
});

test("a model of several documents answers documents, not content", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-edit-"));
  const lib = join(dir, "lib.sysml");
  const car = join(dir, "car.sysml");
  const { writeFileSync } = await import("node:fs");
  writeFileSync(lib, "package Lib { part def Engine; }\n");
  writeFileSync(car, "package Car { part engine : Lib::Engine; }\n");
  await using connection = await connect();
  const model = await connection.parseSources([lib, car]);
  const result = await model.edit().rename("Lib::Engine", "Motor").apply();
  assert.equal(result.content, "");
  assert.ok(result.documents.length > 0);
  const documents = new Map(result.documents.map((d) => [d.name, d.content]));
  const libContent =
    documents.get(lib) ?? [...documents.values()].find((text) => text.includes("Lib"));
  assert.match(libContent ?? "", /part def Motor;/);
  assert.ok(result.applied.every((applied) => applied.document !== ""));
  const again = await connection.loads(libContent ?? "");
  assert.ok((await again.find("Lib::Motor")) !== undefined);
});

test("save writes an edit result's content verbatim", async () => {
  const dir = mkdtempSync(join(tmpdir(), "opensysml-save-"));
  const path = join(dir, "out.sysml");
  await using connection = await connect();
  const model = await connection.loads(MODEL);
  const result = await model.edit().setValue("Demo::sc::unitMass", "1050.0[SI::kg]").apply();
  await save(result, path);
  const { readFileSync } = await import("node:fs");
  assert.equal(readFileSync(path, "utf8"), result.content);
});
