import { randomBytes } from "node:crypto";
import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import {
  connectionOwner,
  describeOwner,
  describeRefusal,
  describeStale,
  editParams,
  endpointPath,
  fileLabel,
  moveDestinations,
  moveOperation,
  offeredOn,
  ownerOf,
  placementOperations,
  referrersByFile,
  reparentOperations,
  staleDocuments,
  unopenedDocuments,
  REDRAWN_MESSAGE,
  Rendering,
  rootOwner,
  validName,
} from "./edits";
import {
  admits,
  APPLY_MODEL_EDIT_CAPABILITY,
  CROSS_DOCUMENT_CAPABILITY,
  declaredHere,
  ownDeclarations,
  APPLY_MODEL_EDIT_METHOD,
  ApplyModelEditResult,
  EdgePlacement,
  EditAction,
  FromWebview,
  ModelEditOperation,
  NodePlacement,
  normalizeRender,
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
  WorkspaceEdit,
} from "./protocol";
import { ActiveEditor, AUTO_OPEN_SETTING, Dismissals, Lifecycle, renamedUri, shouldAutoOpen, TabKind } from "./autoopen";
import { CommandContext, PANEL_TYPE, resolveTarget } from "./target";
import {
  chooseView,
  ChosenViews,
  DeclaredView,
  declaredViewEntries,
  expandChoice,
  impliedView,
  panelKey,
  pseudoViewEntries,
  viewPickItems,
  viewTitle,
} from "./views";

/** Why the diagram commands cannot serve, when they cannot. */
const NOT_RUNNING = "The SysML v2 language server is not running; run \"SysML: Restart Language Server\" to start it.";
const NOT_SERVED = "The SysML v2 language server does not serve diagrams; update sysml-lsp to draw one.";

// The file an exported rendering is saved as, by the form the server wrote.
const EXPORT_FORMS: Record<string, { extension: string; filter: string }> = {
  mermaid: { extension: ".mmd", filter: "Mermaid" },
  markdown: { extension: ".md", filter: "Markdown" },
  text: { extension: ".txt", filter: "Text" },
};

/** The views of a document, as the picker offers them: declared first, then the pseudo-views. */
interface ViewListing {
  declared: DeclaredView[];
  pseudo: PickerEntry[];
  /** Set when the server could not be asked: the empty listing then says nothing about the document. */
  failed?: true;
}

// listViews asks the server for a document's views. A failure is logged and yields
// no declared views: the listing is a picker's content, not the diagram.
async function listViews(client: LanguageClient, uri: string, output: vscode.OutputChannel): Promise<ViewListing> {
  let listing: ViewsResult;
  try {
    listing = await client.sendRequest<ViewsResult>(VIEWS_METHOD, { textDocument: { uri } });
  } catch (err) {
    output.appendLine(`Listing the views of ${vscode.Uri.parse(uri).fsPath} failed: ${errorMessage(err)}`);
    return { declared: [], pseudo: pseudoViewEntries(undefined), failed: true };
  }
  return { declared: declaredViewEntries(listing), pseudo: pseudoViewEntries(listing.pseudoViews) };
}

/**
 * DiagramPanels owns the diagram webviews: one per document and view, drawn from
 * the server's rendering of it and redrawn when the server says it went stale.
 */
export class DiagramPanels implements vscode.Disposable {
  /** The panels, by panelKey of the document and the view each draws. */
  private readonly panels = new Map<string, DiagramPanel>();
  private readonly disposables: vscode.Disposable[] = [];
  private readonly dismissals: Dismissals;
  private readonly chosenViews: ChosenViews;
  private notification: vscode.Disposable | undefined;
  private client: LanguageClient | undefined;
  private unavailable: string | undefined = NOT_RUNNING;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly output: vscode.OutputChannel,
    workspaceState: vscode.Memento,
  ) {
    this.dismissals = new Dismissals(workspaceState);
    this.chosenViews = new ChosenViews(workspaceState);
    // The commands exist whether or not a server serves them, so a keybinding
    // or menu always answers — with the diagram, or with why there is none.
    this.disposables.push(
      vscode.commands.registerCommand("opensysml.openDiagram", (target?: unknown) => this.open(target)),
      vscode.commands.registerCommand("opensysml.exportDiagram", (target?: unknown) => this.export(target)),
      vscode.window.onDidChangeTextEditorSelection((event) => {
        for (const panel of this.panelsOf(event.textEditor.document.uri)) {
          panel.highlightAt(event.selections[0].active);
        }
      }),
      vscode.window.onDidChangeActiveTextEditor((editor) => this.autoOpen(editor)),
      vscode.workspace.onDidChangeConfiguration((event) => {
        if (event.affectsConfiguration(AUTO_OPEN_SETTING)) {
          this.autoOpen(vscode.window.activeTextEditor);
        }
      }),
      // A dismissal, a chosen view and an open panel follow the file through a rename; the records die with the file.
      vscode.workspace.onDidRenameFiles((event) => {
        for (const { oldUri, newUri } of event.files) {
          void this.dismissals.rename(oldUri.toString(), newUri.toString());
          void this.chosenViews.rename(oldUri.toString(), newUri.toString());
          this.rebind(oldUri, newUri);
        }
      }),
      vscode.workspace.onDidDeleteFiles((event) => {
        for (const uri of event.files) {
          void this.dismissals.clear(uri.toString());
          void this.chosenViews.clear(uri.toString());
        }
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
  }

  /**
   * attach binds the panels to a started client. Without the render capability
   * the commands stay registered but explain themselves, so an older `sysml-lsp`
   * keeps working without a diagram panel instead of erroring.
   */
  attach(client: LanguageClient | undefined): void {
    this.detach();
    if (!client || !supportsRender(client)) {
      if (client) {
        this.output.appendLine(
          `Language server does not advertise ${RENDER_CAPABILITY}; the diagram panel stays unavailable.`,
        );
        this.unavailable = NOT_SERVED;
      }
      for (const panel of this.panels.values()) {
        panel.fail("The language server does not serve diagrams.");
      }
      return;
    }
    this.client = client;
    this.unavailable = undefined;
    this.notification = client.onNotification(RENDER_CHANGED_METHOD, (params: RenderChangedParams) => {
      for (const panel of this.panelsOf(vscode.Uri.parse(params.textDocument.uri))) {
        panel.refresh();
      }
    });
    for (const panel of this.panels.values()) {
      panel.refresh();
    }
    // A model file made active while the server was starting gets its diagram now.
    this.autoOpen(vscode.window.activeTextEditor);
  }

  /** detach drops what a client owned, so a restart does not leave it behind. */
  detach(): void {
    this.client = undefined;
    this.unavailable = NOT_RUNNING;
    this.notification?.dispose();
    this.notification = undefined;
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

  /**
   * open shows the diagram of a document beside its source: the one a command
   * names, or the one given. From the panel, it shows the source. The view is the
   * one given, else the one the document implies, the one under the cursor, the
   * one last chosen for the document, or the user's pick; a pick of "All views"
   * opens one panel per drawable view. A panel already drawing the view is
   * revealed rather than opened again.
   */
  async open(target?: unknown, view?: string): Promise<void> {
    const resolved = this.resolve(target, "draw a diagram of it");
    if (!resolved) {
      return;
    }
    const { uri, fromPanel } = resolved;
    // Returning to the source is the editor's own doing, so it works without a server.
    if (fromPanel) {
      await vscode.window.showTextDocument(uri, { viewColumn: this.sourceColumn(uri), preserveFocus: false });
      return;
    }
    if (!this.available()) {
      return;
    }
    // Asking for the diagram undoes an earlier close of it.
    const key = uri.toString();
    await this.dismissals.clear(key);
    // A file chosen in the Explorer is opened first, so the diagram sits beside its source.
    let editor = editorOf(uri);
    if (!editor) {
      editor = await vscode.window.showTextDocument(uri, { preview: false });
    }
    const views = view !== undefined ? [view] : await this.viewsToDraw(uri, editor, true);
    for (const chosen of views) {
      this.show(uri, chosen);
    }
  }

  /**
   * autoOpen draws the diagram of a model file the user is looking at, without
   * being asked and without a word when it cannot; a file whose diagram the user
   * closed stays as it is until Open Diagram asks for it again. A document that
   * declares several views is drawn only when the cursor or an earlier choice
   * names one: nothing is asked.
   */
  private autoOpen(editor: vscode.TextEditor | undefined): void {
    if (editor && this.autoOpenWanted(editor)) {
      void this.viewsToDraw(editor.document.uri, editor, false).then((views) => {
        // The listing took time; the editor may have moved on or the document gained a panel.
        if (vscode.window.activeTextEditor !== editor || !this.autoOpenWanted(editor)) {
          return;
        }
        for (const chosen of views) {
          this.show(editor.document.uri, chosen);
        }
      });
    }
  }

  // autoOpenWanted is whether the editor's document gets a diagram unasked, as of now.
  private autoOpenWanted(editor: vscode.TextEditor | undefined): boolean {
    const uri = editor?.document.uri.toString();
    return shouldAutoOpen({
      enabled: autoOpenEnabled(),
      available: this.unavailable === undefined,
      hasPanel: editor !== undefined && this.panelsOf(editor.document.uri).length > 0,
      dismissed: uri !== undefined && this.dismissals.has(uri),
      editor: editor && activeEditorInfo(editor),
    });
  }

  // viewsToDraw is what is drawn of a document: the view the document, the
  // cursor or an earlier choice decides, else — when asking is allowed — what the
  // user picks. Empty when nothing decides and nothing is picked. The cursor is
  // read after the listing returns: an editor just made active still restores it.
  // Without a listing, the server's own choice is drawn when asked, nothing unasked.
  private async viewsToDraw(docURI: vscode.Uri, editor: vscode.TextEditor, ask: boolean): Promise<string[]> {
    const client = this.client;
    if (!client) {
      return ask ? [""] : [];
    }
    const version = editor.document.version;
    const { declared, pseudo, failed } = await listViews(client, docURI.toString(), this.output);
    if (failed) {
      return ask ? [""] : [];
    }
    // Ranges were listed for the document as it was; an edit since leaves the cursor to decide nothing.
    const cursor = editor.document.version === version ? editor.selection.active : undefined;
    const choice = chooseView(declared, pseudo, cursor, this.chosenViews.get(docURI.toString()));
    if (choice.view !== undefined) {
      return [choice.view];
    }
    if (choice.stale !== undefined) {
      await this.chosenViews.remember(docURI.toString(), undefined);
    }
    if (!ask) {
      return [];
    }
    const picked = await vscode.window.showQuickPick(viewPickItems(declared, pseudo), {
      title: `Which view of ${basename(docURI)}?`,
      matchOnDetail: true,
    });
    if (!picked) {
      return [];
    }
    const views = expandChoice(picked.value, declared);
    if (views.length === 1) {
      await this.chosenViews.remember(docURI.toString(), views[0]);
    }
    return views;
  }

  // show reveals the panel drawing a view of a document, creating it in the
  // diagram column when there is none.
  private show(docURI: vscode.Uri, view: string): void {
    const existing = this.panels.get(panelKey(docURI.toString(), view));
    if (existing) {
      existing.reveal(this.diagramColumn());
      return;
    }
    this.create(docURI, view);
  }

  // create opens a panel of a view of a document in the diagram column, leaving focus where it is.
  private create(uri: vscode.Uri, selected = "", column = this.diagramColumn()): void {
    const panel = vscode.window.createWebviewPanel(
      PANEL_TYPE,
      `Diagram: ${basename(uri)}`,
      { viewColumn: column, preserveFocus: true },
      webviewOptions(this.extensionUri),
    );
    this.adopt(uri, panel, selected);
  }

  /** panelsOf is every panel drawing a view of the document. */
  private panelsOf(docURI: vscode.Uri): DiagramPanel[] {
    const key = docURI.toString();
    return [...this.panels.values()].filter((panel) => panel.documentUri().toString() === key);
  }

  // rebind moves the panels of a renamed document, or of every document in a
  // renamed folder, to their new URIs, keeping view and group; the replacement
  // is the extension's doing, not a dismissal.
  private rebind(from: vscode.Uri, to: vscode.Uri): void {
    // The loop replaces panels, so it walks a snapshot of them.
    for (const old of Array.from(this.panels.values())) {
      const moved = renamedUri(old.documentUri().toString(), from.toString(), to.toString());
      if (moved === undefined) {
        continue;
      }
      const column = old.column() ?? this.diagramColumn();
      const selected = old.selectedView();
      old.dispose();
      if (!this.panels.has(panelKey(moved, selected))) {
        this.create(vscode.Uri.parse(moved), selected, column);
      }
    }
  }

  // diagramColumn is the one group the diagrams share: where a panel already
  // sits, else beside the source. One column of diagram tabs keeps the layout calm.
  private diagramColumn(): vscode.ViewColumn {
    for (const panel of this.panels.values()) {
      const column = panel.column();
      if (column !== undefined) {
        return column;
      }
    }
    return vscode.ViewColumn.Beside;
  }

  /** export writes the machine form (Mermaid or Markdown) of the named document's diagram to a file. */
  private async export(target?: unknown): Promise<void> {
    const resolved = this.resolve(target, "export a diagram of it");
    const client = this.client;
    if (!resolved || !this.available() || !client) {
      return;
    }
    const uri = resolved.uri.toString();
    // Several panels of the document leave the choice between them here, as does a
    // panel that has not chosen among the document's views.
    const panels = this.panelsOf(resolved.uri);
    const view = (panels.length === 1 ? panels[0].selectedView() : "") || await this.exportedView(client, uri);
    if (view === undefined) {
      return;
    }
    let result: RenderResult;
    try {
      result = await client.sendRequest<RenderResult>(RENDER_METHOD, { textDocument: { uri }, view });
    } catch (err) {
      void vscode.window.showErrorMessage(`Rendering ${basename(resolved.uri)} failed: ${errorMessage(err)}`);
      return;
    }
    const { extension, filter } = EXPORT_FORMS[result.form] ?? { extension: ".txt", filter: "Text" };
    const stem = basename(resolved.uri).replace(/\.(sysml|kerml)$/, "");
    const saveAs = await vscode.window.showSaveDialog({
      defaultUri: vscode.Uri.joinPath(resolved.uri, "..", `${stem}${extension}`),
      filters: { [filter]: [extension.slice(1)] },
    });
    if (!saveAs) {
      return;
    }
    await vscode.workspace.fs.writeFile(saveAs, new TextEncoder().encode(result.artifact));
    this.output.appendLine(`Exported ${result.form} of ${basename(resolved.uri)} to ${saveAs.fsPath}`);
  }

  // resolve names the document a command acts on, or tells the user that no model file is in view.
  private resolve(target: unknown, verb: string): { uri: vscode.Uri; fromPanel: boolean } | undefined {
    const context: CommandContext = {
      argument: target instanceof vscode.Uri ? target.toString() : undefined,
      focusedPanel: this.focusedPanel()?.documentUri().toString(),
      activeEditor: vscode.window.activeTextEditor && editorInfo(vscode.window.activeTextEditor),
      visibleEditors: vscode.window.visibleTextEditors.map(editorInfo),
    };
    const resolved = resolveTarget(context, verb);
    if (resolved.kind === "none") {
      void vscode.window.showInformationMessage(resolved.message);
      return undefined;
    }
    return { uri: vscode.Uri.parse(resolved.uri), fromPanel: resolved.fromPanel };
  }

  // available is whether a server draws diagrams; when none does, it tells the user why.
  private available(): boolean {
    if (this.unavailable) {
      void vscode.window.showWarningMessage(this.unavailable);
      return false;
    }
    return true;
  }

  // focusedPanel is the diagram panel that has focus, if one does.
  private focusedPanel(): DiagramPanel | undefined {
    for (const panel of this.panels.values()) {
      if (panel.isActive()) {
        return panel;
      }
    }
    return undefined;
  }

  // sourceColumn is where a document is shown: its visible editor's group, else
  // a group beside the panel.
  private sourceColumn(uri: vscode.Uri): vscode.ViewColumn {
    const key = uri.toString();
    const editor = vscode.window.visibleTextEditors.find((candidate) => candidate.document.uri.toString() === key);
    return editor?.viewColumn ?? vscode.ViewColumn.Beside;
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

  // adopt takes ownership of a panel, whether it was just created or restored. A
  // restored panel drawing what another already draws replaces it. Only a panel
  // whose tab the user closes is a dismissal, and only the document's last one.
  private adopt(docURI: vscode.Uri, panel: vscode.WebviewPanel, selected: string): void {
    const key = panelKey(docURI.toString(), selected);
    this.panels.get(key)?.dispose();
    const diagram: DiagramPanel = new DiagramPanel(docURI, panel, selected, this.extensionUri, this.output, () => this.client, {
      retarget: (from, to, picked) => this.retarget(diagram, from, to, picked),
      closed: (byUser) => {
        if (this.panels.get(diagram.key()) === diagram) {
          this.panels.delete(diagram.key());
        }
        if (byUser && this.panelsOf(docURI).length === 0) {
          void this.dismissals.record(docURI.toString());
        }
        this.retitle(docURI);
      },
    });
    this.panels.set(key, diagram);
    this.retitle(docURI, true);
  }

  // retarget re-keys a panel that moves to another view. When another panel
  // already draws that view, that one is revealed instead and the move is
  // refused, so a document never has two panels of one view. A view the user
  // picked is remembered for the document; one the panel settled on is not.
  private retarget(diagram: DiagramPanel, from: string, to: string, picked: boolean): boolean {
    const docURI = diagram.documentUri();
    const taken = this.panels.get(panelKey(docURI.toString(), to));
    if (taken && taken !== diagram) {
      taken.reveal(this.diagramColumn());
      return false;
    }
    if (this.panels.get(panelKey(docURI.toString(), from)) === diagram) {
      this.panels.delete(panelKey(docURI.toString(), from));
    }
    this.panels.set(panelKey(docURI.toString(), to), diagram);
    if (picked) {
      void this.chosenViews.remember(docURI.toString(), to);
    }
    this.retitle(docURI);
    return true;
  }

  // retitle names a document's panels by view once it has more than one. A
  // panel gained keeps a longer title it was restored with: the document's other
  // panels are restored only once shown, so they may not be known yet.
  private retitle(docURI: vscode.Uri, gained = false): void {
    const panels = this.panelsOf(docURI);
    if (gained && panels.length < 2) {
      return;
    }
    for (const panel of panels) {
      panel.setTitle(panels.length > 1 ? `Diagram: ${basename(docURI)} — ${viewTitle(panel.selectedView())}` : `Diagram: ${basename(docURI)}`);
    }
  }
}

/** What a panel tells its owner: that it moves to another view, which the owner may refuse, and that it closed. */
interface PanelOwner {
  retarget(from: string, to: string, picked: boolean): boolean;
  closed(byUser: boolean): void;
}

// editorOf is the visible editor showing a document, if one does.
function editorOf(uri: vscode.Uri): vscode.TextEditor | undefined {
  const key = uri.toString();
  return vscode.window.visibleTextEditors.find((editor) => editor.document.uri.toString() === key);
}

function autoOpenEnabled(): boolean {
  return vscode.workspace.getConfiguration("opensysml.diagram").get<boolean>("autoOpen", true);
}

// activeEditorInfo reads the active editor and its tab into what the auto-open
// decision looks at: a diff tab and a hover's peek editor draw no diagram.
function activeEditorInfo(editor: vscode.TextEditor): ActiveEditor {
  const uri = editor.document.uri;
  const input = vscode.window.tabGroups.activeTabGroup.activeTab?.input;
  let tab: TabKind = "other";
  if (input === undefined) {
    tab = "none";
  } else if (input instanceof vscode.TabInputTextDiff) {
    tab = "diff";
  } else if (input instanceof vscode.TabInputText && input.uri.toString() === uri.toString()) {
    tab = "text";
  }
  return { uri: uri.toString(), languageId: editor.document.languageId, scheme: uri.scheme, inGroup: editor.viewColumn !== undefined, tab };
}

/** DiagramPanel is one document's diagram. */
class DiagramPanel {
  private readonly disposables: vscode.Disposable[] = [];
  private selected: string;
  private rendering: Rendering = { nodes: [], version: 0 };
  // Counts the drawings posted to the webview; an action names the one its ids came from.
  private drawn = 0;
  private pending = false;
  private again = false;
  private readonly lifecycle = new Lifecycle();

  constructor(
    private readonly docURI: vscode.Uri,
    private readonly panel: vscode.WebviewPanel,
    selected: string,
    extensionUri: vscode.Uri,
    private readonly output: vscode.OutputChannel,
    private readonly client: () => LanguageClient | undefined,
    private readonly owner: PanelOwner,
  ) {
    this.selected = selected;
    // A restored panel keeps the resource roots of the extension version that
    // created it; after an update the script lives elsewhere, so set them again.
    this.panel.webview.options = webviewOptions(extensionUri);
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
    this.panel.onDidDispose(() => {
      const byUser = this.lifecycle.closed();
      this.release();
      owner.closed(byUser);
    });
  }

  reveal(column: vscode.ViewColumn): void {
    this.panel.reveal(column, true);
  }

  /** column is the editor group the panel sits in, if it is shown in one. */
  column(): vscode.ViewColumn | undefined {
    return this.lifecycle.disposed ? undefined : this.panel.viewColumn;
  }

  private get disposed(): boolean {
    return this.lifecycle.disposed;
  }

  /** isActive reports whether the panel is the focused editor. */
  isActive(): boolean {
    return this.panel.active;
  }

  /** documentUri is the document the panel draws. */
  documentUri(): vscode.Uri {
    return this.docURI;
  }

  /** selectedView is the view the panel draws, "" for the document's own. */
  selectedView(): string {
    return this.selected;
  }

  /** key is the panel's place among its owner's panels. */
  key(): string {
    return panelKey(this.docURI.toString(), this.selected);
  }

  setTitle(title: string): void {
    if (!this.disposed && this.panel.title !== title) {
      this.panel.title = title;
    }
  }

  // select makes the panel draw another view, unless its owner shows that view already;
  // `selected` is set before the owner re-keys the panel, so the title it sets is the new one.
  private select(view: string, picked: boolean): void {
    const from = this.selected;
    if (view === from) {
      return;
    }
    this.selected = view;
    if (!this.owner.retarget(from, view, picked)) {
      this.selected = from;
    }
  }

  dispose(): void {
    if (!this.lifecycle.dispose()) {
      return;
    }
    this.release();
    this.panel.dispose();
  }

  private release(): void {
    for (const disposable of this.disposables.splice(0)) {
      disposable.dispose();
    }
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
    // A panel opened on no particular view draws the one the document implies:
    // its sole view, or the model tree when it declares none, so the model being
    // written is shown rather than nothing. Several views leave the picker to say.
    if (this.selected === "") {
      const implied = impliedView(declared);
      if (implied !== undefined) {
        this.select(implied, false);
      }
    }
    this.post({
      type: "views",
      views: [...declared, ...pseudo],
      selected: this.selected,
    });
    try {
      // No form is asked for: the server writes the machine form of the kind it
      // rendered, which is Mermaid for a diagram and Markdown for a table.
      const result = normalizeRender(
        await client.sendRequest<RenderResult>(RENDER_METHOD, {
          textDocument,
          view: this.selected === "" ? undefined : this.selected,
        }),
      );
      // No palette unless the server also computes the edits it would lead to.
      if (!supportsEdit(client)) {
        delete result.palette;
      }
      // A server predating declaredHere names the requested document's own alone.
      if (!supportsCrossDocument(client)) {
        result.nodes = ownDeclarations(result.nodes ?? []);
      }
      this.rendering = {
        nodes: result.nodes,
        edges: result.edges,
        view: result.view,
        version: result.version,
        palette: result.palette,
      };
      this.drawn += 1;
      this.post({ type: "render", result, selected: this.selected, drawn: this.drawn });
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
        this.select(message.view, true);
        this.refresh();
        return;
      case "reveal":
        void this.revealSource(message.id, message.drawn);
        return;
      case "edit":
        void this.edit(message.action, message.drawn);
        return;
      case "place":
        void this.place(message.nodes, message.edges, message.drawn);
        return;
      case "reparent":
        void this.reparent(message.id, message.owner, message.nodes, message.edges, message.drawn);
        return;
      case "failed":
        this.fail(message.message);
        return;
    }
  }

  // revealSource opens the declaration a node was built from; the id names only the drawing it was clicked on.
  private async revealSource(id: string, drawn: number): Promise<void> {
    if (!offeredOn(this.drawn, drawn)) {
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
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

  // edit applies a diagram action as a workspace edit, so it is undone like typing. The action's
  // ids name only the drawing it was offered on, which is checked again after any prompt it held open.
  private async edit(action: EditAction, drawn: number): Promise<void> {
    const rendering = this.rendering;
    if (!offeredOn(this.drawn, drawn)) {
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
    const operations = await this.operationsFor(rendering, action);
    if (!operations) {
      return;
    }
    if (!offeredOn(this.drawn, drawn)) {
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
    await this.apply(rendering, operations, action);
  }

  // place writes where a drag left nodes and edges into the model as one edit, so
  // the whole gesture is one undo step. The canvas shows the drag's outcome until the
  // model changes; whenever it does not, it is redrawn from the model as it stands.
  private async place(nodes: NodePlacement[], edges: EdgePlacement[], drawn: number): Promise<void> {
    const rendering = this.rendering;
    if (!offeredOn(this.drawn, drawn)) {
      this.refresh();
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
    const operations = placementOperations(rendering, nodes, edges);
    if (!operations || operations.length === 0 || !(await this.apply(rendering, operations, { kind: "place" }))) {
      this.refresh();
    }
  }

  // reparent moves a dropped node into the node it was dropped on, placed where it was
  // released, as one edit; a refusal is shown in the panel and the drop is undrawn.
  private async reparent(id: string, owner: string, nodes: NodePlacement[], edges: EdgePlacement[], drawn: number): Promise<void> {
    const rendering = this.rendering;
    if (!offeredOn(this.drawn, drawn)) {
      this.refresh();
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return;
    }
    const operations = reparentOperations(rendering, id, owner, nodes, edges);
    if (!operations) {
      this.post({ type: "revert", message: "The node cannot be moved there from this diagram." });
      return;
    }
    if (!(await this.apply(rendering, operations, { kind: "reparent", id, owner }))) {
      this.post({ type: "revert" });
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
    if (!owner || !declaredHere(owner)) {
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
    if (!node || !declaredHere(node)) {
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
    if (!node || !declaredHere(node)) {
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
    if (!node || !declaredHere(node)) {
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
    const keep = (node: RenderNode) => declaredHere(node) && admits(rendering.palette, memberKind, node);
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
    keep: (node: RenderNode) => boolean = declaredHere,
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
    let edit = await this.compute(client, document, rendering, operations, action);
    if (!edit) {
      return false;
    }
    // A document the server read from disk is opened first, so the edit is computed
    // against a buffer, versioned and synced to the server like the others.
    const unopened = unopenedDocuments(edit, openVersion);
    if (unopened.length > 0) {
      try {
        await Promise.all(unopened.map((uri) => vscode.workspace.openTextDocument(vscode.Uri.parse(uri))));
      } catch (err) {
        this.fail(`${unopened.map(fileLabel).join(", ")} could not be opened for the edit: ${errorMessage(err)}`);
        return false;
      }
      edit = await this.compute(client, document, rendering, operations, action);
      if (!edit) {
        return false;
      }
      const still = unopenedDocuments(edit, openVersion);
      if (still.length > 0) {
        this.fail(`${still.map(fileLabel).join(", ")} is not synced with the language server, so the edit is not applied.`);
        return false;
      }
    }
    const converted = await client.protocol2CodeConverter.asWorkspaceEdit(edit);
    // Checked and applied in one turn: applyEdit pins each document to the version compared here,
    // and VS Code refuses the edit if any moves on before it lands.
    const stale = staleDocuments(edit, openVersion);
    if (stale.length > 0) {
      this.output.appendLine(`Model edit not applied: ${describeStale(stale)}`);
      void vscode.window.showWarningMessage(describeStale(stale));
      return false;
    }
    // One applyEdit call, so every document changes together and one undo reverts them all.
    if (!(await vscode.workspace.applyEdit(converted))) {
      void vscode.window.showErrorMessage("VS Code did not apply the edit.");
      return false;
    }
    return true;
  }

  // compute asks the server for the edit; a stale rendering, a refusal or an error is told and yields nothing.
  private async compute(
    client: LanguageClient,
    document: vscode.TextDocument,
    rendering: Rendering,
    operations: ModelEditOperation[],
    action: AppliedAction,
  ): Promise<WorkspaceEdit | undefined> {
    let result: ApplyModelEditResult;
    try {
      const params = editParams(document.uri.toString(), rendering, operations);
      result = await client.sendRequest<ApplyModelEditResult>(APPLY_MODEL_EDIT_METHOD, params);
    } catch (err) {
      void vscode.window.showErrorMessage(`The edit could not be computed: ${errorMessage(err)}`);
      return undefined;
    }
    // The names acted on may spell other declarations now: redraw, do not retry.
    if (result.stale) {
      this.refresh();
      void vscode.window.showWarningMessage(REDRAWN_MESSAGE);
      return undefined;
    }
    if (result.refused) {
      await this.refused(rendering, result, action);
      return undefined;
    }
    if (!result.edit) {
      this.fail("The language server answered with neither an edit nor a refusal.");
      return undefined;
    }
    return result.edit;
  }

  // A delete refused for references is offered again as a cascade, and applied if
  // taken up; anything else is just told.
  private async refused(rendering: Rendering, result: ApplyModelEditResult, action: AppliedAction): Promise<boolean> {
    const refused = result.refused ?? [];
    const message = describeRefusal(refused);
    this.output.appendLine(`Model edit refused:\n${message}`);
    if (action.kind === "delete" && refused.some((refusal) => refusal.failure === "delete-referenced")) {
      const referring = referrersByFile(refused, this.docURI.toString());
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
    if (action.kind === "reparent") {
      this.post({ type: "revert", message });
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
type AppliedAction = EditAction | { kind: "place" } | { kind: "reparent"; id: string; owner: string };

/** supportsEdit reports whether the server advertised the model-edit capability. */
function supportsEdit(client: LanguageClient): boolean {
  return experimental(client)?.[APPLY_MODEL_EDIT_CAPABILITY] === true;
}

/** supportsCrossDocument reports whether the server advertised the cross-document diagram contract. */
function supportsCrossDocument(client: LanguageClient): boolean {
  return experimental(client)?.[CROSS_DOCUMENT_CAPABILITY] === true;
}

/** openVersion is the version of the open buffer at a URI, or nothing when no buffer holds it. */
function openVersion(uri: string): number | undefined {
  const key = vscode.Uri.parse(uri).toString();
  return vscode.workspace.textDocuments.find((candidate) => candidate.uri.toString() === key)?.version;
}

/** supportsRender reports whether the server advertised the render capability. */
function supportsRender(client: LanguageClient): boolean {
  return experimental(client)?.[RENDER_CAPABILITY] === true;
}

function experimental(client: LanguageClient): Record<string, unknown> | undefined {
  return client.initializeResult?.capabilities?.experimental as Record<string, unknown> | undefined;
}

function editorInfo(editor: vscode.TextEditor): { uri: string; languageId: string } {
  return { uri: editor.document.uri.toString(), languageId: editor.document.languageId };
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

/** webviewOptions lets the panel run scripts and load resources from this installation's bundle only. */
function webviewOptions(extensionUri: vscode.Uri): vscode.WebviewOptions {
  return { enableScripts: true, localResourceRoots: [vscode.Uri.joinPath(extensionUri, "dist")] };
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
      #status.hint { color: var(--vscode-descriptionForeground, var(--vscode-foreground)); }
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
      #diagram:focus { outline: none; }
      #diagram svg { display: block; font-family: var(--vscode-font-family); user-select: none; touch-action: none; }
      #diagram.dragging, #diagram.dragging * { cursor: grabbing; }
      #diagram.dragging.refused, #diagram.dragging.refused * { cursor: not-allowed; }
      #diagram g.lifted { opacity: 0.75; }
      #diagram .opensysml-drop-target > .shape, #diagram .opensysml-drop-target > g.shape > circle {
        stroke: var(--vscode-focusBorder); stroke-width: 3px; stroke-dasharray: 6 3;
        fill: var(--vscode-editor-selectionBackground, var(--vscode-list-activeSelectionBackground));
      }
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
    <div id="diagram" tabindex="-1"></div>
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
