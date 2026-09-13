import { randomBytes } from "node:crypto";
import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import {
  connectionOwner,
  describeOwner,
  describeRefusal,
  editParams,
  endpointPath,
  moveDestinations,
  moveOperation,
  offeredOn,
  ownerOf,
  placementOperations,
  REDRAWN_MESSAGE,
  Rendering,
  rootOwner,
  validName,
} from "./edits";
import {
  admits,
  APPLY_MODEL_EDIT_CAPABILITY,
  APPLY_MODEL_EDIT_METHOD,
  ApplyModelEditResult,
  EdgePlacement,
  EditAction,
  FromWebview,
  ModelEditOperation,
  NodePlacement,
  PickerEntry,
  RENDER_CAPABILITY,
  RENDER_CHANGED_METHOD,
  RENDER_METHOD,
  RenderChangedParams,
  RenderNode,
  RenderResult,
  ToWebview,
  VIEWS_METHOD,
  ViewsResult,
} from "./protocol";
import { declaredViewEntries, DEFAULT_PSEUDO_VIEW, impliedView, pseudoViewEntries } from "./views";

/** The context key the Open Diagram command is enabled by. */
const SUPPORTED_KEY = "opensysml.renderSupported";

/** The type a restored panel is revived under. */
const PANEL_TYPE = "opensysml.diagram";

// The file an exported rendering is saved as, by the form the server wrote.
const EXPORT_FORMS: Record<string, { extension: string; filter: string }> = {
  mermaid: { extension: ".mmd", filter: "Mermaid" },
  markdown: { extension: ".md", filter: "Markdown" },
  text: { extension: ".txt", filter: "Text" },
};

/** The views of a document, as the picker offers them: declared first, then the pseudo-views. */
interface ViewListing {
  declared: PickerEntry[];
  pseudo: PickerEntry[];
}

// listViews asks the server for a document's views. A failure is logged and yields
// no declared views: the listing is a picker's content, not the diagram.
async function listViews(client: LanguageClient, uri: string, output: vscode.OutputChannel): Promise<ViewListing> {
  let listing: ViewsResult | undefined;
  try {
    listing = await client.sendRequest<ViewsResult>(VIEWS_METHOD, { textDocument: { uri } });
  } catch (err) {
    output.appendLine(`Listing the views of ${vscode.Uri.parse(uri).fsPath} failed: ${errorMessage(err)}`);
  }
  return { declared: declaredViewEntries(listing), pseudo: pseudoViewEntries(listing?.pseudoViews) };
}

/**
 * DiagramPanels owns the diagram webviews: one per document, drawn from the
 * server's rendering of it and redrawn when the server says it went stale.
 */
export class DiagramPanels implements vscode.Disposable {
  private readonly panels = new Map<string, DiagramPanel>();
  private readonly disposables: vscode.Disposable[] = [];
  private commands: vscode.Disposable[] = [];
  private notification: vscode.Disposable | undefined;
  private client: LanguageClient | undefined;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly output: vscode.OutputChannel,
  ) {
    this.disposables.push(
      vscode.window.onDidChangeTextEditorSelection((event) => {
        this.panels.get(event.textEditor.document.uri.toString())?.highlightAt(event.selections[0].active);
      }),
      vscode.window.registerWebviewPanelSerializer(PANEL_TYPE, {
        deserializeWebviewPanel: async (panel, state: { uri?: string; view?: string } | undefined) => {
          if (!state?.uri) {
            panel.dispose();
            return;
          }
          this.adopt(vscode.Uri.parse(state.uri), panel, state.view ?? "");
        },
      }),
    );
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, false);
  }

  /**
   * attach binds the panels to a started client. The command is registered only
   * when the server advertised the render capability, so an older `sysml-lsp`
   * keeps working without a diagram panel instead of erroring.
   */
  attach(client: LanguageClient | undefined): void {
    this.detach();
    if (!client || !supportsRender(client)) {
      if (client) {
        this.output.appendLine(
          `Language server does not advertise ${RENDER_CAPABILITY}; the diagram panel stays unavailable.`,
        );
      }
      for (const panel of this.panels.values()) {
        panel.fail("The language server does not serve diagrams.");
      }
      return;
    }
    this.client = client;
    this.commands = [
      vscode.commands.registerCommand("opensysml.openDiagram", () => this.open()),
      vscode.commands.registerCommand("opensysml.exportDiagram", () => this.export(client)),
    ];
    this.notification = client.onNotification(RENDER_CHANGED_METHOD, (params: RenderChangedParams) => {
      this.panels.get(vscode.Uri.parse(params.textDocument.uri).toString())?.refresh();
    });
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, true);
    for (const panel of this.panels.values()) {
      panel.refresh();
    }
  }

  /** detach drops what a client owned, so a restart does not leave it behind. */
  detach(): void {
    this.client = undefined;
    this.notification?.dispose();
    this.notification = undefined;
    for (const command of this.commands) {
      command.dispose();
    }
    this.commands = [];
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, false);
  }

  dispose(): void {
    this.detach();
    for (const disposable of this.disposables) {
      disposable.dispose();
    }
    for (const panel of this.panels.values()) {
      panel.dispose();
    }
  }

  /** open shows the panel for the active document, beside it. */
  private open(): void {
    const editor = vscode.window.activeTextEditor;
    if (!editor || !isModel(editor.document)) {
      void vscode.window.showInformationMessage("Open a .sysml or .kerml file to draw a diagram of it.");
      return;
    }
    const key = editor.document.uri.toString();
    const existing = this.panels.get(key);
    if (existing) {
      existing.reveal();
      return;
    }
    const panel = vscode.window.createWebviewPanel(
      PANEL_TYPE,
      `Diagram: ${basename(editor.document.uri)}`,
      { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
      { enableScripts: true, localResourceRoots: [vscode.Uri.joinPath(this.extensionUri, "dist")] },
    );
    this.adopt(editor.document.uri, panel, "");
  }

  /**
   * export writes the machine form of the active document's diagram — Mermaid
   * for a diagram, Markdown for a table — to a file the user picks: the view
   * its panel has chosen when one is open, else the one the document implies,
   * else the one the user picks.
   */
  private async export(client: LanguageClient): Promise<void> {
    const editor = vscode.window.activeTextEditor;
    if (!editor || !isModel(editor.document)) {
      void vscode.window.showInformationMessage("Open a .sysml or .kerml file to export a diagram of it.");
      return;
    }
    const uri = editor.document.uri.toString();
    // A panel that has not chosen among the document's views leaves the choice here.
    const view = this.panels.get(uri)?.selectedView() || await this.exportedView(client, uri);
    if (view === undefined) {
      return;
    }
    let result: RenderResult;
    try {
      result = await client.sendRequest<RenderResult>(RENDER_METHOD, { textDocument: { uri }, view });
    } catch (err) {
      void vscode.window.showErrorMessage(`Rendering ${basename(editor.document.uri)} failed: ${errorMessage(err)}`);
      return;
    }
    const { extension, filter } = EXPORT_FORMS[result.form] ?? { extension: ".txt", filter: "Text" };
    const stem = basename(editor.document.uri).replace(/\.(sysml|kerml)$/, "");
    const target = await vscode.window.showSaveDialog({
      defaultUri: vscode.Uri.joinPath(editor.document.uri, "..", `${stem}${extension}`),
      filters: { [filter]: [extension.slice(1)] },
    });
    if (!target) {
      return;
    }
    await vscode.workspace.fs.writeFile(target, new TextEncoder().encode(result.artifact));
    this.output.appendLine(`Exported ${result.form} of ${basename(editor.document.uri)} to ${target.fsPath}`);
  }

  // exportedView is the view to export when no panel shows one: the document's
  // sole drawable view, the model tree when it has none, or the user's pick among
  // several. Undefined when the user picks none.
  private async exportedView(client: LanguageClient, uri: string): Promise<string | undefined> {
    const { declared, pseudo } = await listViews(client, uri, this.output);
    const implied = impliedView(declared);
    if (implied !== undefined) {
      return implied;
    }
    const items = [...declared.filter((entry) => entry.supported), ...pseudo].map((entry) => ({ label: entry.label, entry }));
    const picked = await vscode.window.showQuickPick(items, { title: `Export which view of ${basename(vscode.Uri.parse(uri))}?`, matchOnDetail: true });
    return picked?.entry.value;
  }

  // adopt takes ownership of a panel, whether it was just created or restored.
  private adopt(docURI: vscode.Uri, panel: vscode.WebviewPanel, selected: string): void {
    const key = docURI.toString();
    this.panels.get(key)?.dispose();
    const diagram = new DiagramPanel(docURI, panel, selected, this.extensionUri, this.output, () => this.client);
    this.panels.set(key, diagram);
    panel.onDidDispose(() => {
      if (this.panels.get(key) === diagram) {
        this.panels.delete(key);
      }
    });
  }
}

/** DiagramPanel is one document's diagram. */
class DiagramPanel {
  private readonly disposables: vscode.Disposable[] = [];
  private selected: string;
  private rendering: Rendering = { nodes: [], version: 0 };
  private pending = false;
  private again = false;
  private disposed = false;

  constructor(
    private readonly docURI: vscode.Uri,
    private readonly panel: vscode.WebviewPanel,
    selected: string,
    extensionUri: vscode.Uri,
    private readonly output: vscode.OutputChannel,
    private readonly client: () => LanguageClient | undefined,
  ) {
    this.selected = selected;
    this.panel.webview.html = html(this.panel.webview, extensionUri, docURI, selected);
    this.disposables.push(
      this.panel.webview.onDidReceiveMessage((message: FromWebview) => this.receive(message)),
      // A hidden panel is not drawn and not rendered for: the webview is torn
      // down while hidden, so it is refreshed when it comes back.
      this.panel.onDidChangeViewState(() => {
        if (this.panel.visible) {
          this.refresh();
        }
      }),
    );
    this.panel.onDidDispose(() => this.dispose());
  }

  reveal(): void {
    this.panel.reveal(vscode.ViewColumn.Beside, true);
  }

  /** selectedView is the view the panel draws, "" for the document's own. */
  selectedView(): string {
    return this.selected;
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    for (const disposable of this.disposables) {
      disposable.dispose();
    }
    this.panel.dispose();
  }

  /** fail leaves the last diagram on screen and states why it is out of date. */
  fail(message: string): void {
    this.post({ type: "error", message });
  }

  /**
   * refresh pulls a fresh rendering. Nothing is requested for a hidden panel,
   * which is what the push-notify/pull-artifact protocol is for. A request in
   * flight is not doubled; it is repeated once it settles, since a pick of
   * another view has no notification of its own to redraw it.
   */
  refresh(): void {
    if (this.disposed || !this.panel.visible) {
      return;
    }
    if (this.pending) {
      this.again = true;
      return;
    }
    const client = this.client();
    if (!client) {
      this.fail("The language server is not running.");
      return;
    }
    this.pending = true;
    void this.render(client).finally(() => {
      this.pending = false;
      if (this.again) {
        this.again = false;
        this.refresh();
      }
    });
  }

  private async render(client: LanguageClient): Promise<void> {
    const textDocument = { uri: this.docURI.toString() };
    const { declared, pseudo } = await listViews(client, textDocument.uri, this.output);
    // A document declaring no drawable view is rendered as its model tree, so
    // the panel shows the model being written rather than nothing.
    if (this.selected === "" && !declared.some((entry) => entry.supported)) {
      this.selected = DEFAULT_PSEUDO_VIEW;
    }
    this.post({
      type: "views",
      views: [...declared, ...pseudo],
      selected: this.selected,
    });
    try {
      // No form is asked for: the server writes the machine form of the kind it
      // rendered, which is Mermaid for a diagram and Markdown for a table.
      const result = await client.sendRequest<RenderResult>(RENDER_METHOD, {
        textDocument,
        view: this.selected === "" ? undefined : this.selected,
      });
      // No palette unless the server also computes the edits it would lead to.
      if (!supportsEdit(client)) {
        delete result.palette;
      }
      this.rendering = {
        nodes: result.nodes ?? [],
        edges: result.edges ?? [],
        view: result.view,
        version: result.version,
        palette: result.palette,
      };
      this.post({ type: "render", result, selected: this.selected });
      this.highlightActive();
    } catch (err) {
      this.fail(errorMessage(err));
    }
  }

  /** highlightAt marks the node whose declaration contains the cursor. */
  highlightAt(at: vscode.Position): void {
    this.post({ type: "highlight", id: this.nodeAt(this.rendering, at)?.id });
  }

  private highlightActive(): void {
    const editor = vscode.window.visibleTextEditors.find(
      (candidate) => candidate.document.uri.toString() === this.docURI.toString(),
    );
    if (editor) {
      this.highlightAt(editor.selection.active);
    }
  }

  // nodeAt is the innermost located node whose declaration contains at.
  private nodeAt(rendering: Rendering, at: vscode.Position): RenderNode | undefined {
    let found: RenderNode | undefined;
    let foundRange: vscode.Range | undefined;
    for (const node of rendering.nodes) {
      if (!node.origin || vscode.Uri.parse(node.origin.uri).toString() !== this.docURI.toString()) {
        continue;
      }
      const range = toRange(node.origin.range);
      if (!range.contains(at)) {
        continue;
      }
      if (!foundRange || foundRange.contains(range)) {
        found = node;
        foundRange = range;
      }
    }
    return found;
  }

  private receive(message: FromWebview): void {
    switch (message.type) {
      case "ready":
        this.refresh();
        return;
      case "pick":
        this.selected = message.view;
        this.refresh();
        return;
      case "reveal":
        void this.revealSource(message.id);
        return;
      case "edit":
        void this.edit(message.action, message.version);
        return;
      case "place":
        void this.place(message.nodes, message.edges, message.version);
        return;
      case "failed":
        this.fail(message.message);
        return;
    }
  }

  // revealSource opens the declaration a node was built from.
  private async revealSource(id: string): Promise<void> {
    const origin = this.node(this.rendering, id)?.origin;
    if (!origin) {
      return;
    }
    // The identifier alone when the server located it: selecting the whole
    // declaration would select the element's entire body.
    const range = toRange(origin.selectionRange ?? origin.range);
    const document = await vscode.workspace.openTextDocument(vscode.Uri.parse(origin.uri));
    const editor = await vscode.window.showTextDocument(document, {
      viewColumn: vscode.ViewColumn.One,
      preserveFocus: false,
    });
    editor.selection = new vscode.Selection(range.start, range.end);
    editor.revealRange(range, vscode.TextEditorRevealType.InCenterIfOutsideViewport);
  }

  // edit applies a diagram action as a workspace edit, so it is undone like typing; the redraw
  // comes from the server's renderChanged. The action's ids name only the rendering it was offered on.
  private async edit(action: EditAction, version: number): Promise<void> {
    const rendering = this.rendering;
    if (!offeredOn(rendering, version)) {
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
    const operations = await this.operationsFor(rendering, action);
    if (!operations) {
      return;
    }
    await this.apply(rendering, operations, action);
  }

  // place writes where a drag left nodes and edges into the model as one edit, so
  // the whole gesture is one undo step. The canvas shows the drag's outcome until the
  // model changes; whenever it does not, it is redrawn from the model as it stands.
  private async place(nodes: NodePlacement[], edges: EdgePlacement[], version: number): Promise<void> {
    const rendering = this.rendering;
    if (!offeredOn(rendering, version)) {
      this.refresh();
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
    const operations = placementOperations(rendering, nodes, edges);
    if (!operations || operations.length === 0 || !(await this.apply(rendering, operations, { kind: "place" }))) {
      this.refresh();
    }
  }

  private async operationsFor(rendering: Rendering, action: EditAction): Promise<ModelEditOperation[] | undefined> {
    switch (action.kind) {
      case "addMember":
        return this.addMember(rendering, action.memberKind, action.typed, action.owner);
      case "addConnection":
        return this.addConnection(rendering, action.connectionKind, action.from, action.to);
      case "rename":
        return this.rename(rendering, action.id);
      case "delete":
        return this.delete(rendering, action.id, false);
      case "move":
        return this.move(rendering, action.id);
    }
  }

  private async addMember(
    rendering: Rendering,
    memberKind: string,
    typed: boolean,
    at: string | undefined,
  ): Promise<ModelEditOperation[] | undefined> {
    const owner = at ? ownerOf(this.node(rendering, at), rendering.nodes) : await this.ownerFromContext(rendering, memberKind);
    if (!owner?.fqn) {
      return undefined;
    }
    if (!admits(rendering.palette, memberKind, owner)) {
      void vscode.window.showErrorMessage(`A ${memberKind} cannot be declared in ${owner.fqn}.`);
      return undefined;
    }
    const name = await vscode.window.showInputBox({
      title: `Add ${memberKind} to ${owner.fqn}`,
      prompt: "Name of the new declaration",
      validateInput: validName,
    });
    if (name === undefined) {
      return undefined;
    }
    let type: string | undefined;
    if (typed) {
      type = await vscode.window.showInputBox({
        title: `Add ${memberKind} ${name.trim()}`,
        prompt: "Type, as written from that scope (leave empty for none)",
      });
      if (type === undefined) {
        return undefined;
      }
    }
    return [{ kind: "addMember", owner: owner.fqn, memberKind, name: name.trim(), type: type?.trim() || undefined }];
  }

  private async addConnection(
    rendering: Rendering,
    connectionKind: string,
    fromID: string | undefined,
    toID: string | undefined,
  ): Promise<ModelEditOperation[] | undefined> {
    const from = fromID ? this.node(rendering, fromID) : await this.pickNode(rendering, `${connectionKind}: from`, undefined);
    if (!from) {
      return undefined;
    }
    const to = toID ? this.node(rendering, toID) : await this.pickNode(rendering, `${connectionKind} from ${from.name}: to`, from);
    if (!to) {
      return undefined;
    }
    const owner = connectionOwner(from, to);
    if (!owner) {
      void vscode.window.showErrorMessage(`${from.name} and ${to.name} are not both declared in this document, so no ${connectionKind} can join them here.`);
      return undefined;
    }
    const ends = [endpointPath(from, owner), endpointPath(to, owner)];
    if (!ends[0] || !ends[1]) {
      void vscode.window.showErrorMessage(`A ${connectionKind} needs two named features below ${describeOwner(owner)}.`);
      return undefined;
    }
    const name = await vscode.window.showInputBox({
      title: `Add ${connectionKind} from ${ends[0]} to ${ends[1]}`,
      prompt: "Name (leave empty for an unnamed connection)",
      validateInput: (value) => (value.trim() === "" ? undefined : validName(value)),
    });
    if (name === undefined) {
      return undefined;
    }
    return [{
      kind: "addConnection",
      owner: owner.fqn,
      memberKind: connectionKind,
      from: ends[0],
      to: ends[1],
      name: name.trim() || undefined,
    }];
  }

  private async rename(rendering: Rendering, id: string): Promise<ModelEditOperation[] | undefined> {
    const node = this.node(rendering, id);
    if (!node?.fqn) {
      return undefined;
    }
    const newName = await vscode.window.showInputBox({
      title: `Rename ${node.fqn}`,
      value: node.name,
      validateInput: validName,
    });
    if (newName === undefined || newName.trim() === node.name) {
      return undefined;
    }
    return [{ kind: "rename", target: node.fqn, newName: newName.trim() }];
  }

  private async delete(rendering: Rendering, id: string, cascade: boolean): Promise<ModelEditOperation[] | undefined> {
    const node = this.node(rendering, id);
    if (!node?.fqn) {
      return undefined;
    }
    if (!cascade) {
      const answer = await vscode.window.showWarningMessage(`Delete ${node.fqn}?`, { modal: true }, "Delete");
      if (answer !== "Delete") {
        return undefined;
      }
    }
    return [{ kind: "delete", target: node.fqn, cascade: cascade || undefined }];
  }

  // A move is offered the drawn declarations that admit the node's kind, and the document.
  private async move(rendering: Rendering, id: string): Promise<ModelEditOperation[] | undefined> {
    const node = this.node(rendering, id);
    if (!node?.fqn) {
      return undefined;
    }
    const items = moveDestinations(node, rendering).map(({ fqn, node: into }) =>
      into
        ? { label: into.name, description: into.type ? `${into.kind} : ${into.type}` : into.kind, detail: into.fqn, fqn }
        : { label: "Document", description: "a top-level declaration", fqn },
    );
    if (items.length === 0) {
      void vscode.window.showInformationMessage(`The diagram has no node a ${node.notation} can be moved into.`);
      return undefined;
    }
    const picked = await vscode.window.showQuickPick(items, { title: `Move ${node.fqn} to`, matchOnDetail: true });
    if (!picked) {
      return undefined;
    }
    const operation = moveOperation(node, picked.fqn);
    return operation && [operation];
  }

  // A palette addition goes into the declaration at the cursor, else the one root, else a pick;
  // each only if it may own the kind.
  private async ownerFromContext(rendering: Rendering, memberKind: string): Promise<RenderNode | undefined> {
    const keep = (node: RenderNode) => Boolean(node.fqn) && admits(rendering.palette, memberKind, node);
    const editor = vscode.window.visibleTextEditors.find(
      (candidate) => candidate.document.uri.toString() === this.docURI.toString(),
    );
    const atCursor = editor ? ownerOf(this.nodeAt(rendering, editor.selection.active), rendering.nodes) : undefined;
    if (atCursor && keep(atCursor)) {
      return atCursor;
    }
    const root = rootOwner(rendering.nodes);
    if (root && keep(root)) {
      return root;
    }
    return this.pickNode(rendering, `Add ${memberKind} to`, undefined, keep);
  }

  private async pickNode(
    rendering: Rendering,
    title: string,
    except: RenderNode | undefined,
    keep: (node: RenderNode) => boolean = (node) => node.fqn !== undefined,
  ): Promise<RenderNode | undefined> {
    const items = rendering.nodes
      .filter((node) => node !== except && keep(node))
      .map((node) => ({ label: node.name, description: node.type ? `${node.kind} : ${node.type}` : node.kind, detail: node.fqn, node }));
    if (items.length === 0) {
      void vscode.window.showInformationMessage("The diagram has no node this can apply to.");
      return undefined;
    }
    const picked = await vscode.window.showQuickPick(items, { title, matchOnDetail: true });
    return picked?.node;
  }

  private node(rendering: Rendering, id: string): RenderNode | undefined {
    return rendering.nodes.find((node) => node.id === id);
  }

  // apply has the server compute the operations' edit and applies it; it reports
  // whether the document changed.
  private async apply(rendering: Rendering, operations: ModelEditOperation[], action: AppliedAction): Promise<boolean> {
    const client = this.client();
    if (!client || !supportsEdit(client)) {
      this.fail("The language server does not serve model edits.");
      return false;
    }
    const document = vscode.workspace.textDocuments.find(
      (candidate) => candidate.uri.toString() === this.docURI.toString(),
    );
    if (!document) {
      this.fail("The document is not open, so it cannot be edited.");
      return false;
    }
    let result: ApplyModelEditResult;
    try {
      const params = editParams(document.uri.toString(), rendering, operations);
      result = await client.sendRequest<ApplyModelEditResult>(APPLY_MODEL_EDIT_METHOD, params);
    } catch (err) {
      void vscode.window.showErrorMessage(`The edit could not be computed: ${errorMessage(err)}`);
      return false;
    }
    // The names acted on may spell other declarations now: redraw, do not retry.
    if (result.stale) {
      this.refresh();
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return false;
    }
    if (result.refused) {
      return this.refused(rendering, result, action);
    }
    if (!result.edit) {
      this.fail("The language server answered with neither an edit nor a refusal.");
      return false;
    }
    const edit = await client.protocol2CodeConverter.asWorkspaceEdit(result.edit);
    if (!(await vscode.workspace.applyEdit(edit))) {
      void vscode.window.showErrorMessage("VS Code did not apply the edit.");
      return false;
    }
    return true;
  }

  // A delete refused for references is offered again as a cascade, and applied if
  // taken up; anything else is just told.
  private async refused(rendering: Rendering, result: ApplyModelEditResult, action: AppliedAction): Promise<boolean> {
    const refused = result.refused ?? [];
    const message = describeRefusal(refused);
    this.output.appendLine(`Model edit refused:\n${message}`);
    if (action.kind === "delete" && refused.some((refusal) => refusal.failure === "delete-referenced")) {
      const referring = refused.flatMap((refusal) => refusal.referring ?? []);
      const answer = await vscode.window.showWarningMessage(
        `${message}\n\nDelete the referring declarations too?`,
        { modal: true, detail: referring.join("\n") },
        "Delete all",
      );
      if (answer === "Delete all") {
        const operations = await this.delete(rendering, action.id, true);
        if (operations) {
          return this.apply(rendering, operations, { kind: "delete", id: action.id });
        }
      }
      return false;
    }
    void vscode.window.showErrorMessage(message);
    return false;
  }

  private post(message: ToWebview): void {
    if (!this.disposed) {
      void this.panel.webview.postMessage(message);
    }
  }
}

/** AppliedAction is what an edit was made for: a menu or palette action, or a drag on the canvas. */
type AppliedAction = EditAction | { kind: "place" };

/** supportsEdit reports whether the server advertised the model-edit capability. */
function supportsEdit(client: LanguageClient): boolean {
  return experimental(client)?.[APPLY_MODEL_EDIT_CAPABILITY] === true;
}

/** supportsRender reports whether the server advertised the render capability. */
function supportsRender(client: LanguageClient): boolean {
  return experimental(client)?.[RENDER_CAPABILITY] === true;
}

function experimental(client: LanguageClient): Record<string, unknown> | undefined {
  return client.initializeResult?.capabilities?.experimental as Record<string, unknown> | undefined;
}

function isModel(document: vscode.TextDocument): boolean {
  return document.languageId === "sysml" || document.languageId === "kerml";
}

function basename(uri: vscode.Uri): string {
  const parts = uri.path.split("/");
  return parts[parts.length - 1] || uri.toString();
}

function toRange(range: { start: { line: number; character: number }; end: { line: number; character: number } }): vscode.Range {
  return new vscode.Range(
    new vscode.Position(range.start.line, range.start.character),
    new vscode.Position(range.end.line, range.end.character),
  );
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    return String((err as { message: unknown }).message);
  }
  return String(err);
}

/**
 * html is the panel's document. Scripts are the bundled webview script alone,
 * allowed by nonce, and nothing is loaded from the network: the diagram is drawn
 * as SVG by that script.
 */
function html(
  webview: vscode.Webview,
  extensionUri: vscode.Uri,
  docURI: vscode.Uri,
  selected: string,
): string {
  const script = webview.asWebviewUri(vscode.Uri.joinPath(extensionUri, "dist", "webview.js"));
  const nonce = randomNonce();
  const csp = [
    "default-src 'none'",
    `img-src ${webview.cspSource} data:`,
    `style-src ${webview.cspSource} 'unsafe-inline'`,
    `font-src ${webview.cspSource} data:`,
    `script-src 'nonce-${nonce}'`,
  ].join("; ");
  const state = attribute(JSON.stringify({ uri: docURI.toString(), view: selected }));
  return `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="${csp}" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>SysML diagram</title>
    <style>
      body { margin: 0; padding: 0.5rem; font-family: var(--vscode-font-family); color: var(--vscode-foreground); }
      #bar { display: flex; align-items: center; gap: 0.5rem; padding-bottom: 0.5rem; }
      #view { flex: 1 1 auto; max-width: 30rem; }
      #kind { opacity: 0.8; font-size: 0.9em; }
      #add { max-width: 14rem; }
      #status { color: var(--vscode-errorForeground); min-height: 1.2em; font-size: 0.9em; white-space: pre-wrap; }
      #diagram { overflow: auto; }
      #menu { position: fixed; z-index: 10; min-width: 12rem; max-height: calc(100vh - 1rem); overflow-y: auto;
        padding: 0.25rem 0; margin: 0; list-style: none;
        background: var(--vscode-menu-background, var(--vscode-editorWidget-background));
        color: var(--vscode-menu-foreground, var(--vscode-foreground));
        border: 1px solid var(--vscode-menu-border, var(--vscode-widget-border)); box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35); }
      #menu li { padding: 0.25rem 1rem; cursor: pointer; white-space: nowrap; }
      #menu li:hover { background: var(--vscode-menu-selectionBackground); color: var(--vscode-menu-selectionForeground); }
      #menu li.separator { height: 0; padding: 0; margin: 0.25rem 0; border-top: 1px solid var(--vscode-menu-separatorBackground, var(--vscode-widget-border)); cursor: default; }
      #menu li.title { opacity: 0.7; cursor: default; font-size: 0.9em; }
      #diagram.stale { opacity: 0.45; }
      #diagram svg { display: block; font-family: var(--vscode-font-family); user-select: none; touch-action: none; }
      #diagram svg.dragging { cursor: grabbing; }
      #diagram g.opensysml-node { cursor: pointer; }
      #diagram g.opensysml-node.movable { cursor: grab; }
      #diagram .shape { fill: var(--vscode-editorWidget-background, var(--vscode-editor-background)); stroke: var(--vscode-foreground); stroke-width: 1.25px; }
      #diagram .shape.container { fill: var(--vscode-sideBar-background, var(--vscode-editor-background)); }
      #diagram .shape.filled { fill: var(--vscode-foreground); }
      #diagram .label { fill: var(--vscode-foreground); }
      #diagram .label .head { font-weight: 600; }
      #diagram .label .keyword { font-size: 0.85em; opacity: 0.8; }
      #diagram .label .detail { font-size: 0.9em; opacity: 0.9; }
      #diagram .collapsed { fill: var(--vscode-foreground); opacity: 0.7; }
      #diagram .line { fill: none; stroke: var(--vscode-foreground); stroke-width: 1.25px; }
      #diagram .lifeline { stroke: var(--vscode-foreground); stroke-width: 1px; stroke-dasharray: 6 4; opacity: 0.6; }
      #diagram .flow .line { stroke-dasharray: 5 4; }
      #diagram .arrow-fill { fill: var(--vscode-foreground); }
      #diagram .arrow-line { fill: none; stroke: var(--vscode-foreground); stroke-width: 1.25px; }
      #diagram .edge-label { fill: var(--vscode-foreground); font-size: 0.85em; paint-order: stroke; stroke: var(--vscode-editor-background); stroke-width: 3px; stroke-linejoin: round; }
      #diagram .waypoint { fill: var(--vscode-editor-background); stroke: var(--vscode-focusBorder); stroke-width: 1.5px; cursor: move; }
      #diagram .segment { fill: var(--vscode-focusBorder); opacity: 0; cursor: copy; }
      #diagram .segment:hover, #diagram .waypoint:hover { opacity: 1; }
      #diagram svg:hover .segment { opacity: 0.45; }
      #diagram .opensysml-selected > .shape, #diagram .opensysml-selected > g.shape > circle {
        stroke: var(--vscode-focusBorder); stroke-width: 3px;
      }
      details { margin-top: 0.75rem; font-size: 0.9em; }
      pre { white-space: pre-wrap; }
    </style>
  </head>
  <body data-state='${state}'>
    <div id="bar">
      <label for="view">View</label>
      <select id="view"></select>
      <span id="kind"></span>
      <select id="add" hidden aria-label="Add to the model"></select>
    </div>
    <div id="status"></div>
    <div id="diagram"></div>
    <ul id="menu" hidden role="menu"></ul>
    <details id="notices" hidden>
      <summary></summary>
      <ul id="notice-list"></ul>
    </details>
    <details id="undrawable" hidden>
      <summary></summary>
      <ul id="undrawable-list"></ul>
    </details>
    <script nonce="${nonce}" src="${script}"></script>
  </body>
</html>`;
}

// attribute escapes a value written into an HTML attribute. A document path or a
// quoted view name may hold any of these, and one of them would end the value.
function attribute(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("'", "&#39;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

// randomNonce is a per-page nonce, so only the script this page shipped runs.
function randomNonce(): string {
  return randomBytes(16).toString("hex");
}
