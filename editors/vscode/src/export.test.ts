import assert from "node:assert/strict";
import { test } from "node:test";

import {
  DOCUMENTED_FORMS,
  ExportHost,
  exportFile,
  exportFileName,
  exportRendering,
  formPickItems,
  FormPickItem,
  serverForms,
  serverStyles,
} from "./export";
import { RENDER_FORMS_CAPABILITY, RENDER_STYLES_CAPABILITY, RenderParams, RenderResult } from "./protocol";

const rendering = (form: string, artifact: string): RenderResult =>
  ({ view: "Kit::widgetTree", kind: "tree", stated: "", form, artifact, nodes: [], edges: [], version: 1 }) as unknown as RenderResult;

// A host that records what the sequence asked of it and answers as scripted.
class FakeHost implements ExportHost {
  offered: FormPickItem[] = [];
  pickTitle = "";
  requests: RenderParams[] = [];
  saveDefault?: string;
  saveFilter?: { extension: string; filter: string };
  written?: { location: string; artifact: string };
  writeRefusal?: Error;
  dialogRefusal?: Error;

  constructor(
    private readonly picks: string | undefined,
    private readonly answer: (params: RenderParams) => RenderResult,
    private readonly saveAt: string | null = "file:///ws/out",
  ) {}

  async pickForm(items: FormPickItem[], title: string): Promise<string | undefined> {
    this.offered = items;
    this.pickTitle = title;
    return this.picks;
  }

  async render(params: RenderParams): Promise<RenderResult> {
    this.requests.push(params);
    return this.answer(params);
  }

  async pickSaveLocation(defaultName: string, file: { extension: string; filter: string }): Promise<string | undefined> {
    this.saveDefault = defaultName;
    this.saveFilter = file;
    if (this.dialogRefusal) {
      throw this.dialogRefusal;
    }
    return this.saveAt ?? undefined;
  }

  async write(location: string, artifact: string): Promise<void> {
    if (this.writeRefusal) {
      throw this.writeRefusal;
    }
    this.written = { location, artifact };
  }
}

const echoForm = (params: RenderParams) => rendering(params.form ?? "mermaid", `artifact in ${params.form}`);

test("serverStyles takes the drawing styles the server advertises, none from a server predating them", () => {
  assert.deepEqual(serverStyles({ [RENDER_STYLES_CAPABILITY]: ["pilot", "cameo", "cameo"] }), ["pilot", "cameo"]);
  assert.deepEqual(serverStyles(undefined), []);
  assert.deepEqual(serverStyles({ openSysmlRender: true }), []);
  assert.deepEqual(serverStyles({ [RENDER_STYLES_CAPABILITY]: ["cameo", 3] }), []);
});

test("an export asks the server for the drawing style the panel draws in, and for none under the other looks", async () => {
  const styled = new FakeHost("dot", echoForm);
  await exportRendering(styled, { uri: "file:///ws/kit.sysml", documentName: "kit.sysml", view: "", forms: ["dot"], style: "cameo" });
  assert.equal(styled.requests[0].style, "cameo");
  const plain = new FakeHost("dot", echoForm);
  await exportRendering(plain, { uri: "file:///ws/kit.sysml", documentName: "kit.sysml", view: "", forms: ["dot"] });
  assert.equal(plain.requests[0].style, undefined);
});

test("serverForms takes the forms the server advertises, else the documented five", () => {
  assert.deepEqual(serverForms({ [RENDER_FORMS_CAPABILITY]: ["text", "mermaid", "markdown", "dot", "plantuml"] }), [
    "text",
    "mermaid",
    "markdown",
    "dot",
    "plantuml",
  ]);
  assert.deepEqual(serverForms({ [RENDER_FORMS_CAPABILITY]: ["mermaid", "dot", "mermaid"] }), ["mermaid", "dot"]);
  assert.deepEqual(serverForms(undefined), DOCUMENTED_FORMS);
  assert.deepEqual(serverForms({ openSysmlRender: true }), DOCUMENTED_FORMS);
  assert.deepEqual(serverForms({ [RENDER_FORMS_CAPABILITY]: [] }), DOCUMENTED_FORMS);
  assert.deepEqual(serverForms({ [RENDER_FORMS_CAPABILITY]: ["dot", 3] }), DOCUMENTED_FORMS);
  assert.deepEqual(DOCUMENTED_FORMS, ["text", "mermaid", "markdown", "dot", "plantuml"]);
});

test("every form saves under its own extension and filter, and an unknown one as text", () => {
  assert.deepEqual(
    DOCUMENTED_FORMS.map((form) => [form, exportFile(form).extension, exportFile(form).filter]),
    [
      ["text", ".txt", "Text"],
      ["mermaid", ".mmd", "Mermaid"],
      ["markdown", ".md", "Markdown"],
      ["dot", ".dot", "Graphviz DOT"],
      ["plantuml", ".puml", "PlantUML"],
    ],
  );
  assert.deepEqual(exportFile("svg"), { extension: ".txt", filter: "Text" });
  assert.equal(exportFileName("car.sysml", "dot"), "car.dot");
  assert.equal(exportFileName("core.kerml", "plantuml"), "core.puml");
  assert.equal(exportFileName("notes", "mermaid"), "notes.mmd");
});

test("the picker offers the server's forms in its order, each naming the file it saves as", () => {
  const items = formPickItems(["mermaid", "dot", "plantuml", "svg"]);
  assert.deepEqual(
    items.map((item) => [item.label, item.value, item.description.split(" — ")[0]]),
    [
      ["mermaid", "mermaid", ".mmd"],
      ["dot", "dot", ".dot"],
      ["plantuml", "plantuml", ".puml"],
      ["svg", "svg", ".txt"],
    ],
  );
});

test("picking dot asks the server for dot and saves the answer as a .dot file", async () => {
  const host = new FakeHost("dot", echoForm, "file:///ws/graph.dot");
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTree", forms: DOCUMENTED_FORMS });
  assert.deepEqual(host.offered.map((item) => item.value), DOCUMENTED_FORMS);
  assert.equal(host.pickTitle, "Export car.sysml as which form?");
  assert.deepEqual(host.requests, [{ textDocument: { uri: "file:///ws/car.sysml" }, view: "Kit::widgetTree", form: "dot" }]);
  assert.equal(host.saveDefault, "car.dot");
  assert.deepEqual(host.saveFilter, { extension: ".dot", filter: "Graphviz DOT" });
  assert.deepEqual(host.written, { location: "file:///ws/graph.dot", artifact: "artifact in dot" });
  assert.deepEqual(outcome, { kind: "saved", form: "dot", location: "file:///ws/graph.dot" });
});

test("picking plantuml saves a .puml, and the other forms their own extension", async () => {
  for (const [form, name] of [
    ["plantuml", "car.puml"],
    ["mermaid", "car.mmd"],
    ["text", "car.txt"],
    ["markdown", "car.md"],
  ]) {
    const host = new FakeHost(form, echoForm);
    const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTree", forms: DOCUMENTED_FORMS });
    assert.equal(host.requests[0].form, form, form);
    assert.equal(host.saveDefault, name, form);
    assert.equal(host.written?.artifact, `artifact in ${form}`, form);
    assert.equal(outcome.kind, "saved", form);
  }
});

test("the file is named by the form the server answered, should it differ from the one asked", async () => {
  const host = new FakeHost("dot", () => rendering("markdown", "| a |"));
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTable", forms: DOCUMENTED_FORMS });
  assert.equal(host.saveDefault, "car.md");
  assert.deepEqual(host.saveFilter, { extension: ".md", filter: "Markdown" });
  assert.deepEqual(outcome, { kind: "saved", form: "markdown", location: "file:///ws/out" });
});

test("dismissing the form pick sends no request and writes nothing", async () => {
  const host = new FakeHost(undefined, echoForm);
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTree", forms: DOCUMENTED_FORMS });
  assert.deepEqual(outcome, { kind: "cancelled" });
  assert.deepEqual(host.requests, []);
  assert.equal(host.saveDefault, undefined);
  assert.equal(host.written, undefined);
});

test("cancelling the save dialog writes nothing after the render", async () => {
  const host = new FakeHost("plantuml", echoForm, null);
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTree", forms: DOCUMENTED_FORMS });
  assert.deepEqual(outcome, { kind: "cancelled" });
  assert.equal(host.requests.length, 1);
  assert.equal(host.written, undefined);
});

test("a server that refuses the form fails the export with its message, before any save dialog", async () => {
  const host = new FakeHost("dot", () => {
    throw new Error('"dot" is not a form of a sequence rendering: write "mermaid", "text" or "plantuml"');
  });
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetSequence", forms: DOCUMENTED_FORMS });
  assert.deepEqual(outcome, { kind: "failed", step: "render", message: '"dot" is not a form of a sequence rendering: write "mermaid", "text" or "plantuml"' });
  assert.equal(host.saveDefault, undefined);
  assert.equal(host.written, undefined);
});

test("a save dialog that fails to open fails the export as a save failure, with its message", async () => {
  const host = new FakeHost("dot", echoForm);
  host.dialogRefusal = new Error("Unable to open the save dialog");
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTree", forms: DOCUMENTED_FORMS });
  assert.deepEqual(outcome, { kind: "failed", step: "save", message: "Unable to open the save dialog" });
  assert.equal(host.requests.length, 1);
  assert.equal(host.saveDefault, "car.dot");
  assert.equal(host.written, undefined);
});

test("a destination that refuses the write fails the export as a save failure, with the file system's message", async () => {
  const host = new FakeHost("dot", echoForm, "file:///ws/readonly/car.dot");
  host.writeRefusal = new Error("EACCES: permission denied, open '/ws/readonly/car.dot'");
  const outcome = await exportRendering(host, { uri: "file:///ws/car.sysml", documentName: "car.sysml", view: "Kit::widgetTree", forms: DOCUMENTED_FORMS });
  assert.deepEqual(outcome, { kind: "failed", step: "save", message: "EACCES: permission denied, open '/ws/readonly/car.dot'" });
  assert.equal(host.requests.length, 1);
  assert.equal(host.saveDefault, "car.dot");
  assert.equal(host.written, undefined);
});
